import { createClient } from '@solana/kit';
import { solanaRpc } from '@solana/kit-plugin-rpc';
import { walletSigner } from '@solana/kit-plugin-wallet';
import { useClient } from '@solana/react';
import { toSolanaChainIdentifier } from './chain';

const configuredRpcUrl = import.meta.env.VITE_SOLANA_RPC_URL?.trim() || 'https://api.devnet.solana.com';
export const rpcUrl = new URL(configuredRpcUrl, window.location.origin).toString();
export const walletChain = toSolanaChainIdentifier(import.meta.env.VITE_SOLANA_CLUSTER);

export const solanaClient = createClient()
  .use(walletSigner({ chain: walletChain }))
  .use(solanaRpc({ rpcUrl }));

export type SolanaClient = Awaited<typeof solanaClient>;

export function useSolanaClient(): SolanaClient {
  return useClient<SolanaClient>();
}
