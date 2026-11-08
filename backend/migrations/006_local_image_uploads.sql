-- +goose NO TRANSACTION
-- +goose Up
-- SQLite cannot change a column CHECK constraint directly. Rebuild listings
-- with foreign-key checks disabled on this dedicated migration connection.
PRAGMA foreign_keys = OFF;

CREATE TABLE listings_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seller_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    price INTEGER NOT NULL CHECK (price > 0),
    currency TEXT NOT NULL CHECK (currency IN ('NGN', 'USD', 'USDC')),
    category TEXT NOT NULL DEFAULT 'General' CHECK (length(trim(category)) BETWEEN 1 AND 80),
    unit TEXT NOT NULL DEFAULT 'item' CHECK (length(trim(unit)) BETWEEN 1 AND 60),
    location TEXT NOT NULL DEFAULT '' CHECK (length(location) <= 160),
    image_url TEXT NOT NULL DEFAULT '' CHECK (image_url = '' OR image_url LIKE 'https://%' OR image_url LIKE '/uploads/%'),
    inventory_quantity INTEGER NOT NULL DEFAULT 0 CHECK (inventory_quantity >= 0),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'sold_out', 'archived')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO listings_new (id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at)
SELECT id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at
FROM listings;

DROP TABLE listings;
ALTER TABLE listings_new RENAME TO listings;
CREATE INDEX listings_status_created_idx ON listings(status, created_at DESC);
CREATE INDEX listings_seller_idx ON listings(seller_id);
PRAGMA foreign_keys = ON;

-- +goose Down
-- Local upload files remain on disk; this downgrade clears their listing paths.
PRAGMA foreign_keys = OFF;

CREATE TABLE listings_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seller_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    price INTEGER NOT NULL CHECK (price > 0),
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

INSERT INTO listings_old (id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at)
SELECT id, seller_id, title, description, price, currency, category, unit, location,
       CASE WHEN image_url LIKE '/uploads/%' THEN '' ELSE image_url END,
       inventory_quantity, status, created_at, updated_at
FROM listings;

DROP TABLE listings;
ALTER TABLE listings_old RENAME TO listings;
CREATE INDEX listings_status_created_idx ON listings(status, created_at DESC);
CREATE INDEX listings_seller_idx ON listings(seller_id);
PRAGMA foreign_keys = ON;
