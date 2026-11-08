# Pact on-chain program

This is the Anchor workspace for Pact's token escrow. It implements configuration initialization, escrow creation, exact USDC funding, buyer-confirmed release, and timeout refunds. Administrator refunds and dispute resolution are explicit `NotImplemented` extension points; they do not move tokens.

For devnet checkout testing, create a classic SPL test mint with 6 decimals, then initialize the config once with `PACT_USDC_MINT=<mint> ANCHOR_PROVIDER_URL=https://api.devnet.solana.com ANCHOR_WALLET=~/.config/solana/id.json npm run initialize:devnet`. The command verifies that the RPC is devnet, checks the mint, and refuses to silently change an existing configuration. The selected mint is permanent for this config PDA; use a new program ID if you need to test with a different mint.

The toolchain is pinned in `Anchor.toml`: Anchor 1.2.2 and Solana/Agave 4.3.2. Install/select the project toolchain with AVM, then run:

```sh
npm ci
anchor build
anchor test
```

PDA seeds and account relationships are documented in `../docs/onchain-design.md`. `target/deploy/pact_escrow-keypair.json` is a generated local program keypair and is ignored by Git. Never replace it with a funded user key or commit key material.
