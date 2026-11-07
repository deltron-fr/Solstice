-- name: GetUser :one
SELECT id, name, email, wallet_address, created_at FROM users WHERE id = ?;

-- name: CreateUser :one
INSERT INTO users (name, email, wallet_address)
VALUES (?, ?, ?)
RETURNING id, name, email, wallet_address, created_at;

-- name: GetListing :one
SELECT id, seller_id, title, description, price, currency, inventory_quantity, status, created_at, updated_at
FROM listings WHERE id = ? AND status = 'active';

-- name: ListActiveListings :many
SELECT id, seller_id, title, description, price, currency, inventory_quantity, status, created_at, updated_at
FROM listings WHERE status = 'active' ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: CreateListing :one
INSERT INTO listings (seller_id, title, description, price, currency, inventory_quantity, status)
VALUES (?, ?, ?, ?, ?, ?, 'active')
RETURNING id, seller_id, title, description, price, currency, inventory_quantity, status, created_at, updated_at;

-- name: GetOrder :one
SELECT id, buyer_id, seller_id, listing_id, quantity, unit_price, total_amount, status, created_at, updated_at
FROM orders WHERE id = ?;

-- name: ListOrdersForUser :many
SELECT id, buyer_id, seller_id, listing_id, quantity, unit_price, total_amount, status, created_at, updated_at
FROM orders WHERE buyer_id = ? OR seller_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: GetPaymentForOrder :one
SELECT id, order_id, escrow_pda_address, transaction_signature, amount, token_mint, status, created_at, updated_at
FROM payments WHERE order_id = ?;
