# Pact frontend

React 19 and strict TypeScript run on Vite 8. npm is used for dependency management.

```sh
npm ci
npm run dev
npm run check
npm run build
```

The marketplace and listing detail pages use typed mock listings. The app shell includes seller and order placeholders. The wallet integration uses Solana Kit, `@solana/react`, and the Wallet Standard plugin on devnet; it can discover/connect/disconnect wallets, but no escrow transaction is built or signed yet. Set `VITE_SOLANA_RPC_URL` to override the devnet RPC URL. Browser wallets keep their own keys; Pact does not handle or store private keys.
