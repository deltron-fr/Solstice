-- name: GetUser :one
SELECT id, name, email, wallet_address, created_at FROM users WHERE id = ?;

-- name: GetUserByWallet :one
SELECT id, name, email, wallet_address, created_at FROM users WHERE wallet_address = ?;

-- name: GetPublicUser :one
SELECT id, name, wallet_address FROM users WHERE id = ?;

-- name: CreateUser :one
INSERT INTO users (name, email, wallet_address)
VALUES (?, ?, ?)
RETURNING id, name, email, wallet_address, created_at;

-- name: GetListing :one
SELECT id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at
FROM listings WHERE id = ? AND status = 'active';

-- name: GetListingForUpdate :one
SELECT id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at
FROM listings WHERE id = ?;

-- name: ListActiveListings :many
SELECT id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at
FROM listings WHERE status = 'active' ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?;

-- name: ListListingsForSeller :many
SELECT id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at
FROM listings WHERE seller_id = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?;

-- name: CreateListing :one
INSERT INTO listings (seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at;

-- name: UpdateListing :one
UPDATE listings
SET title = ?, description = ?, price = ?, currency = ?, category = ?, unit = ?, location = ?, image_url = ?, inventory_quantity = ?, status = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND seller_id = ?
RETURNING id, seller_id, title, description, price, currency, category, unit, location, image_url, inventory_quantity, status, created_at, updated_at;

-- name: ReserveListingInventory :execrows
UPDATE listings SET inventory_quantity = inventory_quantity - ?,
    status = CASE WHEN inventory_quantity - ? = 0 THEN 'sold_out' ELSE status END,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'active' AND inventory_quantity >= ?;

-- name: RestoreListingInventory :exec
UPDATE listings SET inventory_quantity = inventory_quantity + ?, status = 'active',
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: CreateOrder :one
INSERT INTO orders (buyer_id, seller_id, listing_id, quantity, unit_price, total_amount, chain_order_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, buyer_id, seller_id, listing_id, quantity, unit_price, total_amount, chain_order_id, status, created_at, updated_at;

-- name: GetOrder :one
SELECT id, buyer_id, seller_id, listing_id, quantity, unit_price, total_amount, chain_order_id, status, created_at, updated_at
FROM orders WHERE id = ?;

-- name: GetOrderForUser :one
SELECT id, buyer_id, seller_id, listing_id, quantity, unit_price, total_amount, chain_order_id, status, created_at, updated_at
FROM orders WHERE id = ? AND (buyer_id = ? OR seller_id = ?);

-- name: ListOrdersForUser :many
SELECT id, buyer_id, seller_id, listing_id, quantity, unit_price, total_amount, chain_order_id, status, created_at, updated_at
FROM orders WHERE (buyer_id = ? AND ? = 'buyer') OR (seller_id = ? AND ? = 'seller')
ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?;

-- name: MarkOrderFulfilling :execrows
UPDATE orders SET status = 'fulfilling', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND seller_id = ? AND status = 'paid';

-- name: CancelUnfundedOrder :execrows
UPDATE orders SET status = 'cancelled', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND buyer_id = ? AND status = 'pending_payment';

-- name: SetOrderPaid :execrows
UPDATE orders SET status = 'paid', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'pending_payment';

-- name: SetOrderCompleted :execrows
UPDATE orders SET status = 'completed', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'fulfilling';

-- name: SetOrderRefunded :execrows
UPDATE orders SET status = 'cancelled', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status IN ('paid', 'fulfilling');

-- name: CreatePayment :one
INSERT INTO payments (order_id, escrow_pda_address, amount, token_mint, status, expires_at)
VALUES (?, ?, ?, ?, 'awaiting_signature', ?)
RETURNING id, order_id, escrow_pda_address, transaction_signature, amount, token_mint, status, expires_at, created_at, updated_at;

-- name: GetPaymentForOrder :one
SELECT id, order_id, escrow_pda_address, transaction_signature, amount, token_mint, status, expires_at, created_at, updated_at
FROM payments WHERE order_id = ?;

-- name: SetPaymentSubmitted :exec
UPDATE payments SET status = 'submitted', transaction_signature = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE order_id = ?;

-- name: SetPaymentStatus :exec
UPDATE payments SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE order_id = ?;

-- name: SetPaymentFailed :exec
UPDATE payments SET status = 'failed', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE order_id = ? AND transaction_signature = ? AND status IN ('awaiting_signature', 'submitted', 'failed');

-- name: GetTransactionHistoryBySignature :one
SELECT id, order_id, transaction_signature, transaction_type, status, created_at
FROM transaction_history WHERE transaction_signature = ?;

-- name: CreateTransactionHistory :one
INSERT INTO transaction_history (order_id, transaction_signature, transaction_type, status)
VALUES (?, ?, ?, 'submitted')
RETURNING id, order_id, transaction_signature, transaction_type, status, created_at;

-- name: UpdateTransactionStatus :exec
UPDATE transaction_history SET status = ?, last_verification_error = '' WHERE transaction_signature = ?;

-- name: RejectTransactionVerification :exec
UPDATE transaction_history SET status = 'failed', last_verification_error = ? WHERE transaction_signature = ? AND status IN ('submitted', 'confirmed');

-- name: ListPendingTransactions :many
SELECT id, order_id, transaction_signature, transaction_type, status, verification_attempts, next_verification_at, last_verification_error, created_at
FROM transaction_history WHERE status IN ('submitted', 'confirmed') AND next_verification_at <= strftime('%Y-%m-%dT%H:%M:%fZ', 'now') ORDER BY id LIMIT ?;

-- name: ScheduleTransactionRetry :exec
UPDATE transaction_history
SET verification_attempts = verification_attempts + 1, next_verification_at = ?, last_verification_error = ?
WHERE transaction_signature = ? AND status IN ('submitted', 'confirmed');

-- name: GetConversation :one
SELECT id, buyer_id, seller_id, order_id, created_at FROM conversations WHERE id = ?;

-- name: GetConversationBetween :one
SELECT id, buyer_id, seller_id, order_id, created_at FROM conversations
WHERE buyer_id = ? AND seller_id = ? AND order_id IS ?;

-- name: CreateConversation :one
INSERT INTO conversations (buyer_id, seller_id, order_id)
VALUES (?, ?, ?)
ON CONFLICT DO NOTHING
RETURNING id, buyer_id, seller_id, order_id, created_at;

-- name: ListConversationsForUser :many
SELECT id, buyer_id, seller_id, order_id, created_at FROM conversations
WHERE buyer_id = ? OR seller_id = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?;

-- name: CreateMessage :one
INSERT INTO messages (conversation_id, sender_id, content)
VALUES (?, ?, ?)
RETURNING id, conversation_id, sender_id, content, created_at;

-- name: ListMessages :many
SELECT id, conversation_id, sender_id, content, created_at FROM messages
WHERE conversation_id = ? AND id < ? ORDER BY id DESC LIMIT ?;

-- name: CreateAuthChallenge :one
INSERT INTO auth_challenges (wallet_address, nonce, message, expires_at)
VALUES (?, ?, ?, ?)
RETURNING id, wallet_address, nonce, message, expires_at, consumed_at, created_at;

-- name: CountRecentAuthChallenges :one
SELECT count(*) FROM auth_challenges
WHERE wallet_address = ? AND created_at > strftime('%Y-%m-%dT%H:%M:%fZ', 'now', '-15 minutes');

-- name: GetAuthChallenge :one
SELECT id, wallet_address, nonce, message, expires_at, consumed_at, created_at
FROM auth_challenges WHERE nonce = ?;

-- name: ConsumeAuthChallenge :execrows
UPDATE auth_challenges SET consumed_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE nonce = ? AND wallet_address = ? AND consumed_at IS NULL AND expires_at > strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

-- name: DeleteExpiredChallenges :exec
DELETE FROM auth_challenges WHERE expires_at <= strftime('%Y-%m-%dT%H:%M:%fZ', 'now') OR consumed_at IS NOT NULL;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?);

-- name: GetSessionUser :one
SELECT users.id, users.name, users.email, users.wallet_address, users.created_at
FROM sessions JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = ? AND sessions.expires_at > strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
