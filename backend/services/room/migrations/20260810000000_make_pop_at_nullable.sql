-- +goose Up
-- Allow rooms without an auto-pop schedule (NULL = no auto pop).
ALTER TABLE rooms ALTER COLUMN pop_at DROP NOT NULL;
ALTER TABLE rooms ALTER COLUMN pop_at DROP DEFAULT;

-- +goose Down
ALTER TABLE rooms ALTER COLUMN pop_at SET DEFAULT NOW() + INTERVAL '1 day';
ALTER TABLE rooms ALTER COLUMN pop_at SET NOT NULL;
