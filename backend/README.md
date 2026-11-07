# Pact backend

The backend is an explicit-dependency Go `net/http` application with SQLite persistence, goose migrations, sqlc-generated query code, structured logs, wallet-signature authentication, and a Solana RPC verification boundary. It never stores wallet keys or submits transactions for users.

## Run and test

From the repository root:

```sh
make backend-run
make backend-test
make backend-build
make sqlc
```

Or from this directory:

```sh
go mod download
go test ./...
go run -buildvcs=false ./cmd/api
```

The server applies the embedded goose migrations at startup. `sql/schema.sql` is the final schema snapshot used by sqlc; update it with every migration. Files in `internal/data` with the generated header must be regenerated with `make sqlc`, never edited by hand.

## Configuration

See the root `.env.example`. The backend reads environment variables directly and does not load dotenv files. Set `PACT_ALLOWED_ORIGINS` to the browser origins that may make cookie-authenticated writes. The `https://*.trycloudflare.com` entry allows any HTTPS Cloudflare quick-tunnel subdomain, which is convenient for temporary development URLs but unsafe for production; use explicit origins in deployed environments. Set `PACT_COOKIE_SECURE=true` behind HTTPS. Use a devnet/localnet RPC URL and matching program ID/mint while developing.

Escrow-backed order creation is disabled unless `PACT_USDC_MINT` is configured to the accepted classic SPL Token mint. Listings use integer minor units: kobo for NGN, cents for USD, and micro-USDC for USDC. There is no implicit FX conversion.

## Authentication

1. `POST /auth/challenge` with `wallet_address` returns a one-time message and nonce.
2. The browser wallet signs the exact message; Pact verifies the Ed25519 signature.
3. `POST /auth/verify` with wallet, nonce, and signature creates a profile on first sign-in (requiring name and contact email) or signs in an existing wallet. It sets an opaque, database-backed `HttpOnly`, `SameSite=Lax` session cookie.
4. `GET /auth/me` returns the current profile; `POST /auth/logout` revokes the session.

Challenges expire after five minutes and can be requested at most ten times per wallet in a rolling fifteen-minute window. Email is contact information only; it is not verified or used for authentication. All write requests require an allowed `Origin`. Production must use HTTPS and `PACT_COOKIE_SECURE=true`.

## HTTP API

- `GET /health`
- `POST /auth/challenge`, `POST /auth/verify`, `GET /auth/me`, `POST /auth/logout`
- `GET /users/{id}` (current user's private profile only)
- `POST /listings`, `GET /listings`, `GET /listings/{id}`, `GET /listings/mine`, `PATCH /listings/{id}`
- `POST /orders`, `GET /orders`, `GET /orders/{id}`, `POST /orders/{id}/fulfill`, `POST /orders/{id}/cancel`
- `GET /orders/{id}/payment`, `POST /orders/{id}/payment/transactions`
- `POST /conversations`, `GET /conversations`, `GET /conversations/{id}/messages`, `POST /conversations/{id}/messages`

The authenticated user determines buyer/seller identity; request bodies cannot choose another user's identity. Order creation reserves inventory in the same SQLite transaction that writes the order/payment and snapshots unit price and total. Orders require a USDC listing, positive quantity, sufficient stock, and a non-self purchase.

Conversations and messages are participant-only REST resources intended for frontend polling. Message content is limited to 4,000 characters. Pagination is bounded and uses `limit`/`offset` for collections and `before_id` for messages.

## Chain verification boundary

`GET /orders/{id}/payment` returns payment data, a stable 32-byte `chain_order_id`, the escrow PDA, accepted mint, amount, and expiry. The frontend prepares and wallet-signs Anchor transactions. It posts the transaction type and signature to `/orders/{id}/payment/transactions`; Pact independently reads the finalized transaction and escrow/vault accounts through Solana RPC. Pending signatures are stored for retry with bounded exponential backoff. Marketplace order state changes only after verified finalized state.

The implemented Anchor path supports create/fund/release and timeout refund, but administrator-controlled refunds and dispute resolution are not implemented. The backend does not treat successful HTTP submission as proof of payment. Safe off-chain cancellation depends on deploying the matching program and configuring the exact RPC URL, program ID, and mint; do not use this project scaffold for real-value custody without a security review.
