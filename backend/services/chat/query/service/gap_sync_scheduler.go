package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/repo"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

const (
	gapSyncLastSyncKey = "gap_sync:last_sync"
	gapSyncInterval    = 30 * time.Second // Run every 30 seconds
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

func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	c := cursor{CreatedAt: createdAt, Id: id}
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
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

type GapSyncScheduler struct {
	sourceDB       sqlx.ExtContext
	targetDB       *sqlx.DB
	projectionRepo repo.ProjectionRepoInterface
	redis          *redis.Client
	authServiceURL string
	log            *slog.Logger
	httpClient     *http.Client
	userCache      map[string]string
}

func NewGapSyncScheduler(
	sourceDB sqlx.ExtContext,
	targetDB *sqlx.DB,
	projectionRepo repo.ProjectionRepoInterface,
	redis *redis.Client,
	authServiceURL string,
	log *slog.Logger,
) *GapSyncScheduler {
	return &GapSyncScheduler{
		sourceDB:       sourceDB,
		targetDB:       targetDB,
		projectionRepo: projectionRepo,
		redis:          redis,
		authServiceURL: authServiceURL,
		log:            log,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		userCache:      make(map[string]string),
	}
}

func (s *GapSyncScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(gapSyncInterval)
	defer ticker.Stop()

	// Run once on startup
	s.syncGaps(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncGaps(ctx)
		}
	}
}

func (s *GapSyncScheduler) syncGaps(ctx context.Context) {
	// Get last sync time from Redis
	var lastSyncTime time.Time
	lastSyncStr, err := s.redis.Get(ctx, gapSyncLastSyncKey).Result()
	if err == nil {
		err := lastSyncTime.UnmarshalText([]byte(lastSyncStr))
		if err != nil {
			s.log.Warn("failed to parse last sync time", "error", err)
			lastSyncTime = time.Now().Add(-5 * time.Minute) // Default: 5 minutes ago
		}
	} else if err == redis.Nil {
		// First run: sync from 5 minutes ago
		lastSyncTime = time.Now().Add(-5 * time.Minute)
	} else {
		s.log.Warn("failed to get last sync time from Redis", "error", err)
		return
	}

	// Fetch messages from source DB since last sync
	var msgs []chatMessage
	err = sqlx.SelectContext(
		ctx, s.sourceDB, &msgs,
		`SELECT id, room_id, author_id, message_type, content, created_at, updated_at, deleted_at
		 FROM chat_messages
		 WHERE created_at > $1
		 ORDER BY created_at ASC, id ASC`,
		lastSyncTime,
	)
	if err != nil {
		s.log.Error("failed to query messages from source DB", "error", err)
		return
	}

	if len(msgs) == 0 {
		s.log.Debug("no new messages to sync", "since", lastSyncTime)
		return
	}

	s.log.Info("syncing messages", "count", len(msgs), "since", lastSyncTime)

	// Sync messages to target DB
	tx, err := s.targetDB.BeginTx(ctx, nil)
	if err != nil {
		s.log.Error("failed to begin transaction", "error", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO message_projections (id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at)
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
		s.log.Error("failed to prepare statement", "error", err)
		_ = tx.Rollback()
		return
	}
	defer func() { _ = stmt.Close() }()

	var inserted int
	var lastProcessedTime time.Time
	var lastProcessedId uuid.UUID

	for _, msg := range msgs {
		authorName, err := s.getUserName(ctx, msg.AuthorId)
		if err != nil {
			s.log.Error("failed to resolve author name",
				"message_id", msg.Id,
				"author_id", msg.AuthorId,
				"error", err)
			// Continue with empty name rather than failing entire sync
			authorName = ""
		}

		_, err = stmt.ExecContext(ctx,
			msg.Id, msg.RoomId, msg.AuthorId, authorName,
			msg.MessageType, msg.Content, msg.CreatedAt, msg.UpdatedAt, msg.DeletedAt,
		)
		if err != nil {
			s.log.Error("failed to insert message", "id", msg.Id, "error", err)
			tx.Rollback()
			return
		}

		inserted++
		lastProcessedTime = msg.CreatedAt
		lastProcessedId = uuid.MustParse(msg.Id)
	}

	if err := tx.Commit(); err != nil {
		s.log.Error("failed to commit transaction", "error", err)
		return
	}

	// Update high-water mark in Redis
	nowBytes, _ := time.Now().MarshalText()
	err = s.redis.Set(ctx, gapSyncLastSyncKey, string(nowBytes), 0).Err()
	if err != nil {
		s.log.Warn("failed to update last sync time in Redis", "error", err)
	}

	s.log.Info("gap sync completed",
		"synced", inserted,
		"last_message_time", lastProcessedTime,
		"last_message_id", lastProcessedId.String(),
	)
}

func (s *GapSyncScheduler) getUserName(ctx context.Context, userId string) (string, error) {
	if name, ok := s.userCache[userId]; ok {
		return name, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.authServiceURL+"/user/"+userId, nil)
	if err != nil {
		return "", fmt.Errorf("create user request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
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

	s.userCache[userId] = wrapped.Data.Name
	return wrapped.Data.Name, nil
}
