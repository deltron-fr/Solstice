import { useEffect, useState } from 'react';
import { api, type ListingResponse } from '../../lib/api/client';
import type { Listing } from '../../types/marketplace';
import { mockListings } from './mockListings';

const sellerNameCache = new Map<number, Promise<string>>();
const fallbackImage = 'https://images.unsplash.com/photo-1542838132-92c53300491e?auto=format&fit=crop&w=900&q=85';

function sellerName(id: number): Promise<string> {
  const cached = sellerNameCache.get(id);
  if (cached !== undefined) return cached;
  const lookup = api.getPublicUser(id).then((seller) => seller.name).catch(() => `Seller #${id}`);
  sellerNameCache.set(id, lookup);
  return lookup;
}

function presentListing(listing: ListingResponse, name: string): Listing {
  return {
    id: String(listing.id),
    sellerId: listing.seller_id,
    title: listing.title,
    description: listing.description,
    category: listing.category,
    sellerName: name,
    location: listing.location,
    price: listing.price,
    currency: listing.currency,
    unit: listing.unit,
    inventory: listing.inventory_quantity,
    status: listing.status === 'active' || listing.status === 'paused' || listing.status === 'sold_out' ? listing.status : 'paused',
    imageUrl: listing.image_url || fallbackImage,
  };
}

export interface ListingsState {
  listings: Listing[];
  loading: boolean;
  usingMockData: boolean;
  error: string | null;
}

export function useMarketplaceListings(limit = 100): ListingsState {
  const [state, setState] = useState<ListingsState>({ listings: [], loading: true, usingMockData: false, error: null });

  useEffect(() => {
    let active = true;
    api.listListings({ limit })
      .then((result) => {
        return Promise.all(result.listings.map(async (listing) => presentListing(listing, await sellerName(listing.seller_id))));
      })
      .then((listings) => {
        if (active) setState({ listings, loading: false, usingMockData: false, error: null });
      })
      .catch((reason: unknown) => {
        if (active) setState({
          listings: mockListings.slice(0, limit),
          loading: false,
          usingMockData: true,
          error: reason instanceof Error ? reason.message : 'Marketplace service is unavailable',
        });
      });
    return () => { active = false; };
  }, [limit]);

  return state;
}
