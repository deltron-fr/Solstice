import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../auth/AuthProvider';
import { api, type ListingCurrency, type ListingResponse } from '../../lib/api/client';
import { formatListingPrice } from '../listings/formatPrice';

const emptyDraft = {
  title: '', description: '', category: 'Food & produce', unit: 'crate', location: '',
  price: '', currency: 'USDC' as ListingCurrency, image_url: '', inventory_quantity: 1,
};

export function SellerPage() {
  const { user } = useAuth();
  const [listings, setListings] = useState<ListingResponse[]>([]);
  const [draft, setDraft] = useState(emptyDraft);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try { setListings((await api.listMyListings()).listings); setError(null); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not load your listings.'); }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { if (user !== null) void refresh(); }, [refresh, user]);

  async function createListing(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try {
      const maxDecimals = draft.currency === 'USDC' ? 6 : 2;
      const enteredDecimals = draft.price.split('.')[1]?.length ?? 0;
      const price = Number(draft.price) * (draft.currency === 'USDC' ? 1_000_000 : 100);
      if (enteredDecimals > maxDecimals || !Number.isFinite(price) || !Number.isSafeInteger(Math.round(price)) || price <= 0) {
        throw new Error(`Enter a positive price with at most ${maxDecimals} decimal places.`);
      }
      await api.createListing({ ...draft, price: Math.round(price) });
      setDraft(emptyDraft);
      setNotice('Your listing is live in the marketplace.');
      await refresh();
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not create the listing.'); }
    finally { setBusy(false); }
  }

  async function updateStatus(listing: ListingResponse) {
    const status = listing.status === 'active' ? 'paused' : 'active';
    setBusy(true); setError(null);
    try { await api.updateListing(listing.id, { status }); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not update the listing.'); }
    finally { setBusy(false); }
  }

  if (user === null) return <section className="seller-placeholder page-section"><span className="large-symbol">✳</span><p className="eyebrow">SELL ON PACT</p><h1>Your business belongs here.</h1><p>Connect and sign in with your Solana wallet to publish listings and manage orders.</p><Link className="button button-dark" to="/listings">Explore the marketplace</Link></section>;

  return <section className="page-section seller-dashboard">
    <div className="section-heading"><div><p className="eyebrow">SELLER DASHBOARD</p><h1>Welcome, {user.name}.</h1><p className="page-intro">Publish the goods and services your business is proud to offer.</p></div><Link className="button button-outline" to="/orders?as=seller">View seller orders</Link></div>
    <div className="dashboard-grid">
      <form className="form-card listing-form" onSubmit={(event) => { void createListing(event); }}>
        <h2>Create a listing</h2>
        <label>Listing title<input value={draft.title} onChange={(event) => setDraft({ ...draft, title: event.target.value })} required maxLength={200} /></label>
        <label>Description<textarea rows={5} value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} maxLength={20000} /></label>
        <div className="form-row"><label>Category<input value={draft.category} onChange={(event) => setDraft({ ...draft, category: event.target.value })} required maxLength={80} /></label><label>Unit<input value={draft.unit} onChange={(event) => setDraft({ ...draft, unit: event.target.value })} required maxLength={60} /></label></div>
        <div className="form-row"><label>Price<input type="number" min="0.01" step={draft.currency === 'USDC' ? '0.000001' : '0.01'} value={draft.price} onChange={(event) => setDraft({ ...draft, price: event.target.value })} required /><small>Enter major units. Stored as kobo, cents, or micro-USDC.</small></label><label>Currency<select value={draft.currency} onChange={(event) => { const value = event.target.value; if (value === 'USDC' || value === 'NGN' || value === 'USD') setDraft({ ...draft, currency: value }); }}><option value="USDC">USDC · escrow eligible</option><option value="NGN">NGN · agreement only</option><option value="USD">USD · agreement only</option></select></label></div>
        <div className="form-row"><label>Available quantity<input type="number" min={0} step={1} value={draft.inventory_quantity} onChange={(event) => setDraft({ ...draft, inventory_quantity: Number(event.target.value) })} required /></label><label>Location<input value={draft.location} onChange={(event) => setDraft({ ...draft, location: event.target.value })} maxLength={160} placeholder="City, country" /></label></div>
        <label>Image URL<input type="url" value={draft.image_url} onChange={(event) => setDraft({ ...draft, image_url: event.target.value })} placeholder="https://…" /></label>
        {error !== null && <p className="form-error" role="alert">{error}</p>}{notice !== null && <p className="success-notice" role="status">{notice}</p>}
        <button className="button button-dark" type="submit" disabled={busy}>{busy ? 'Publishing…' : 'Publish listing'}</button>
      </form>
      <div className="seller-listings">
        <h2>Your listings <span>{listings.length}</span></h2>
        {loading ? <p className="loading-state">Loading your listings…</p> : listings.length === 0 ? <div className="empty-state"><p>Your storefront is ready when you are.</p><span>Listings you publish will appear here.</span></div> : listings.map((listing) => <article className="seller-listing-row" key={listing.id}>
          {listing.image_url ? <img src={listing.image_url} alt="" /> : <div className="seller-listing-placeholder">✳</div>}
          <div className="seller-listing-copy"><Link to={`/listings/${listing.id}`}><strong>{listing.title}</strong></Link><span>{formatListingPrice(listing.price, listing.currency)} / {listing.unit} · {listing.inventory_quantity} available</span><small className={`status-pill status-${listing.status}`}>{listing.status.replace('_', ' ')}</small></div>
          <button className="button button-quiet" type="button" disabled={busy} onClick={() => { void updateStatus(listing); }}>{listing.status === 'active' ? 'Pause' : 'Activate'}</button>
        </article>)}
        <button className="text-link" type="button" onClick={() => { void refresh(); }}>Refresh listings</button>
      </div>
    </div>
  </section>;
}
