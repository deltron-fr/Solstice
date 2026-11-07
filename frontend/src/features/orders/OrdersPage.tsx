import { useCallback, useEffect, useState } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { useSignTransaction } from '@solana/react';
import { useConnectedWallet } from '@solana/kit-plugin-wallet/react';
import type { WalletState } from '@solana/kit-plugin-wallet';
import { useAuth } from '../auth/AuthProvider';
import { api, ApiRequestError, type ListingResponse, type OrderResponse, type PaymentResponse } from '../../lib/api/client';
import { useSolanaClient } from '../../lib/solana/client';
import { buildEscrowTransaction, simulateAndSubmit, walletChain } from '../../lib/solana/escrowTransactions';

type OrderRow = { order: OrderResponse; listing: ListingResponse | null; payment: PaymentResponse | null; counterpartyName: string | null; sellerWallet: string | null };
type ConnectedWallet = NonNullable<WalletState['connected']>;

function formatUsdc(minorUnits: number): string {
  return `${(minorUnits / 1_000_000).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })} USDC`;
}

function dateLabel(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString();
}

function PaymentAction({ row, connected, buyerAddress, sellerAddress, onComplete }: {
  row: OrderRow;
  connected: ConnectedWallet;
  buyerAddress: string;
  sellerAddress: string;
  onComplete: () => void;
}) {
  const signTransaction = useSignTransaction(connected.account, walletChain);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);

  async function run(action: 'fund' | 'release' | 'refund') {
    if (row.payment === null) return;
    if (connected.account.address !== buyerAddress) {
      setError('Connect the buyer wallet for this order to continue.');
      return;
    }
    if (!connected.supportedTransactionVersions.has(1)) {
      setError('This wallet does not currently support Solana transaction v1, required for this payment transaction.');
      return;
    }
    setWorking(true); setError(null); setStatus(null);
    try {
      const transaction = await buildEscrowTransaction(action, row.order, row.payment, buyerAddress, sellerAddress);
      const signature = await simulateAndSubmit(transaction, async (bytes) => (await signTransaction({ transaction: new Uint8Array(bytes) })).signedTransaction);
      setStatus(`Transaction submitted: ${signature}. Checking finalized escrow state…`);
      try {
        const result = await api.submitPaymentTransaction(row.order.id, action, signature);
        setStatus(result.status === 'finalized'
          ? 'Escrow transaction finalized and verified.'
          : `Transaction ${result.status}; Pact will verify it against Solana RPC.`);
      } catch (reason) {
        if (reason instanceof ApiRequestError && reason.status === 503) {
          setStatus(`Transaction sent (${signature}); backend verification is temporarily unavailable and will retry.`);
        } else {
          throw new Error(`Transaction sent as ${signature}, but Pact could not record the signature: ${reason instanceof Error ? reason.message : 'API error'}`);
        }
      }
      onComplete();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'The escrow transaction failed.');
    } finally { setWorking(false); }
  }

  return <div className="payment-actions">
    {row.order.status === 'pending_payment' && row.payment?.escrow_state?.status !== 'funded' && <button className="button button-dark" type="button" disabled={working || row.payment === null} onClick={() => { void run('fund'); }}>{working ? 'Waiting for wallet…' : 'Pay securely with USDC'}</button>}
    {row.order.status === 'pending_payment' && row.payment?.escrow_state?.status === 'funded' && <p className="detail-note">Payment is secured on-chain; Pact is updating the order status.</p>}
    {row.order.status === 'fulfilling' && row.payment?.escrow_state?.status === 'funded' && <button className="button button-dark" type="button" disabled={working} onClick={() => { void run('release'); }}>{working ? 'Waiting for wallet…' : 'Confirm receipt & release'}</button>}
    {row.order.status === 'fulfilling' && row.payment?.escrow_state?.status !== 'funded' && <p className="detail-note">Pact is refreshing escrow state before enabling release.</p>}
    {row.payment?.escrow_state?.status === 'funded' && row.payment.escrow_state.expires_at <= Math.floor(Date.now() / 1000) && <button className="button button-outline" type="button" disabled={working} onClick={() => { void run('refund'); }}>{working ? 'Waiting for wallet…' : 'Refund after timeout'}</button>}
    {error !== null && <p className="form-error" role="alert">{error}</p>}
    {status !== null && <p className="success-notice" role="status">{status}</p>}
  </div>;
}

export function OrdersPage() {
  const { user } = useAuth();
  const client = useSolanaClient();
  const connected = useConnectedWallet(client);
  const [searchParams, setSearchParams] = useSearchParams();
  const role = searchParams.get('as') === 'seller' ? 'seller' : 'buyer';
  const [rows, setRows] = useState<OrderRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyOrder, setBusyOrder] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (user === null) { setRows([]); setLoading(false); return; }
    setLoading(true); setError(null);
    try {
      const result = await api.listOrders(role);
      const nextRows = await Promise.all(result.orders.map(async (order): Promise<OrderRow> => {
        const [listing, payment] = await Promise.all([
          api.getListing(order.listing_id).catch(() => null),
          api.getPayment(order.id).catch(() => null),
        ]);
        const counterpartyId = role === 'buyer' ? order.seller_id : order.buyer_id;
        const counterparty = await api.getPublicUser(counterpartyId).catch(() => null);
        return { order, listing, payment, counterpartyName: counterparty?.name ?? null, sellerWallet: role === 'buyer' ? counterparty?.wallet_address ?? null : null };
      }));
      setRows(nextRows);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not load orders.');
    } finally { setLoading(false); }
  }, [role, user]);

  useEffect(() => { void refresh(); }, [refresh]);

  async function markFulfilling(orderId: number) {
    setBusyOrder(orderId); setError(null);
    try { await api.fulfillOrder(orderId); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not update this order.'); }
    finally { setBusyOrder(null); }
  }

  async function cancelOrder(orderId: number) {
    setBusyOrder(orderId); setError(null);
    try { await api.cancelOrder(orderId); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'This order cannot be cancelled yet.'); }
    finally { setBusyOrder(null); }
  }

  if (user === null) return <section className="seller-placeholder page-section"><span className="large-symbol">↗</span><p className="eyebrow">YOUR ORDERS</p><h1>Keep every deal in view.</h1><p>Connect and sign in with your wallet to see your buyer and seller orders.</p><Link className="button button-dark" to="/listings">Browse marketplace</Link></section>;

  const focusOrder = Number(searchParams.get('order'));
  return <section className="page-section orders-page">
    <div className="section-heading"><div><p className="eyebrow">YOUR BUSINESS, IN GOOD HANDS</p><h1>Orders</h1><p className="page-intro">Marketplace order status and on-chain escrow state are shown separately.</p></div><button className="button button-outline" type="button" onClick={() => { void refresh(); }}>Refresh</button></div>
    <div className="order-tabs" role="tablist" aria-label="Order role">
      {(['buyer', 'seller'] as const).map((nextRole) => <button type="button" role="tab" aria-selected={role === nextRole} className={role === nextRole ? 'selected' : ''} key={nextRole} onClick={() => setSearchParams(nextRole === 'buyer' ? {} : { as: nextRole })}>As {nextRole}</button>)}
    </div>
    {error !== null && <p className="form-error" role="alert">{error}</p>}
    {loading ? <p className="loading-state">Loading orders…</p> : rows.length === 0 ? <div className="empty-state"><p>No {role} orders yet.</p><Link className="text-link" to="/listings">Explore marketplace listings</Link></div> : <div className="orders-list">{rows.map((row) => {
      const order = row.order;
      const payment = row.payment?.payment;
      const chainState = row.payment?.escrow_state;
      const counterpartyId = role === 'buyer' ? order.seller_id : order.buyer_id;
      return <article className={`order-card${focusOrder === order.id ? ' order-card-focus' : ''}`} key={order.id}>
        <div className="order-card-top"><div><p className="eyebrow">ORDER #{order.id}</p><h2>{row.listing?.title ?? `Listing #${order.listing_id}`}</h2></div><span className={`status-pill status-${order.status}`}>{order.status.replaceAll('_', ' ')}</span></div>
        <dl className="order-facts"><div><dt>{role === 'buyer' ? 'Seller' : 'Buyer'}</dt><dd>{row.counterpartyName ?? `Account #${counterpartyId}`}</dd></div><div><dt>Quantity</dt><dd>{order.quantity}</dd></div><div><dt>Total</dt><dd>{formatUsdc(order.total_amount)}</dd></div><div><dt>Created</dt><dd>{dateLabel(order.created_at)}</dd></div></dl>
        <div className="escrow-summary"><strong>On-chain escrow</strong><span>{chainState?.status ?? 'Not yet observed by Pact RPC'}</span><span>Funding deadline: {payment ? dateLabel(payment.funding_expires_at) : 'unavailable'}</span><span>Vault balance: {chainState ? formatUsdc(chainState.vault_amount) : 'unknown'}</span>{payment?.escrow_pda_address && <code>{payment.escrow_pda_address}</code>}</div>
        {role === 'seller' && order.status === 'paid' && <button className="button button-dark" type="button" disabled={busyOrder === order.id} onClick={() => { void markFulfilling(order.id); }}>{busyOrder === order.id ? 'Updating…' : 'Mark as fulfilling'}</button>}
        {role === 'buyer' && (order.status === 'pending_payment' || order.status === 'paid' || order.status === 'fulfilling') && payment && row.payment && user.wallet_address && row.sellerWallet && connected && <PaymentAction row={row} connected={connected} buyerAddress={user.wallet_address} sellerAddress={row.sellerWallet} onComplete={() => { void refresh(); }} />}
        {role === 'buyer' && (order.status === 'pending_payment' || order.status === 'paid' || order.status === 'fulfilling') && connected === null && <p className="detail-note">Connect the buyer wallet to sign an escrow transaction.</p>}
        {role === 'buyer' && (order.status === 'pending_payment' || order.status === 'paid' || order.status === 'fulfilling') && connected !== null && row.sellerWallet === null && <p className="detail-note">The seller has not connected a wallet, so this order cannot use on-chain escrow.</p>}
        {role === 'buyer' && order.status === 'pending_payment' && <button className="text-link cancel-order" type="button" disabled={busyOrder === order.id || new Date(payment?.funding_expires_at ?? Date.now()).getTime() + 5 * 60_000 > Date.now()} onClick={() => { void cancelOrder(order.id); }}>Cancel after payment window expires</button>}
      </article>;
    })}</div>}
  </section>;
}
