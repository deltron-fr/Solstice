import { Link, useParams } from 'react-router-dom';
import { mockListings } from '../marketplace/mockListings';

export function ListingDetailPage() {
  const { id } = useParams();
  const listing = mockListings.find((item) => item.id === id);
  if (!listing) return <section className="page-section"><p>We couldn’t find that listing.</p><Link to="/">Back to marketplace</Link></section>;
  return <section className="detail-layout page-section"><img className="detail-image" src={listing.imageUrl} alt={listing.title} /><div className="detail-copy"><p className="eyebrow">{listing.category}</p><h1>{listing.title}</h1><p className="detail-seller">Sold by <strong>{listing.sellerName}</strong> · {listing.location}</p><p>{listing.description}</p><p className="detail-price">₦{listing.price.toLocaleString()} <small>/ {listing.unit}</small></p><p className="stock-label">{listing.inventory} available</p><button className="button button-dark" type="button" disabled>Messaging is coming soon</button><p className="detail-note">Agree on the details first. Pact’s secure payment flow is coming soon.</p></div></section>;
}
