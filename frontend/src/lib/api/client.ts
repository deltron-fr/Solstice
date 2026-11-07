export interface ListingResponse {
  id: number;
  seller_id: number;
  title: string;
  description: string;
  price: number;
  currency: 'NGN' | 'USD' | 'USDC';
  inventory_quantity: number;
  status: 'draft' | 'active' | 'paused' | 'sold_out' | 'archived';
  created_at: string;
  updated_at: string;
}

interface ListingListResponse {
  listings: ListingResponse[];
  limit: number;
  offset: number;
}

interface ErrorResponse {
  error: string;
  message: string;
}

function isErrorResponse(value: unknown): value is ErrorResponse {
  if (typeof value !== 'object' || value === null) return false;
  const record = value as Record<string, unknown>;
  return typeof record.error === 'string' && typeof record.message === 'string';
}

function isListingResponse(value: unknown): value is ListingResponse {
  if (typeof value !== 'object' || value === null) return false;
  const listing = value as Record<string, unknown>;
  return typeof listing.id === 'number'
    && typeof listing.seller_id === 'number'
    && typeof listing.title === 'string'
    && typeof listing.description === 'string'
    && typeof listing.price === 'number'
    && (listing.currency === 'NGN' || listing.currency === 'USD' || listing.currency === 'USDC')
    && typeof listing.inventory_quantity === 'number'
    && (listing.status === 'draft' || listing.status === 'active' || listing.status === 'paused' || listing.status === 'sold_out' || listing.status === 'archived')
    && typeof listing.created_at === 'string'
    && typeof listing.updated_at === 'string';
}

function isListingListResponse(value: unknown): value is ListingListResponse {
  if (typeof value !== 'object' || value === null) return false;
  const record = value as Record<string, unknown>;
  return Array.isArray(record.listings)
    && record.listings.every(isListingResponse)
    && typeof record.limit === 'number'
    && typeof record.offset === 'number';
}

export class ApiRequestError extends Error {
  constructor(readonly status: number, readonly code: string, message: string) {
    super(message);
    this.name = 'ApiRequestError';
  }
}

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? '/api';

export async function listListings(options: { limit?: number; offset?: number } = {}): Promise<ListingListResponse> {
  const query = new URLSearchParams();
  if (options.limit !== undefined) query.set('limit', String(options.limit));
  if (options.offset !== undefined) query.set('offset', String(options.offset));
  const suffix = query.size > 0 ? `?${query.toString()}` : '';
  const response = await fetch(`${apiBaseUrl}/listings${suffix}`);
  if (!response.ok) {
    const payload: unknown = await response.json();
    const error = isErrorResponse(payload) ? payload : { error: 'request_failed', message: 'The request could not be completed' };
    throw new ApiRequestError(response.status, error.error, error.message);
  }
  const payload: unknown = await response.json();
  if (!isListingListResponse(payload)) throw new Error('The listings API returned an invalid response');
  return payload;
}
