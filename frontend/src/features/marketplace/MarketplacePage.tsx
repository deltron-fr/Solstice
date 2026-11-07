import { Link } from 'react-router-dom';
import { mockListings } from './mockListings';
import { ListingCard } from '../listings/ListingCard';

export function MarketplacePage() {
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
        <div className="hero-art" role="img" aria-label="Fresh produce ready for local business"><img src={mockListings[0]?.imageUrl} alt="Fresh produce at a local market" /><div className="floating-note"><span className="note-icon">✳</span><span><strong>Trade with confidence</strong><small>Payments secured by escrow</small></span></div><span className="art-caption">MADE FOR LOCAL BUSINESS <span>✳</span></span></div>
      </section>
      <section className="trust-strip" id="how-it-works"><span>From independent sellers</span><span className="strip-divider" /><span>Built around fair agreements</span><span className="strip-divider" /><span>Payments secured on-chain</span></section>
      <section className="listing-section" id="listings">
        <div className="section-heading"><div><p className="eyebrow">THE MARKETPLACE</p><h2>Find your next good thing.</h2></div><Link className="text-link" to="/listings">View all listings <span>→</span></Link></div>
        <div className="category-row"><button className="category-chip selected">All listings</button><button className="category-chip">Food & produce</button><button className="category-chip">Home & craft</button><button className="category-chip">Business services</button></div>
        <div className="listing-grid">{mockListings.map((listing) => <ListingCard key={listing.id} listing={listing} />)}</div>
      </section>
      <section className="seller-cta"><div><p className="eyebrow">YOUR NEXT CUSTOMER IS HERE</p><h2>What are you building?</h2></div><Link to="/seller" className="button button-light">Start selling <span>↗</span></Link></section>
    </>
  );
}
