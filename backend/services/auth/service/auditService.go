package service

import (
	"context"
	"encoding/json"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/repo"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
)

type AuditServiceInterface interface {
	Record(ctx context.Context, adminID, action, targetType, targetID string, metadata map[string]any) error
	List(ctx context.Context, req structs.ListAuditRequest) ([]structs.AuditEvent, *structs.CursorMeta, error)
}

type AuditService struct {
	repo repo.AuditRepoInterface
}

func NewAuditService(auditRepo repo.AuditRepoInterface) AuditServiceInterface {
	return &AuditService{repo: auditRepo}
}

func (s *AuditService) Record(ctx context.Context, adminID, action, targetType, targetID string, metadata map[string]any) error {
	var meta []byte
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		meta = b
	} else {
		meta = []byte("{}")
	}

	return s.repo.Insert(ctx, domain.NewAuditEvent(adminID, action, targetType, targetID, meta))
}

func (s *AuditService) List(ctx context.Context, req structs.ListAuditRequest) ([]structs.AuditEvent, *structs.CursorMeta, error) {
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}
	events, meta, err := s.repo.List(ctx, req.Cursor, req.Limit, req.AdminId, req.Action, req.TargetType)
	if err != nil {
		return nil, nil, err
	}

	out := make([]structs.AuditEvent, len(events))
	for i, ev := range events {
		var metadata map[string]any
		if len(ev.Metadata) > 0 {
			_ = json.Unmarshal(ev.Metadata, &metadata)
		}
		out[i] = structs.AuditEvent{
			Id:         ev.Id,
			AdminId:    ev.AdminId,
			Action:     ev.Action,
			TargetType: ev.TargetType,
			TargetId:   ev.TargetId,
			Metadata:   metadata,
			CreatedAt:  ev.CreatedAt,
		}
	}

	return out, meta, nil
}
