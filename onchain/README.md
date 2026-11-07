# Pact on-chain program

This is the Anchor workspace for the future token escrow program. It currently implements `initialize` only. Escrow instructions have account layouts and validation scaffolding but intentionally return `NotImplemented` without moving tokens.

The toolchain is pinned in `Anchor.toml`: Anchor 1.2.2 and Solana/Agave 4.3.2. Install/select the project toolchain with AVM, then run:

```sh
npm ci
anchor build
anchor test
```

PDA seeds and account relationships are documented in `../docs/onchain-design.md`. `target/deploy/pact_escrow-keypair.json` is a generated local program keypair and is ignored by Git. Never replace it with a funded user key or commit key material.
