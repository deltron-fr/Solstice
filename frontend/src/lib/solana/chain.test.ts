import { describe, expect, it } from 'vitest';
import { toSolanaChainIdentifier } from './chain';

describe('toSolanaChainIdentifier', () => {
  it('adds the Wallet Standard namespace to a cluster name', () => {
    expect(toSolanaChainIdentifier('devnet')).toBe('solana:devnet');
  });

  it('does not add the namespace twice', () => {
    expect(toSolanaChainIdentifier('solana:devnet')).toBe('solana:devnet');
  });

  it('defaults to devnet when no cluster is configured', () => {
    expect(toSolanaChainIdentifier(undefined)).toBe('solana:devnet');
  });
});
