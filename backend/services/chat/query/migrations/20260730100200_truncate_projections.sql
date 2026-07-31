-- +goose Up
TRUNCATE TABLE message_projections;

-- +goose Down
-- Data loss is intentional; restore via backfill tool
SELECT 1;
