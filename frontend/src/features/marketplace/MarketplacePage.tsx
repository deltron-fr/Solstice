import { Link } from 'react-router-dom';
import { useState } from 'react';
import { ListingCard } from '../listings/ListingCard';
import { useMarketplaceListings } from './useListings';

export function MarketplacePage() {
  const { listings, loading, usingMockData } = useMarketplaceListings(6);
  const [category, setCategory] = useState('All listings');
  const visibleListings = category === 'All listings' ? listings : listings.filter((listing) => listing.category === category);
  const heroImage = listings[0]?.imageUrl || 'https://images.unsplash.com/photo-1542838132-92c53300491e?auto=format&fit=crop&w=900&q=85';

  return (
    <>
      <section className="hero">
        <div className="hero-copy">
          <p className="eyebrow"><span className="eyebrow-dot" /> A better way to do business</p>
          <h1>Good deals.<br /><em>Good people.</em></h1>
          <p className="hero-description">Discover quality goods and services from businesses across your community. Every deal starts with a conversation.</p>
          <div className="hero-actions"><a className="button button-dark" href="#listings">Explore the marketplace <span aria-hidden="true">↗</span></a><Link className="button button-outline" to="/seller">I’m a seller</Link></div>
          <div className="hero-proof"><span className="avatar-stack"><i>G</i><i>M</i><i>D</i></span><span>Built for the people behind the business</span></div>
        </div>
        <div className="hero-art" role="img" aria-label="Fresh produce ready for local business"><img src={heroImage} alt="Fresh produce at a local market" /><div className="floating-note"><span className="note-icon">✳</span><span><strong>Trade with confidence</strong><small>Payments secured by escrow</small></span></div><span className="art-caption">MADE FOR LOCAL BUSINESS <span>✳</span></span></div>
      </section>
      <section className="trust-strip" id="how-it-works"><span>From independent sellers</span><span className="strip-divider" /><span>Built around fair agreements</span><span className="strip-divider" /><span>Payments secured on-chain</span></section>
      <section className="listing-section" id="listings">
        <div className="section-heading"><div><p className="eyebrow">THE MARKETPLACE</p><h2>Find your next good thing.</h2></div><Link className="text-link" to="/listings">View all listings <span>→</span></Link></div>
        <div className="category-row">{['All listings', 'Food & produce', 'Home & craft', 'Business services'].map((value) => <button className={`category-chip${category === value ? ' selected' : ''}`} key={value} type="button" aria-pressed={category === value} onClick={() => setCategory(value)}>{value}</button>)}</div>
        {usingMockData && <p className="service-notice">Showing sample listings while the marketplace API is unavailable.</p>}
        {loading ? <p className="loading-state">Loading marketplace…</p> : visibleListings.length > 0 ? <div className="listing-grid">{visibleListings.map((listing) => <ListingCard key={listing.id} listing={listing} />)}</div> : <p className="empty-state">No listings in this category yet. Try another filter.</p>}
      </section>
      <section className="seller-cta"><div><p className="eyebrow">YOUR NEXT CUSTOMER IS HERE</p><h2>What are you building?</h2></div><Link to="/seller" className="button button-light">Start selling <span>↗</span></Link></section>
    </>
  );
}
