import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiRequestError } from './client';

afterEach(() => vi.unstubAllGlobals());

describe('Pact API client', () => {
  it('sends order creation with same-origin credentials', async () => {
    const order = {
      id: 1, buyer_id: 2, seller_id: 3, listing_id: 4, quantity: 1,
      unit_price: 125_000, total_amount: 125_000, chain_order_id: 'a'.repeat(64),
      status: 'pending_payment', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
    };
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(order), { status: 201 }));
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.createOrder(4, 1)).resolves.toEqual(order);
    expect(fetchMock).toHaveBeenCalledWith('/api/orders', expect.objectContaining({
      method: 'POST', credentials: 'include', body: JSON.stringify({ listing_id: 4, quantity: 1 }),
    }));
  });

  it('normalizes backend errors without losing the error code', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(
      JSON.stringify({ error: 'insufficient_inventory', message: 'Not enough stock' }),
      { status: 409, headers: { 'Content-Type': 'application/json' } },
    )));

    const result = api.createOrder(4, 8);
    await expect(result).rejects.toBeInstanceOf(ApiRequestError);
    await expect(result).rejects.toMatchObject({
      status: 409, code: 'insufficient_inventory', message: 'Not enough stock',
    });
  });
});
