-- +goose Up
ALTER TABLE orders ADD COLUMN chain_order_id BLOB;
UPDATE orders SET chain_order_id = randomblob(32) WHERE chain_order_id IS NULL;
CREATE UNIQUE INDEX orders_chain_order_id_idx ON orders(chain_order_id);
-- +goose StatementBegin
CREATE TRIGGER orders_chain_order_id_required_insert
BEFORE INSERT ON orders
WHEN NEW.chain_order_id IS NULL OR length(NEW.chain_order_id) <> 32
BEGIN
    SELECT RAISE(ABORT, 'chain_order_id must contain exactly 32 bytes');
END;
-- +goose StatementEnd

CREATE TABLE auth_challenges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    wallet_address TEXT NOT NULL,
    nonce TEXT NOT NULL UNIQUE,
    message TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX auth_challenges_wallet_idx ON auth_challenges(wallet_address, created_at DESC);

CREATE TABLE sessions (
    token_hash BLOB PRIMARY KEY CHECK (length(token_hash) = 32),
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX sessions_user_idx ON sessions(user_id);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);

CREATE UNIQUE INDEX conversations_without_order_idx ON conversations(buyer_id, seller_id) WHERE order_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS conversations_without_order_idx;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS auth_challenges;
DROP TRIGGER IF EXISTS orders_chain_order_id_required_insert;
DROP INDEX IF EXISTS orders_chain_order_id_idx;
ALTER TABLE orders DROP COLUMN chain_order_id;
