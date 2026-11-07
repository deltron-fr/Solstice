# Pact

Pact is a peer-to-peer marketplace for business products and services. The Go service owns marketplace records and verifies chain activity; an Anchor program will eventually custody and enforce escrowed token payments. Listings, conversations, profiles, and inventory stay off-chain.

## Repository layout

- `frontend/` — React + strict TypeScript marketplace shell (Vite, npm).
- `backend/` — Go HTTP API, SQLite migrations, and sqlc-generated data access.
- `onchain/` — Anchor workspace with config initialization and explicit escrow instruction placeholders.
- `docs/` — architecture and local development notes.

## Prerequisites

- Go 1.27.3 or newer in the Go 1.27 series.
- Node.js 24 LTS and npm 11.
- Rust stable; the on-chain app pins Anchor 1.2.2 and Solana/Agave 4.1.2. Install/select these using AVM as described in `docs/development.md`.
- `sqlc` 1.31.1 and `goose` 3.27.0 for regeneration and migrations. Root Make targets invoke them by name.

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

This repository is a foundation, not a complete marketplace. User creation/read and listing creation/read/list are persistence-backed; listing updates, order, and payment routes are registered and return explicit `501` responses until their workflows are designed. The only implemented Anchor instruction initializes marketplace configuration. Escrow lifecycle instructions return an explicit `NotImplemented` error and do not move tokens.

See [architecture](docs/architecture.md), [on-chain design](docs/onchain-design.md), and [development guide](docs/development.md) before extending the scaffold.
