export function toSolanaChainIdentifier(cluster: string | undefined): `solana:${string}` {
  const configuredCluster = cluster?.trim() || 'devnet';
  const identifier = configuredCluster.startsWith('solana:')
    ? configuredCluster
    : `solana:${configuredCluster}`;
  return identifier as `solana:${string}`;
}
