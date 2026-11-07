CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    wallet_address TEXT UNIQUE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE TABLE listings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seller_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    price INTEGER NOT NULL CHECK (price > 0), -- smallest unit of currency, e.g. kobo for NGN
    currency TEXT NOT NULL CHECK (currency IN ('NGN', 'USD', 'USDC')),
    category TEXT NOT NULL DEFAULT 'General' CHECK (length(trim(category)) BETWEEN 1 AND 80),
    unit TEXT NOT NULL DEFAULT 'item' CHECK (length(trim(unit)) BETWEEN 1 AND 60),
    location TEXT NOT NULL DEFAULT '' CHECK (length(location) <= 160),
    image_url TEXT NOT NULL DEFAULT '' CHECK (image_url = '' OR image_url LIKE 'https://%'),
    inventory_quantity INTEGER NOT NULL DEFAULT 0 CHECK (inventory_quantity >= 0),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'sold_out', 'archived')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX listings_status_created_idx ON listings(status, created_at DESC);
CREATE INDEX listings_seller_idx ON listings(seller_id);
CREATE TABLE orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    buyer_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    seller_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    listing_id INTEGER NOT NULL REFERENCES listings(id) ON DELETE RESTRICT,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price INTEGER NOT NULL CHECK (unit_price > 0),
    total_amount INTEGER NOT NULL CHECK (total_amount > 0),
    chain_order_id BLOB NOT NULL UNIQUE CHECK (length(chain_order_id) = 32),
    status TEXT NOT NULL DEFAULT 'pending_payment' CHECK (status IN ('pending_payment', 'paid', 'fulfilling', 'completed', 'cancelled', 'disputed')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (buyer_id <> seller_id)
);
CREATE INDEX orders_buyer_created_idx ON orders(buyer_id, created_at DESC);
CREATE INDEX orders_seller_created_idx ON orders(seller_id, created_at DESC);
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
CREATE TABLE payments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    escrow_pda_address TEXT UNIQUE,
    transaction_signature TEXT UNIQUE,
    amount INTEGER NOT NULL CHECK (amount > 0),
    token_mint TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'awaiting_signature' CHECK (status IN ('awaiting_signature', 'submitted', 'confirmed', 'finalized', 'failed', 'refunded', 'released')),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE TABLE conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    buyer_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    seller_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    order_id INTEGER REFERENCES orders(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (buyer_id <> seller_id),
    UNIQUE (buyer_id, seller_id, order_id)
);
CREATE INDEX conversations_buyer_idx ON conversations(buyer_id, created_at DESC);
CREATE INDEX conversations_seller_idx ON conversations(seller_id, created_at DESC);
CREATE UNIQUE INDEX conversations_without_order_idx ON conversations(buyer_id, seller_id) WHERE order_id IS NULL;
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    content TEXT NOT NULL CHECK (length(trim(content)) > 0),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX messages_conversation_created_idx ON messages(conversation_id, created_at);
CREATE TABLE transaction_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    transaction_signature TEXT NOT NULL UNIQUE,
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('fund', 'release', 'refund', 'dispute_resolution')),
    status TEXT NOT NULL CHECK (status IN ('submitted', 'confirmed', 'finalized', 'failed')),
    verification_attempts INTEGER NOT NULL DEFAULT 0 CHECK (verification_attempts >= 0),
    next_verification_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    last_verification_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX transaction_history_order_idx ON transaction_history(order_id, created_at DESC);
CREATE INDEX transaction_history_retry_idx ON transaction_history(status, next_verification_at, id);
