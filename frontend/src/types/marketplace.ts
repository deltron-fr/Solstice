export type ListingStatus = 'active' | 'paused' | 'sold_out';

export interface Listing {
  id: string;
  title: string;
  description: string;
  category: string;
  sellerName: string;
  location: string;
  price: number;
  currency: 'NGN';
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
