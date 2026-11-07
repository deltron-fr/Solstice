# Pact architecture

## Ownership boundaries

The React frontend is the user interface. It talks to the Go API over HTTP for users, listings, orders, and conversations. For a future escrow transaction, it may also build a transaction and ask the user's browser wallet to sign and submit it.

The Go backend owns conventional marketplace data in SQLite: profiles, listing details and inventory, orders, conversations, messages, and a local record of payments and transaction history. Wallet signatures authenticate users; server-side opaque sessions authorize API actions. The backend is the source of application workflow state. It verifies submitted signatures against finalized Solana RPC transactions and escrow/vault state before recording a payment as secured; a frontend success message alone is never proof of payment.

The Anchor program owns only the minimum financial state: marketplace configuration, one escrow state account per order, and the token vault required to hold escrow funds. It does not store product data, profiles, messages, or inventory. SPL Token (or a deliberately chosen Token-2022 deployment in a future design) performs token movement through CPI. This scaffold uses the classic SPL Token program model.

```mermaid
flowchart LR
  UI[React frontend] -->|HTTP marketplace API| API[Go backend]
  UI -->|wallet-signed transaction, future| RPC[Solana RPC]
  API -->|verify signatures and read accounts| RPC
  RPC --> P[Anchor program]
  P -->|CPI, future| T[SPL Token program]
  API --> DB[(SQLite)]
```

SQLite is the selected database. PostgreSQL is not part of this architecture.

## Three distinct states

An order's marketplace status describes the business workflow (for example `pending_payment`, `paid`, `fulfilling`, `completed`, or `cancelled`). An escrow status describes program state (for example `created`, `funded`, `released`, `refunded`, or `disputed`). Transaction submission status describes a particular RPC transaction (`submitted`, `confirmed`, `finalized`, or `failed`).

These states are related but not interchangeable. A transaction signature can be submitted yet fail; an order can be awaiting payment while its escrow PDA is initialized; and an on-chain escrow can be funded while the seller has not fulfilled the order. The backend stores transaction submissions separately and a retry worker reconciles finalized chain state into order/payment transitions with an auditable transaction-history entry.

## Trust and integration boundary

Wallet signing stays in the frontend. The backend never receives or stores user private keys. The Go blockchain package derives the escrow PDA, reads version-aware finalized transactions, validates the expected Anchor instruction/account set, verifies escrow account ownership/layout and classic SPL token vault state, and retries pending signature checks. It does not shell out to Rust. The current Anchor lifecycle instructions still return `NotImplemented`, so no actual token transition can pass end-to-end verification yet.

Each order stores a cryptographically random, unique 32-byte `chain_order_id`, returned as hex and used directly with the program's `escrow` seed. The payment record includes a 30-minute expiry passed to escrow creation; cancellation waits for the expiry and a finality grace period, and checks finalized escrow state before restoring inventory. This is only safe once the deployed program enforces that exact expiry before funding. Do not enable real-value use until that on-chain check and lifecycle are implemented and audited.
