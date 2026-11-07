import { useClient } from '@solana/react';
import { useConnect, useConnectedWallet, useDisconnect, useWalletStatus, useWallets } from '@solana/kit-plugin-wallet/react';
import type { SolanaClient } from '../../lib/solana/client';

export function WalletButton() {
  const client = useClient<SolanaClient>();
  const status = useWalletStatus(client);
  const wallets = useWallets(client);
  const connected = useConnectedWallet(client);
  const connect = useConnect(client);
  const disconnect = useDisconnect(client);

  if (status === 'pending' || status === 'reconnecting') {
    return <button className="button button-quiet" type="button" disabled>Checking wallet…</button>;
  }

  if (connected) {
    const address = connected.account.address;
    return <button className="button button-quiet" type="button" onClick={() => disconnect.dispatch()} title="Disconnect wallet">{`${address.slice(0, 4)}…${address.slice(-4)} · Disconnect`}</button>;
  }

  if (wallets.length === 0) {
    return <button className="button button-quiet" type="button" disabled title="Install a Wallet Standard compatible Solana wallet">No wallet found</button>;
  }

  return (
    <div className="wallet-options" aria-label="Connect a Solana wallet">
      {wallets.map((wallet) => <button className="button button-quiet" key={wallet.name} type="button" disabled={connect.isRunning} onClick={() => connect.dispatch(wallet)}>Connect {wallet.name}</button>)}
      {Boolean(connect.error) && <span className="wallet-error" role="alert">Wallet connection failed</span>}
    </div>
  );
}
