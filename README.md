# Pact

Pact is a peer-to-peer marketplace for business products and services. The Go service owns marketplace records and independently verifies Solana activity; an Anchor program holds and enforces eligible USDC escrow. Listings, conversations, profiles, and inventory stay off-chain.

## Repository layout

- `frontend/` — React + strict TypeScript commerce UI with wallet authentication, listings, orders, messages, and escrow transaction flows (Vite, npm).
- `backend/` — Go HTTP API with wallet-signature sessions, SQLite migrations, sqlc-generated data access, marketplace workflows, and a finalized-RPC verification boundary.
- `onchain/` — Anchor workspace with config initialization, USDC escrow funding/release, timeout refunds, and explicitly deferred dispute/admin refund hooks.
- `docs/` — architecture and local development notes.

## Prerequisites

- Go 1.27.3 or newer in the Go 1.27 series.
- Node.js 24 LTS and npm 11.
- Rust stable; the on-chain app pins Anchor 1.2.2 and Solana/Agave 4.3.2. Install/select these using AVM as described in `docs/development.md`.
- `sqlc` 1.30.0 for regeneration and the goose 3.27.0 library for embedded migrations. `make migrate-up` additionally expects the goose CLI.

The versions above reflect the scaffolded toolchain and the official compatibility notes checked for this project. Update the pins deliberately as a group.

## Run locally

Copy `.env.example` to `.env` if you want to customize configuration, then export it in the shell with `set -a; source .env; set +a`. The backend reads environment variables directly and has defaults for local development; it does not load dotenv files itself.

```sh
make backend-run
make frontend-dev
```

The frontend reads marketplace data from the API and falls back to visibly marked sample listings if the backend is unavailable. The API is available at `http://localhost:8080`; Vite proxies same-origin `/api` requests there.

## Common commands

```sh
make backend-test       # Go unit and handler tests
make backend-build
make frontend-check     # strict TypeScript, ESLint, and frontend unit tests
make frontend-build
make onchain-build      # Anchor build; requires pinned Anchor/Solana tools
make onchain-test       # Anchor local-validator tests
make test               # backend + frontend checks; on-chain is separate
```

`make sqlc` regenerates `backend/internal/data` from `backend/sql/` and `backend/sql/schema.sql`. Generated files have a header identifying them. Update the sqlc schema snapshot alongside goose migrations.

## Current scope

The backend implements wallet-authenticated profiles, listing management, inventory-backed USDC orders, conversations/messages, and finalized-RPC payment verification/reconciliation. The frontend supports those APIs and builds/simulates wallet-signed create/fund/release/timeout-refund transactions. The Anchor program's admin refund and dispute-resolution hooks intentionally remain `NotImplemented`. Real payments require matching local configuration, a deployed program, a supported Wallet Standard wallet/network, and backend RPC verification.

See [architecture](docs/architecture.md), [on-chain design](docs/onchain-design.md), and [development guide](docs/development.md) before extending the scaffold.
