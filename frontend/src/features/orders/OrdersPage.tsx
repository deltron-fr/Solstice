import { Link } from 'react-router-dom';

export function OrdersPage() {
  return <section className="seller-placeholder page-section"><span className="large-symbol">↗</span><p className="eyebrow">YOUR ORDERS</p><h1>Your business, in good hands.</h1><p>When you place an order, you’ll be able to follow each step here. We’re building that flow now.</p><Link className="button button-dark" to="/">Browse marketplace</Link></section>;
}
