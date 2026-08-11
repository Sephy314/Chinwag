package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditEvent records an administrative action. It is append-only: there is no
// admin API to update or delete these rows.
type AuditEvent struct {
	Id         string    `db:"id"`
	AdminId    string    `db:"admin_id"`
	Action     string    `db:"action"`
	TargetType string    `db:"target_type"`
	TargetId   string    `db:"target_id"`
	Metadata   []byte    `db:"metadata"`
	CreatedAt  time.Time `db:"created_at"`
}

func NewAuditEvent(adminID, action, targetType, targetID string, metadata []byte) AuditEvent {
	return AuditEvent{
		Id:         uuid.Must(uuid.NewV7()).String(),
		AdminId:    adminID,
		Action:     action,
		TargetType: targetType,
		TargetId:   targetID,
		Metadata:   metadata,
		CreatedAt:  time.Now().UTC(),
	}
}
