# On-chain design

## Toolchain

The workspace pins Anchor CLI/framework 1.2.2 and Solana/Agave 4.1.2, the toolchain recommended in Anchor 1.2.2 release notes. Rust edition 2021 is used. Keep Anchor CLI, `anchor-lang`, `anchor-spl`, and the TypeScript Anchor client on the same 1.2.2 line.

## Accounts and seeds

- `MarketplaceConfig`: singleton PDA derived from `[b"marketplace_config"]`. It stores the administrator, accepted mint, and a version field reserved for safe configuration evolution.
- `Escrow`: one PDA per order, derived from `[b"escrow", order_id]`, where `order_id` is exactly 32 canonical bytes. It stores order ID, buyer, seller, token mint, token amount, status, creation time, and expiry time.
- `Escrow vault`: an associated token account whose wallet/authority is the Escrow PDA and whose mint is the configured mint. The token account is not the authority; the Escrow PDA is. The account constraint uses the classic SPL Token program, keeping the first version small and auditable.

The Anchor account constraints validate PDA seeds, canonical bumps, mint and token-account relationships. Future token transfers should use `transfer_checked` CPI and signer seeds for the escrow PDA when releasing or refunding vault funds. No production USDC mint is embedded; the administrator selects the accepted mint during initialization.

## Current implementation

`initialize` creates the config PDA once. Lifecycle instruction names and account layouts are established for `create_escrow`, `fund_escrow`, `release_escrow`, `refund_escrow`, `resolve_dispute`, and `refund_after_timeout`, but each currently returns `NotImplemented`. They do not create an escrow, accept a deposit, or transfer tokens. Their validation and transition rules need to be implemented alongside adversarial tests before deployment.

Initial test coverage verifies config PDA initialization, stored values, and rejection of a second initialization. The escrow account structure and PDA are architectural scaffolding only until `create_escrow` is implemented.

## Planned safety rules

Before implementing token movement, specify who may create orders, whether order IDs are globally unique, amount bounds, expiry semantics, token decimals and mint validation, seller/buyer signer rules, reentrancy/duplicate-call protection, and admin dispute powers. Every state transition should be one-way and checked against the current enum state. Test unauthorized callers, wrong mint/vault, expired and unexpired cases, duplicate deposits, and exact token balance deltas.
