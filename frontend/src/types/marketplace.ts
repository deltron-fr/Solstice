import type { ListingCurrency, ListingStatus } from '../lib/api/client';

export interface Listing {
  id: string;
  sellerId?: number;
  title: string;
  description: string;
  category: string;
  sellerName: string;
  location: string;
  price: number;
  currency: ListingCurrency;
  unit: string;
  inventory: number;
  status: ListingStatus;
  imageUrl: string;
}

export type OrderStatus =
  | 'pending_payment'
  | 'paid'
  | 'fulfilling'
  | 'completed'
  | 'cancelled'
  | 'disputed';

export interface ApiError {
  error: string;
  message: string;
}
