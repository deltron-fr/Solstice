# Pact

Pact is a peer-to-peer marketplace for business products and services. The Go service owns marketplace records and verifies chain activity; an Anchor program will eventually custody and enforce escrowed token payments. Listings, conversations, profiles, and inventory stay off-chain.

## Repository layout

- `frontend/` — React + strict TypeScript marketplace shell (Vite, npm).
- `backend/` — Go HTTP API with wallet-signature sessions, SQLite migrations, sqlc-generated data access, marketplace workflows, and a finalized-RPC verification boundary.
- `onchain/` — Anchor workspace with config initialization and explicit escrow instruction placeholders.
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

The frontend currently displays typed mock marketplace data; the API foundation is independently available at `http://localhost:8080`.

## Common commands

```sh
make backend-test       # Go unit and handler tests
make backend-build
make frontend-check     # ESLint and strict TypeScript check
make frontend-build
make onchain-build      # Anchor build; requires pinned Anchor/Solana tools
make onchain-test       # Anchor local-validator tests
make test               # backend + frontend checks; on-chain is separate
```

`make sqlc` regenerates `backend/internal/data` from `backend/sql/` and `backend/sql/schema.sql`. Generated files have a header identifying them. Update the sqlc schema snapshot alongside goose migrations.

## Current scope

The backend now implements wallet-authenticated profile creation, listing management, inventory-backed USDC orders, order state transitions, REST conversations/messages, and RPC-based payment verification/reconciliation. The UI still uses mock listings and its wallet integration does not yet build escrow transactions. The Anchor program only implements marketplace initialization; all escrow lifecycle instructions remain explicit `NotImplemented` placeholders and do not move tokens. As a result, the backend chain boundary is testable with fakes but real payments are not operational until the program is implemented, deployed, and verified.

See [architecture](docs/architecture.md), [on-chain design](docs/onchain-design.md), and [development guide](docs/development.md) before extending the scaffold.
