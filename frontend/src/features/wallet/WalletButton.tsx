import { useState, type FormEvent } from 'react';
import { useConnect, useConnectedWallet, useDisconnect, useWalletStatus, useWallets } from '@solana/kit-plugin-wallet/react';
import { useSolanaClient, walletChain } from '../../lib/solana/client';
import { useAuth } from '../auth/AuthProvider';

export function WalletButton() {
  const client = useSolanaClient();
  const status = useWalletStatus(client);
  const wallets = useWallets(client);
  const connected = useConnectedWallet(client);
  const connect = useConnect(client);
  const disconnect = useDisconnect(client);
  const { user, loading, signIn, signOut } = useAuth();
  const [showSignIn, setShowSignIn] = useState(false);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [working, setWorking] = useState(false);

  async function handleSignIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setWorking(true);
    setError(null);
    try {
      await signIn(name, email);
      setShowSignIn(false);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Wallet sign-in failed. Please try again.');
    } finally {
      setWorking(false);
    }
  }

  async function handleSignOut() {
    setError(null);
    try {
      await signOut();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not sign out.');
    }
  }

  if (status === 'pending' || status === 'reconnecting' || loading) {
    return <button className="button button-quiet" type="button" disabled>Checking wallet…</button>;
  }

  if (connected !== null && user !== null) {
    const address = connected.account.address;
    return (
      <div className="wallet-menu">
        <span className="wallet-connected"><span className="wallet-dot" /> {user.name || `${address.slice(0, 4)}…${address.slice(-4)}`}</span>
        <button className="button button-quiet" type="button" onClick={() => { void handleSignOut(); void disconnect.dispatch(); }}>Sign out</button>
        {error !== null && <span className="wallet-error" role="alert">{error}</span>}
      </div>
    );
  }

  if (connected !== null) {
    return (
      <div className="wallet-menu">
        <button className="button button-quiet" type="button" onClick={() => setShowSignIn(true)}>Sign in with wallet</button>
        {showSignIn && <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setShowSignIn(false); }}>
          <form className="dialog-card" onSubmit={(event) => { void handleSignIn(event); }}>
            <button className="dialog-close" type="button" aria-label="Close" onClick={() => setShowSignIn(false)}>×</button>
            <p className="eyebrow">WELCOME TO PACT</p>
            <h2>Sign in with your wallet</h2>
            <p className="form-help">We’ll ask your wallet to sign a one-time message. Your private keys stay in your wallet.</p>
            <label>Your name<input autoComplete="name" value={name} onChange={(event) => setName(event.target.value)} required maxLength={120} /></label>
            <label>Contact email<input autoComplete="email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} required /></label>
            {error !== null && <p className="form-error" role="alert">{error}</p>}
            <button className="button button-dark" type="submit" disabled={working}>{working ? 'Waiting for wallet…' : 'Sign message & continue'}</button>
          </form>
        </div>}
      </div>
    );
  }

  if (user !== null) {
    return <div className="wallet-menu"><span className="wallet-connected">{user.name} · wallet disconnected</span><button className="button button-quiet" type="button" onClick={() => { void handleSignOut(); }}>Sign out</button></div>;
  }

  if (wallets.length === 0) {
    return <div className="wallet-unavailable" role="status">
      <span>No wallet found</span>
      <small>Install or unlock a Solana wallet, then reload. This app is looking for wallets on {walletChain}.</small>
    </div>;
  }

  const connectError = connect.error instanceof Error
    ? connect.error.message
    : connect.error === null || connect.error === undefined ? null : String(connect.error);

  return <div className="wallet-options" aria-label="Connect a Solana wallet">
    {wallets.map((wallet) => <button className="button button-quiet" key={wallet.name} type="button" disabled={connect.isRunning} onClick={() => connect.dispatch(wallet)}>{connect.isRunning ? 'Connecting…' : `Connect ${wallet.name}`}</button>)}
    {connectError !== null && <span className="wallet-error" role="alert">Could not connect wallet: {connectError}</span>}
  </div>;
}
