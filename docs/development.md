# Development guide

## Tool versions

The scaffold targets Go 1.27.3, Node.js 24 LTS/npm 11, sqlc 1.31.1, goose 3.27.0, Rust stable, Anchor 1.2.2, and Solana/Agave 4.1.2. Frontend package versions are pinned in `frontend/package.json` and the resolved graph belongs in `package-lock.json`.

Install sqlc and goose using their official installation instructions. The backend SQLite driver is `github.com/ncruces/go-sqlite3/driver`, a pure Go `database/sql` driver; no CGO or ORM is used.

For on-chain work, use the official Anchor installation guide and AVM to install/select Anchor 1.2.2. The project's toolchain pin selects its documented Solana CLI 4.1.2 and platform tools. AVM can resolve and install the matching toolchain for the project.

## Backend

```sh
cd backend
go mod download
sqlc generate
goose -dir migrations sqlite3 'file:pact.db' up
go run ./cmd/api
go test ./...
```

The server applies embedded goose migrations at startup so a fresh local run initializes SQLite without a separate command. `make migrate-up` is useful for inspecting/operating migrations separately. Set `PACT_DATABASE_DSN`, `PACT_HTTP_ADDR`, and `PACT_SOLANA_RPC_URL` to override defaults.

## Frontend

```sh
cd frontend
npm ci
npm run dev
npm run check
npm run build
```

The wallet foundation uses Solana Kit, `@solana/react`, and the Kit Wallet Standard plugin. It only discovers/connects browser wallets at this stage. A future payment flow will request a prepared escrow transaction, request wallet signing, submit through Kit/RPC, obtain a signature, then submit the signature to the Go API for independent chain verification.

## On-chain

```sh
cd onchain
anchor build
anchor test
```

`anchor test` needs the pinned CLI/tool suite and a local validator. The checked-in test uses Anchor's TypeScript client. `target/deploy/pact_escrow-keypair.json` is a generated local deployment key ignored by Git; regenerate it with `solana-keygen` and update the program ID in `Anchor.toml` and `src/lib.rs` if you discard it. Never place a funded keypair in this repository; use a disposable local/development wallet and keep credentials outside source control.

Anchor's local provider defaults to `~/.config/solana/id.json`. If you do not already have a disposable local wallet there, create one with `solana-keygen new` before running tests. The test validator funds its own local ledger; no devnet funds are needed for `anchor test`.

## sqlc generated code

Run `make sqlc` after editing a query in `backend/sql/` or changing the schema. Goose migrations are authoritative at runtime; `backend/sql/schema.sql` is the schema snapshot sqlc uses for type generation, so update it alongside a migration. Do not hand-edit files in `backend/internal/data` with the generated header.
