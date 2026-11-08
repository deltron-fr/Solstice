import { Link } from 'react-router-dom';
import type { Listing } from '../../types/marketplace';
import { formatListingPrice } from './formatPrice';

interface ListingCardProps { listing: Listing }

export function ListingCard({ listing }: ListingCardProps) {
  return (
    <article className="listing-card">
      <Link to={`/listings/${listing.id}`} className="listing-image-link" aria-label={`View ${listing.title}`}><img src={listing.imageUrl} alt="" /><span className="listing-category">{listing.category}</span></Link>
      <div className="listing-info"><div className="seller-line"><span className="seller-avatar">{listing.sellerName.charAt(0)}</span><span>{listing.sellerName}</span></div><Link className="listing-title" to={`/listings/${listing.id}`}>{listing.title}</Link><p className="listing-location">⌖ &nbsp;{listing.location || 'Location not listed'}</p><div className="listing-card-bottom"><p><strong>{formatListingPrice(listing.price, listing.currency)}</strong><span> / {listing.unit}</span></p><span className="stock-label">{listing.inventory} available</span></div></div>
    </article>
  );
}
