package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type UserInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type wrappedUserResponse struct {
	Data UserInfo `json:"data"`
	Err  string   `json:"message"`
}

type chatMessage struct {
	Id          string    `db:"id"`
	RoomId      string    `db:"room_id"`
	AuthorId    string    `db:"author_id"`
	MessageType int16     `db:"message_type"`
	Content     string    `db:"content"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

type cursor struct {
	CreatedAt time.Time `json:"created_at"`
	Id        uuid.UUID `json:"id"`
}

func decodeCursor(s string) (cursor, error) {
	c := cursor{}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

func main() {
	_ = godotenv.Load()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Command-line flags
	afterCursorStr := flag.String("after-cursor", "", "Cursor string (base64-encoded) to start gap sync from (e.g., from client's newest known message)")
	limit := flag.Int("limit", 1000, "Maximum number of messages to sync in one run")
	flag.Parse()

	sourceDBUrl := os.Getenv("CHAT_DB_URL")
	if sourceDBUrl == "" {
		sourceDBUrl = "postgres://sephy:ouilala0328@localhost:5432/chinwag_chat?sslmode=disable"
	}

	targetDBUrl := os.Getenv("CHAT_QUERY_DB_URL")
	if targetDBUrl == "" {
		targetDBUrl = "postgres://sephy:ouilala0328@localhost:5432/chinwag_chat_projection?sslmode=disable"
	}

	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://localhost:8081"
	}

	sourceDB, err := sqlx.Connect("pgx", sourceDBUrl)
	if err != nil {
		log.Error("failed to connect to source DB", "error", err)
		os.Exit(1)
	}
	defer sourceDB.Close()

	targetDB, err := sqlx.Connect("pgx", targetDBUrl)
	if err != nil {
		log.Error("failed to connect to target DB", "error", err)
		os.Exit(1)
	}
	defer targetDB.Close()

	var msgs []chatMessage
	var query string
	var queryArgs []interface{}

	if *afterCursorStr != "" {
		// Incremental sync: fetch messages strictly newer than the cursor
		c, err := decodeCursor(*afterCursorStr)
		if err != nil {
			log.Error("failed to decode cursor", "error", err)
			os.Exit(1)
		}
		query = `SELECT id, room_id, author_id, message_type, content, created_at, updated_at, deleted_at
		         FROM chat_messages
		         WHERE (created_at, id) > ($1, $2)
		         ORDER BY created_at ASC, id ASC
		         LIMIT $3`
		queryArgs = []interface{}{c.CreatedAt, c.Id, *limit}
		log.Info("starting incremental gap sync",
			"after_created_at", c.CreatedAt,
			"after_id", c.Id.String(),
			"limit", *limit,
		)
	} else {
		// Full sync: fetch all messages
		query = `SELECT id, room_id, author_id, message_type, content, created_at, updated_at, deleted_at
		         FROM chat_messages
		         ORDER BY created_at ASC, id ASC
		         LIMIT $1`
		queryArgs = []interface{}{*limit}
		log.Info("starting full gap sync", "limit", *limit)
	}

	err = sourceDB.SelectContext(context.Background(), &msgs, query, queryArgs...)
	if err != nil {
		log.Error("failed to query messages", "error", err)
		os.Exit(1)
	}

	if len(msgs) == 0 {
		log.Info("no messages to sync")
		return
	}

	log.Info("found messages to sync", "count", len(msgs))

	httpClient := &http.Client{Timeout: 5 * time.Second}
	userCache := make(map[string]string)

	getUserName := func(userId string) (string, error) {
		if name, ok := userCache[userId]; ok {
			return name, nil
		}

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, authServiceURL+"/user/"+userId, nil)
		if err != nil {
			return "", fmt.Errorf("create user request: %w", err)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("fetch user %s: %w", userId, err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read user %s response: %w", userId, err)
		}

		var wrapped wrappedUserResponse
		if err := json.Unmarshal(body, &wrapped); err != nil {
			return "", fmt.Errorf("unmarshal user %s: %w", userId, err)
		}

		if wrapped.Err != "" {
			return "", fmt.Errorf("auth service error for %s: %s", userId, wrapped.Err)
		}

		if wrapped.Data.Name == "" {
			return "", fmt.Errorf("user %s has empty name", userId)
		}

		userCache[userId] = wrapped.Data.Name
		return wrapped.Data.Name, nil
	}

	tx, err := targetDB.Begin()
	if err != nil {
		log.Error("failed to begin transaction", "error", err)
		os.Exit(1)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO message_projections (id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			room_id = EXCLUDED.room_id,
			author_id = EXCLUDED.author_id,
			author_name = EXCLUDED.author_name,
			message_type = EXCLUDED.message_type,
			content = EXCLUDED.content,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at,
			deleted_at = EXCLUDED.deleted_at`)
	if err != nil {
		log.Error("failed to prepare statement", "error", err)
		os.Exit(1)
	}
	defer stmt.Close()

	var inserted int
	for _, msg := range msgs {
		authorName, err := getUserName(msg.AuthorId)
		if err != nil {
			log.Error("failed to resolve author name", "id", msg.Id, "author_id", msg.AuthorId, "error", err)
			tx.Rollback()
			os.Exit(1)
		}

		_, err = stmt.Exec(
			msg.Id, msg.RoomId, msg.AuthorId, authorName,
			msg.MessageType, msg.Content, msg.CreatedAt, msg.UpdatedAt, msg.DeletedAt,
		)
		if err != nil {
			log.Error("failed to insert message", "id", msg.Id, "error", err)
			tx.Rollback()
			os.Exit(1)
		}

		inserted++
		if inserted%100 == 0 {
			log.Info("sync progress", "inserted", inserted, "total", len(msgs))
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error("failed to commit transaction", "error", err)
		os.Exit(1)
	}

	log.Info("gap sync completed",
		"total_messages_synced", len(msgs),
		"inserted", inserted,
		"user_cache_size", len(userCache),
		"auth_url", authServiceURL,
	)

	// Record gap-sync event in outbox
	msgId := uuid.Must(uuid.NewV7())
	now := time.Now()
	gapSyncPayload, _ := json.Marshal(map[string]interface{}{
		"type": "gap_sync_complete",
		"data": map[string]interface{}{
			"total_messages": len(msgs),
			"inserted":       inserted,
			"after_cursor":   *afterCursorStr,
			"timestamp":      now,
		},
	})
	_, err = sourceDB.ExecContext(context.Background(),
		`INSERT INTO outbox_events (id, event_type, subject, payload, room_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		msgId, "gap_sync_complete", "chat.system", gapSyncPayload, uuid.Nil, now,
	)
	if err != nil {
		log.Warn("failed to record gap-sync event in outbox", "error", err)
	}
}
