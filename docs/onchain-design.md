# On-chain design

## Toolchain

The workspace pins Anchor CLI/framework 1.2.2 and Solana/Agave 4.3.2. Rust edition 2021 is used. Keep Anchor CLI, `anchor-lang`, `anchor-spl`, and the TypeScript Anchor client on the same 1.2.2 line.

## Accounts and seeds

- `MarketplaceConfig`: singleton PDA derived from `[b"marketplace_config"]`. It stores the administrator, accepted mint, and a version field reserved for safe configuration evolution.
- `Escrow`: one PDA per order, derived from `[b"escrow", order_id]`, where `order_id` is exactly 32 canonical bytes. It stores order ID, buyer, seller, token mint, token amount, status, creation time, funding expiry, funded time, post-funding expiry, and bump.
- `Escrow vault`: an associated token account whose wallet/authority is the Escrow PDA and whose mint is the configured mint. The token account is not the authority; the Escrow PDA is. The account constraint uses the classic SPL Token program, keeping the first version small and auditable.

The Anchor account constraints validate PDA seeds, canonical bumps, mint and token-account relationships. Funding uses `transfer_checked` from the buyer's associated token account. Release and timeout refund use `transfer_checked` CPI signed by the escrow PDA. No production USDC mint is embedded; the administrator selects the accepted mint during initialization.

## Current implementation

`initialize` creates the config PDA once. `create_escrow` creates the order PDA and canonical vault; `fund_escrow` deposits the exact amount; `release_escrow` pays the seller after buyer confirmation; and `refund_after_timeout` handles an expired unfunded order or returns a funded escrow after its fulfillment window. `refund_escrow` and `resolve_dispute` deliberately return `NotImplemented` until the dispute/admin policy is designed.

The funding window is 30 minutes from order creation. While the escrow is `Created`, funding is rejected after `funding_expires_at`; after expiry, anyone may mark it `Refunded` without a token transfer because the vault is empty. Successful funding records `funded_at` and starts a separate seven-day fulfillment window in `expires_at`. The buyer may release while `Funded` and before that deadline; after it, anyone may trigger a return to the buyer. Released/refunded terminal escrows cannot be funded or released again.

Anchor tests cover config initialization, exact token funding, timeout protection before expiry, and buyer-confirmed release. Rust host `cargo check`/`cargo test` can validate the crate without SBF tooling; `anchor build` and the local-validator tests additionally need the Solana SBF platform-tools installed.

## Planned safety rules

Before deployment, review who may initialize configuration, amount bounds, expiry semantics, token decimals/mint validation, seller/buyer signer rules, duplicate-call protection, and admin dispute powers. Every state transition must remain checked against the current enum state. Add adversarial coverage for wrong mint/vault, expired and unexpired cases, wrong signers, duplicate deposits, and exact token balance deltas.
