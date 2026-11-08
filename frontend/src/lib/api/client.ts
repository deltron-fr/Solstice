export type ListingCurrency = 'NGN' | 'USD' | 'USDC';
export type ListingStatus = 'draft' | 'active' | 'paused' | 'sold_out' | 'archived';
export type OrderStatus = 'pending_payment' | 'paid' | 'fulfilling' | 'completed' | 'cancelled' | 'refunded' | 'disputed';
export type PaymentStatus = 'created' | 'submitted' | 'finalized' | 'failed' | 'released' | 'refunded';

export interface UserResponse {
  id: number;
  name: string;
  email: string;
  wallet_address: string | null;
  created_at: string;
}

export interface PublicUserResponse {
  id: number;
  name: string;
  wallet_address: string | null;
}

export interface ListingResponse {
  id: number;
  seller_id: number;
  title: string;
  description: string;
  price: number;
  currency: ListingCurrency;
  category: string;
  unit: string;
  location: string;
  image_url: string;
  inventory_quantity: number;
  status: ListingStatus;
  created_at: string;
  updated_at: string;
}

export interface ListingListResponse {
  listings: ListingResponse[];
  limit: number;
  offset: number;
}

export interface OrderResponse {
  id: number;
  buyer_id: number;
  seller_id: number;
  listing_id: number;
  quantity: number;
  unit_price: number;
  total_amount: number;
  chain_order_id: string;
  status: OrderStatus;
  created_at: string;
  updated_at: string;
}

export interface OrderListResponse {
  orders: OrderResponse[];
  role: 'buyer' | 'seller';
  limit: number;
  offset: number;
}

export interface PaymentResponse {
  payment: {
    id: number;
    order_id: number;
    escrow_pda_address: string | null;
    transaction_signature: string | null;
    amount: number;
    token_mint: string;
    status: PaymentStatus;
    funding_expires_at: string;
    created_at: string;
    updated_at: string;
  };
  chain_order_id: string;
  escrow_pda_address: string | null;
  funding_expires_at: string;
  escrow_state: {
    status: 'created' | 'funded' | 'released' | 'refunded' | 'disputed';
    funding_expires_at: number;
    funded_at: number;
    expires_at: number;
    vault_amount: number;
  } | null;
}

export interface ConversationResponse {
  id: number;
  buyer_id: number;
  seller_id: number;
  order_id: number | null;
  created_at: string;
}

export interface MessageResponse {
  id: number;
  conversation_id: number;
  sender_id: number;
  content: string;
  created_at: string;
}

export interface ApiErrorBody {
  error: string;
  message: string;
}

export class ApiRequestError extends Error {
  constructor(readonly status: number, readonly code: string, message: string) {
    super(message);
    this.name = 'ApiRequestError';
  }
}

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? '/api';
const normalizedApiBaseUrl = apiBaseUrl.replace(/\/$/, '');

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined && !(init.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  const response = await fetch(`${normalizedApiBaseUrl}${path}`, { ...init, headers, credentials: 'include' });
  if (!response.ok) {
    let body: ApiErrorBody = { error: 'request_failed', message: 'The request could not be completed' };
    try {
      const payload: unknown = await response.json();
      if (typeof payload === 'object' && payload !== null) {
        const value = payload as Record<string, unknown>;
        if (typeof value.error === 'string' && typeof value.message === 'string') {
          body = { error: value.error, message: value.message };
        }
      }
    } catch {
      // An empty or non-JSON server response still becomes a consistent API error.
    }
    throw new ApiRequestError(response.status, body.error, body.message);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

function jsonBody(value: unknown): string {
  return JSON.stringify(value);
}

export const api = {
  health: () => request<{ status: string }>('/health'),
  getCurrentUser: () => request<UserResponse>('/auth/me'),
  createChallenge: (walletAddress: string) => request<{ nonce: string; message: string; expires_at: string }>('/auth/challenge', {
    method: 'POST', body: jsonBody({ wallet_address: walletAddress }),
  }),
  verifyChallenge: (input: { wallet_address: string; nonce: string; signature: string; name: string; email: string }) =>
    request<UserResponse>('/auth/verify', { method: 'POST', body: jsonBody(input) }),
  logout: () => request<void>('/auth/logout', { method: 'POST', body: '{}' }),
  listListings: (options: { limit?: number; offset?: number } = {}) => {
    const query = new URLSearchParams();
    if (options.limit !== undefined) query.set('limit', String(options.limit));
    if (options.offset !== undefined) query.set('offset', String(options.offset));
    return request<ListingListResponse>(`/listings${query.size > 0 ? `?${query}` : ''}`);
  },
  getListing: (id: number) => request<ListingResponse>(`/listings/${id}`),
  listMyListings: () => request<ListingListResponse>('/listings/mine'),
  uploadImage: (image: File) => {
    const body = new FormData();
    body.append('image', image);
    return request<{ image_url: string }>('/uploads', { method: 'POST', body });
  },
  getPublicUser: (id: number) => request<PublicUserResponse>(`/public/users/${id}`),
  createListing: (input: Omit<ListingResponse, 'id' | 'seller_id' | 'status' | 'created_at' | 'updated_at'>) =>
    request<ListingResponse>('/listings', { method: 'POST', body: jsonBody(input) }),
  updateListing: (id: number, input: Partial<Pick<ListingResponse, 'title' | 'description' | 'price' | 'currency' | 'category' | 'unit' | 'location' | 'image_url' | 'inventory_quantity' | 'status'>>) =>
    request<ListingResponse>(`/listings/${id}`, { method: 'PATCH', body: jsonBody(input) }),
  createOrder: (listingId: number, quantity: number) => request<OrderResponse>('/orders', {
    method: 'POST', body: jsonBody({ listing_id: listingId, quantity }),
  }),
  listOrders: (role: 'buyer' | 'seller') => request<OrderListResponse>(`/orders?as=${role}`),
  getOrder: (id: number) => request<OrderResponse>(`/orders/${id}`),
  fulfillOrder: (id: number) => request<OrderResponse>(`/orders/${id}/fulfill`, { method: 'POST', body: '{}' }),
  cancelOrder: (id: number) => request<OrderResponse>(`/orders/${id}/cancel`, { method: 'POST', body: '{}' }),
  getPayment: (id: number) => request<PaymentResponse>(`/orders/${id}/payment`),
  submitPaymentTransaction: (id: number, type: 'fund' | 'release' | 'refund', signature: string) =>
    request<{ signature: string; status: 'submitted' | 'finalized' | 'failed'; order_id: number }>(`/orders/${id}/payment/transactions`, {
      method: 'POST', body: jsonBody({ type, signature }),
    }),
  createConversation: (sellerId: number, orderId?: number) => request<ConversationResponse>('/conversations', {
    method: 'POST', body: jsonBody({ seller_id: sellerId, ...(orderId === undefined ? {} : { order_id: orderId }) }),
  }),
  listConversations: () => request<{ conversations: ConversationResponse[]; limit: number; offset: number }>('/conversations'),
  listMessages: (conversationId: number) => request<{ messages: MessageResponse[]; before_id: number; limit: number }>(`/conversations/${conversationId}/messages`),
  sendMessage: (conversationId: number, content: string) => request<MessageResponse>(`/conversations/${conversationId}/messages`, {
    method: 'POST', body: jsonBody({ content }),
  }),
};

export function imageSource(imagePath: string): string {
  if (imagePath.startsWith('/uploads/')) return `${normalizedApiBaseUrl}${imagePath}`;
  return imagePath;
}
