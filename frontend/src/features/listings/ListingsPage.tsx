import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ListingCard } from './ListingCard';
import { useMarketplaceListings } from '../marketplace/useListings';

export function ListingsPage() {
  const { listings, loading, usingMockData } = useMarketplaceListings();
  const [category, setCategory] = useState('All listings');
  const categories = ['All listings', ...new Set(listings.map((listing) => listing.category))];
  const filtered = category === 'All listings' ? listings : listings.filter((listing) => listing.category === category);

  return <section className="page-section">
    <div className="section-heading"><div><p className="eyebrow">THE MARKETPLACE</p><h1>Explore listings</h1><p className="page-intro">Find the people and products that make your business better.</p></div><Link className="button button-dark" to="/seller">Create a listing</Link></div>
    {usingMockData && <p className="service-notice">The API could not be reached. These sample listings are for preview only.</p>}
    <div className="category-row">{categories.map((value) => <button className={`category-chip${category === value ? ' selected' : ''}`} key={value} type="button" aria-pressed={category === value} onClick={() => setCategory(value)}>{value}</button>)}</div>
    {loading ? <p className="loading-state">Loading listings…</p> : filtered.length > 0 ? <div className="listing-grid">{filtered.map((listing) => <ListingCard key={listing.id} listing={listing} />)}</div> : <div className="empty-state"><p>No listings found for this category.</p><button className="text-link" type="button" onClick={() => setCategory('All listings')}>Show all listings</button></div>}
  </section>;
}
