import type { Listing } from '../../types/marketplace';

export function formatListingPrice(price: number, currency: Listing['currency']): string {
  const scale = currency === 'USDC' ? 1_000_000 : 100;
  const majorUnits = price / scale;
  if (currency === 'NGN') return `₦${majorUnits.toLocaleString('en-NG', { maximumFractionDigits: 2 })}`;
  if (currency === 'USD') return `$${majorUnits.toLocaleString('en-US', { maximumFractionDigits: 2 })}`;
  return `${majorUnits.toLocaleString('en-US', { maximumFractionDigits: 6 })} USDC`;
}
