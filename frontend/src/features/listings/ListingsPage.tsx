import { Link } from 'react-router-dom';
import { mockListings } from '../marketplace/mockListings';
import { ListingCard } from './ListingCard';

export function ListingsPage() {
  return <section className="page-section"><div className="section-heading"><div><p className="eyebrow">THE MARKETPLACE</p><h1>Explore listings</h1><p className="page-intro">Find the people and products that make your business better.</p></div><Link className="button button-dark" to="/seller">Create a listing</Link></div><div className="category-row"><button className="category-chip selected">All listings</button><button className="category-chip">Products</button><button className="category-chip">Services</button></div><div className="listing-grid">{mockListings.map((listing) => <ListingCard key={listing.id} listing={listing} />)}</div></section>;
}
