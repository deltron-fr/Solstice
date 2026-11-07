import type { PropsWithChildren } from 'react';
import { ClientProvider } from '@solana/react';
import { solanaClient } from '../../lib/solana/client';

export function SolanaProvider({ children }: PropsWithChildren) {
  return <ClientProvider client={solanaClient}>{children}</ClientProvider>;
}
