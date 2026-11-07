# Pact frontend

React 19 and strict TypeScript run on Vite 8. npm is used for dependency management.

```sh
npm ci
npm run dev
npm run check
npm run build
```

The frontend is connected to the Go API for listing browse/detail, seller listing management, wallet-message sign-in, conversations, orders, fulfillment, and payment status. Sample listings are shown with an explicit notice if the API is unavailable. Browser wallet sign-in signs the server's one-time challenge; keys are never read or stored by Pact.

The wallet payment flow uses Solana Kit and Wallet Standard. The compact escrow instruction builder currently covers `create_escrow` + `fund_escrow` and buyer-confirmed `release_escrow`, simulates before asking the wallet to sign, submits through the configured Solana RPC, then submits the signature to Go for independent finalized-state verification. It encodes the small Anchor instruction/account layouts in `src/lib/solana/escrowTransactions.ts`; keep these synchronized with the Anchor program (or replace them with a Codama-generated client when the IDL/client generation workflow is added). Administrator refunds/disputes are not exposed in the UI.

Copy `.env.example` to `.env.local` and set `VITE_SOLANA_RPC_URL`, `VITE_SOLANA_CLUSTER`, `VITE_SOLANA_PROGRAM_ID`, and `VITE_SOLANA_USDC_MINT` to one matching network. Devnet is the default because browser wallets commonly advertise devnet, while many do not advertise localnet; the wallet plugin only lists wallets that support the configured chain. For localnet, configure a wallet that supports the custom local cluster and set both `VITE_SOLANA_RPC_URL=http://127.0.0.1:8899` and `VITE_SOLANA_CLUSTER=localnet`. Checkout only accepts USDC-priced listings; prices are entered in major units and stored as kobo, cents, or micro-USDC.

Vite's `/api` proxy forwards to the local Go server. The configured tunnel hostname is `huge-facilitate-hayes-morris.trycloudflare.com`; add its exact HTTPS origin to `PACT_ALLOWED_ORIGINS` on the backend for authenticated writes.
