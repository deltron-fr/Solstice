export type WalletConnectionState =
  | { status: 'disconnected' }
  | { status: 'connected'; address: string };

// Wallet discovery/signing is provided by Solana Kit and Wallet Standard.
// No key material is read or stored by Pact.
