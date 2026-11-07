import { describe, expect, it } from 'vitest';
import { formatListingPrice } from './formatPrice';

describe('formatListingPrice', () => {
  it('converts stored kobo to naira', () => {
    expect(formatListingPrice(1_850_000, 'NGN')).toBe('₦18,500');
  });

  it('converts stored cents to dollars', () => {
    expect(formatListingPrice(6_800, 'USD')).toBe('$68');
  });

  it('converts stored micro-USDC to USDC', () => {
    expect(formatListingPrice(1_234_567, 'USDC')).toBe('1.234567 USDC');
  });
});
