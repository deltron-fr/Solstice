# Development guide

## Tool versions

The scaffold targets Go 1.27.3, Node.js 24 LTS/npm 11, sqlc 1.30.0, goose library 3.27.0, Solana Go SDK 2.3.0, Rust stable, Anchor 1.2.2, and Solana/Agave 4.3.2. Frontend package versions are pinned in `frontend/package.json` and the resolved graph belongs in `package-lock.json`.

Install sqlc and goose using their official installation instructions. The backend SQLite driver is `github.com/ncruces/go-sqlite3/driver`, a pure Go `database/sql` driver; no CGO or ORM is used.

For on-chain work, use the official Anchor installation guide and AVM to install/select Anchor 1.2.2 and Solana/Agave 4.3.2. Run `anchor --version` and `solana --version` to confirm the selected versions before building. On this development machine the Solana CLI is already at 4.3.2; older AVM installations may need updating to resolve it.

## Backend

```sh
cd backend
go mod download
sqlc generate
goose -dir migrations sqlite3 'file:solstice.db' up
go run ./cmd/api
go test ./...
```

The server applies embedded goose migrations at startup so a fresh local run initializes SQLite without a separate command. `make migrate-up` is useful for inspecting/operating migrations separately. Set `SOLSTICE_DATABASE_DSN`, `SOLSTICE_HTTP_ADDR`, `SOLSTICE_SOLANA_RPC_URL`, `SOLSTICE_SOLANA_PROGRAM_ID`, `SOLSTICE_USDC_MINT`, `SOLSTICE_ALLOWED_ORIGINS`, and `SOLSTICE_UPLOAD_DIR` as documented in the root `.env.example`. For the Cloudflare tunnel UI, include its exact HTTPS origin in `SOLSTICE_ALLOWED_ORIGINS`; use `SOLSTICE_COOKIE_SECURE=true` for HTTPS deployment.

Uploaded listing images are written under `backend/var/uploads` by default. Keep that directory on persistent storage and back it up with SQLite. The existing local database was renamed to `backend/solstice.db`; its data was preserved.

Wallet sign-in requires browser requests to send an allowed `Origin`. The Vite `/api` proxy keeps local development same-origin; production should similarly serve/proxy the API under the UI origin. Escrow initialization, funding, buyer release, and timeout refund are implemented in Anchor; administrator refund and dispute resolution remain explicit `NotImplemented` extension points. The backend only advances application payment/order state after independent finalized RPC verification.

## Frontend

```sh
cd frontend
npm ci
npm run dev
npm run check
npm run build
```

The frontend uses Solana Kit, `@solana/react`, and the Kit Wallet Standard plugin. It signs one-time backend login challenges, uses the marketplace API for listings/orders/messages, and builds the current create/fund/release Anchor instructions locally. The browser simulates each transaction, requests wallet signing, submits through configured RPC, and posts the signature to the backend for independent finalized-state verification. The hand-encoded Anchor instruction builder is intentionally small and should be replaced by a generated Codama client when client generation is established.

The frontend defaults to devnet because browser wallets commonly advertise devnet and may not advertise localnet. The backend example defaults to localnet. For a full payment test, configure both applications and the connected wallet for the same deployed program ID, RPC/network, and test mint: set `SOLSTICE_SOLANA_RPC_URL=https://api.devnet.solana.com` and the matching `SOLSTICE_USDC_MINT` for Go, and set `VITE_SOLANA_RPC_URL=https://api.devnet.solana.com`, `VITE_SOLANA_CLUSTER=devnet`, `VITE_SOLANA_PROGRAM_ID`, and `VITE_SOLANA_USDC_MINT` for Vite. Do not use a mainnet mint or real funds with this scaffold.

For VM deployment, Docker image builds, required GitHub secrets/variables, and persistent storage notes, see [the deployment runbook](../deploy/README.md). Production frontend images route `/solana-rpc` through Caddy, while local Vite development uses the proxy in `vite.config.ts`.

## On-chain

```sh
cd onchain
anchor build
anchor test
```

`anchor test` needs the pinned CLI/tool suite and a local validator. The checked-in test uses Anchor's TypeScript client. `target/deploy/solstice_escrow-keypair.json` is a generated local deployment key ignored by Git; regenerate it with `solana-keygen` and update the program ID in `Anchor.toml` and `src/lib.rs` if you discard it. Never place a funded keypair in this repository; use a disposable local/development wallet and keep credentials outside source control.

Anchor's local provider defaults to `~/.config/solana/id.json`. If you do not already have a disposable local wallet there, create one with `solana-keygen new` before running tests. The test validator funds its own local ledger; no devnet funds are needed for `anchor test`.

## sqlc generated code

Run `make sqlc` after editing a query in `backend/sql/` or changing the schema. Goose migrations are authoritative at runtime; `backend/sql/schema.sql` is the schema snapshot sqlc uses for type generation, so update it alongside a migration. Do not hand-edit files in `backend/internal/data` with the generated header.
