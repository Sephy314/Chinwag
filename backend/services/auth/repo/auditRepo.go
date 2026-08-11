package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/jmoiron/sqlx"
)

type auditCursor struct {
	CreatedAt time.Time `json:"created_at"`
	Id        string    `json:"id"`
}

func encodeAuditCursor(createdAt time.Time, id string) string {
	c := auditCursor{CreatedAt: createdAt, Id: id}
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
}

func decodeAuditCursor(s string) (auditCursor, error) {
	var c auditCursor
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

const auditDefaultLimit = 50
const auditMaxLimit = 200

type AuditRepoInterface interface {
	Insert(ctx context.Context, ev domain.AuditEvent) error
	List(ctx context.Context, cursor string, limit int, adminID, action, targetType string) ([]domain.AuditEvent, *structs.CursorMeta, error)
}

type AuditRepo struct {
	db sqlx.ExtContext
}

func NewAuditRepo(db sqlx.ExtContext) AuditRepoInterface {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Insert(ctx context.Context, ev domain.AuditEvent) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO admin_audit_log (id, admin_id, action, target_type, target_id, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		ev.Id, ev.AdminId, ev.Action, ev.TargetType, ev.TargetId, ev.Metadata,
	)
	return err
}

func (r *AuditRepo) List(ctx context.Context, cursor string, limit int, adminID, action, targetType string) ([]domain.AuditEvent, *structs.CursorMeta, error) {
	if limit <= 0 || limit > auditMaxLimit {
		limit = auditDefaultLimit
	}

	where := []string{"1=1"}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "?"
	}

	if adminID != "" {
		where = append(where, "admin_id = "+arg(adminID))
	}
	if action != "" {
		where = append(where, "action = "+arg(action))
	}
	if targetType != "" {
		where = append(where, "target_type = "+arg(targetType))
	}

	fetchLimit := limit + 1

	query := `SELECT id, admin_id, action, target_type, target_id, metadata, created_at
	          FROM admin_audit_log
	          WHERE ` + strings.Join(where, " AND ")

	if cursor == "" {
		query += ` ORDER BY created_at DESC, id DESC LIMIT $` + arg(fetchLimit)
	} else {
		c, cerr := decodeAuditCursor(cursor)
		if cerr != nil {
			return nil, nil, cerr
		}
		query += ` AND (created_at, id) < ($` + arg(c.CreatedAt) + `, $` + arg(c.Id) + `)
		          ORDER BY created_at DESC, id DESC LIMIT $` + arg(fetchLimit)
	}

	query = rebind(query)

	var rows []domain.AuditEvent
	if err := sqlx.SelectContext(ctx, r.db, &rows, query, args...); err != nil {
		return nil, nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	var meta *structs.CursorMeta
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		meta = &structs.CursorMeta{
			NextCursor: encodeAuditCursor(last.CreatedAt, last.Id),
			HasMore:    true,
		}
	}

	return rows, meta, nil
}

// rebind converts "?" placeholders to the driver's native style ($N for pgx).
func rebind(q string) string {
	// Simple positional renumbering for the pgx ($N) dialect.
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
