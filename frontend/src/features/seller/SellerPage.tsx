import { Link } from 'react-router-dom';

export function SellerPage() {
  return <section className="seller-placeholder page-section"><span className="large-symbol">✳</span><p className="eyebrow">SELL ON PACT</p><h1>Your business belongs here.</h1><p>Build a storefront, meet new customers, and agree on the details that make every order work.</p><button className="button button-dark" type="button" disabled>Seller tools are coming soon</button><Link className="text-link" to="/">Explore the marketplace</Link></section>;
}
