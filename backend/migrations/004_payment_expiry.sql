-- +goose Up
ALTER TABLE payments ADD COLUMN expires_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00Z';

-- +goose Down
ALTER TABLE payments DROP COLUMN expires_at;
