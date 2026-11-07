import { getBase58Decoder } from '@solana/kit';
import { useConnectedWallet } from '@solana/kit-plugin-wallet/react';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react';
import { ApiRequestError, api, type UserResponse } from '../../lib/api/client';
import { useSolanaClient } from '../../lib/solana/client';

interface AuthContextValue {
  user: UserResponse | null;
  loading: boolean;
  signIn: (name: string, email: string) => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const client = useSolanaClient();
  const connected = useConnectedWallet(client);
  const [user, setUser] = useState<UserResponse | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    api.getCurrentUser()
      .then((current) => { if (active) setUser(current); })
      .catch((error: unknown) => {
        if (active && !(error instanceof ApiRequestError && error.status === 401)) {
          console.error('Could not restore the Pact session', error);
        }
      })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    if (loading || user === null || connected === null || user.wallet_address === connected.account.address) return;
    void api.logout().catch((error: unknown) => console.error('Could not clear the previous wallet session', error));
  }, [connected, loading, user]);

  const signIn = useCallback(async (name: string, email: string) => {
    if (connected === null) throw new Error('Connect a Solana wallet before signing in');
    const walletAddress = connected.account.address;
    const challenge = await api.createChallenge(walletAddress);
    const signature = await client.wallet.signMessage(new TextEncoder().encode(challenge.message));
    const signatureBase58 = getBase58Decoder().decode(signature);
    const signedInUser = await api.verifyChallenge({
      wallet_address: walletAddress,
      nonce: challenge.nonce,
      signature: signatureBase58,
      name: name.trim(),
      email: email.trim(),
    });
    setUser(signedInUser);
  }, [client, connected]);

  const signOut = useCallback(async () => {
    await api.logout();
    setUser(null);
  }, []);

  const activeUser = connected !== null && user?.wallet_address !== connected.account.address ? null : user;
  const value = useMemo(() => ({ user: activeUser, loading, signIn, signOut }), [activeUser, loading, signIn, signOut]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (value === null) throw new Error('useAuth must be used within AuthProvider');
  return value;
}
