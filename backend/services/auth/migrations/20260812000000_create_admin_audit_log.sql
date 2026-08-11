-- +goose Up
CREATE TABLE admin_audit_log (
    id          UUID PRIMARY KEY,
    admin_id    VARCHAR(255) NOT NULL,
    action      VARCHAR(64)  NOT NULL,
    target_type VARCHAR(64)  NOT NULL,
    target_id   VARCHAR(255) NOT NULL,
    metadata    JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_created ON admin_audit_log (created_at DESC);
CREATE INDEX idx_audit_admin   ON admin_audit_log (admin_id);
CREATE INDEX idx_audit_target  ON admin_audit_log (target_type, target_id);

-- +goose Down
DROP TABLE IF EXISTS admin_audit_log;
