# Pact on-chain program

This is the Anchor workspace for Pact's token escrow. It implements configuration initialization, escrow creation, exact USDC funding, buyer-confirmed release, and timeout refunds. Administrator refunds and dispute resolution are explicit `NotImplemented` extension points; they do not move tokens.

The toolchain is pinned in `Anchor.toml`: Anchor 1.2.2 and Solana/Agave 4.3.2. Install/select the project toolchain with AVM, then run:

```sh
npm ci
anchor build
anchor test
```

PDA seeds and account relationships are documented in `../docs/onchain-design.md`. `target/deploy/pact_escrow-keypair.json` is a generated local program keypair and is ignored by Git. Never replace it with a funded user key or commit key material.
