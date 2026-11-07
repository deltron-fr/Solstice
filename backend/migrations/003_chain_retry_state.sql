-- +goose Up
ALTER TABLE transaction_history ADD COLUMN verification_attempts INTEGER NOT NULL DEFAULT 0 CHECK (verification_attempts >= 0);
ALTER TABLE transaction_history ADD COLUMN next_verification_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
ALTER TABLE transaction_history ADD COLUMN last_verification_error TEXT NOT NULL DEFAULT '';
CREATE INDEX transaction_history_retry_idx ON transaction_history(status, next_verification_at, id);

-- +goose Down
DROP INDEX IF EXISTS transaction_history_retry_idx;
ALTER TABLE transaction_history DROP COLUMN last_verification_error;
ALTER TABLE transaction_history DROP COLUMN next_verification_at;
ALTER TABLE transaction_history DROP COLUMN verification_attempts;
