import { createBrowserRouter } from 'react-router-dom';
import { AppLayout } from '../components/layout/AppLayout';
import { ListingDetailPage } from '../features/listings/ListingDetailPage';
import { ListingsPage } from '../features/listings/ListingsPage';
import { InboxPage } from '../features/messaging/InboxPage';
import { MarketplacePage } from '../features/marketplace/MarketplacePage';
import { OrdersPage } from '../features/orders/OrdersPage';
import { SellerPage } from '../features/seller/SellerPage';

export const router = createBrowserRouter([
  { path: '/', element: <AppLayout />, children: [
    { index: true, element: <MarketplacePage /> },
    { path: 'listings', element: <ListingsPage /> },
    { path: 'listings/:id', element: <ListingDetailPage /> },
    { path: 'messages', element: <InboxPage /> },
    { path: 'seller', element: <SellerPage /> },
    { path: 'orders', element: <OrdersPage /> },
  ] },
]);
