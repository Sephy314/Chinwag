package structs

import (
	"time"
)

// AdminUserResponse is the admin-safe projection of a user. Authentication
// secrets (password hash, refresh tokens, DPoP keys) are never included.
type AdminUserResponse struct {
	Id        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	Provider  string     `json:"provider,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type ListUsersRequest struct {
	Cursor  string `query:"cursor"`
	Limit   int    `query:"limit"`
	Search  string `query:"q"`
	Role    string `query:"role"`
	Deleted string `query:"deleted"` // "", "only", "include"
}

type CreateAdminUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role,omitempty"`
}

type UpdateAdminUserRequest struct {
	Name     *string `json:"name,omitempty"`
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role"`
}

// AdminSession is the safe representation of a refresh-token lineage. The raw
// refresh token value is never exposed.
type AdminSession struct {
	LineageId string `json:"lineage_id"`
	UserId    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
	Used      bool   `json:"used"`
	Revoked   bool   `json:"revoked"`
	Jkt       string `json:"jkt,omitempty"`
	Tokens    int    `json:"tokens"`
}

type ListSessionsRequest struct {
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit"`
}

type ListAuditRequest struct {
	Cursor     string `query:"cursor"`
	Limit      int    `query:"limit"`
	AdminId    string `query:"admin_id"`
	Action     string `query:"action"`
	TargetType string `query:"target_type"`
}

type AuditEvent struct {
	Id         string         `json:"id"`
	AdminId    string         `json:"admin_id"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetId   string         `json:"target_id"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type StatsResponse struct {
	Count int64 `json:"count"`
}

// CreateAuditRequest is the internal, system-to-system payload used by other
// services to record audit events in the auth-owned audit log.
type CreateAuditRequest struct {
	AdminId    string         `json:"admin_id"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetId   string         `json:"target_id"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
