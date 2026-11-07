import { useEffect, useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../auth/AuthProvider';
import { api } from '../../lib/api/client';
import type { Listing } from '../../types/marketplace';
import { mockListings } from '../marketplace/mockListings';
import { formatListingPrice } from './formatPrice';

export function ListingDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { user } = useAuth();
  const [listing, setListing] = useState<Listing | null>(null);
  const [loading, setLoading] = useState(true);
  const [sample, setSample] = useState(false);
  const [quantity, setQuantity] = useState(1);
  const [message, setMessage] = useState('Hi, I’m interested in this listing. Is it still available?');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    const numericId = Number(id);
    if (!Number.isSafeInteger(numericId) || numericId < 1) {
      setListing(mockListings.find((item) => item.id === id) ?? null);
      setSample(true);
      setLoading(false);
      return;
    }
    api.getListing(numericId)
      .then(async (result) => {
        let sellerName = `Seller #${result.seller_id}`;
        try { sellerName = (await api.getPublicUser(result.seller_id)).name; } catch { /* Show an ID-based fallback if public profile lookup is unavailable. */ }
        if (active) setListing({
          id: String(result.id), sellerId: result.seller_id, title: result.title, description: result.description,
          category: result.category, sellerName, location: result.location, price: result.price,
          currency: result.currency, unit: result.unit, inventory: result.inventory_quantity,
          status: result.status === 'active' || result.status === 'paused' || result.status === 'sold_out' ? result.status : 'paused',
          imageUrl: result.image_url || 'https://images.unsplash.com/photo-1542838132-92c53300491e?auto=format&fit=crop&w=900&q=85',
        });
      })
      .catch(() => {
        if (active) {
          setListing(null);
          setError('This listing could not be loaded from the marketplace.');
        }
      })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [id]);

  async function handleContact(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!user || !listing?.sellerId) return;
    setBusy(true); setError(null);
    try {
      const conversation = await api.createConversation(listing.sellerId);
      await api.sendMessage(conversation.id, message);
      navigate(`/messages?conversation=${conversation.id}`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not send your message.');
    } finally { setBusy(false); }
  }

  async function handleOrder() {
    if (!user || !listing || !listing.sellerId) return;
    setBusy(true); setError(null);
    try {
      const order = await api.createOrder(Number(listing.id), quantity);
      navigate(`/orders?order=${order.id}`);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not create this order.');
    } finally { setBusy(false); }
  }

  if (loading) return <section className="page-section"><p className="loading-state">Loading listing…</p></section>;
  if (!listing) return <section className="page-section"><p>{error ?? 'We couldn’t find that listing.'}</p><Link to="/">Back to marketplace</Link></section>;

  const ownListing = listing.sellerId !== undefined && user?.id === listing.sellerId;
  return <section className="detail-layout page-section">
    <img className="detail-image" src={listing.imageUrl} alt={listing.title} />
    <div className="detail-copy">
      {sample && <p className="service-notice">Sample listing · orders and messages are not connected for preview data.</p>}
      <p className="eyebrow">{listing.category}</p><h1>{listing.title}</h1>
      <p className="detail-seller">Sold by <strong>{listing.sellerName}</strong> · {listing.location || 'Location not listed'}</p>
      <p>{listing.description}</p>
      <p className="detail-price">{formatListingPrice(listing.price, listing.currency)} <small>/ {listing.unit}</small></p>
      <p className="stock-label">{listing.inventory} available</p>
      {!sample && user && !ownListing && listing.sellerId !== undefined && <>
        <div className="purchase-box">
          <label>Quantity<input type="number" min={1} max={listing.inventory} value={quantity} onChange={(event) => setQuantity(Math.max(1, Number(event.target.value)))} /></label>
          <button className="button button-dark" type="button" disabled={busy || listing.currency !== 'USDC' || listing.inventory < 1} onClick={() => { void handleOrder(); }}>{busy ? 'Creating order…' : listing.currency === 'USDC' ? 'Create order' : 'USDC checkout only'}</button>
          {listing.currency !== 'USDC' && <small>Escrow checkout is currently available for USDC-priced listings only.</small>}
        </div>
        <form className="contact-box" onSubmit={(event) => { void handleContact(event); }}>
          <label>Contact the seller<textarea rows={3} value={message} onChange={(event) => setMessage(event.target.value)} maxLength={4000} required /></label>
          <button className="button button-outline" type="submit" disabled={busy}>{busy ? 'Sending…' : 'Send message'}</button>
        </form>
      </>}
      {!user && <p className="detail-note">Connect and sign in with a wallet to contact the seller or start an order.</p>}
      {ownListing && <p className="detail-note">This is your listing. Manage it from the seller dashboard.</p>}
      {error !== null && <p className="form-error" role="alert">{error}</p>}
      <p className="detail-note">Marketplace details stay off-chain; Pact’s escrow contract secures eligible USDC orders.</p>
    </div>
  </section>;
}
