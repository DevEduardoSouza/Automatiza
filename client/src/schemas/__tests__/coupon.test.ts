import { describe, expect, it } from 'vitest';
import { couponSchema } from '../coupon';

const t = (key: string) => key;
const schema = couponSchema(t);

describe('couponSchema', () => {
  const validComplete = {
    code: 'PROMO10',
    title: 'Desconto Shopee',
    store: 'shopee' as const,
    affiliate_link: 'https://shopee.com/aff/123',
    expires_at: '2026-12-31T23:59:59Z',
    category: 'electronics',
    discount_type: 'percent' as const,
    discount_value: 10,
    max_discount: 50,
    min_product_price: 100,
    coupon_text: 'Use este cupom para ganhar 10% de desconto',
  };

  const validMinimal = {
    code: 'MIN',
    title: 'Minimal Coupon',
    store: 'amazon' as const,
  };

  // ── Valid data ────────────────────────────────────────────

  it('should pass with complete valid data', () => {
    const result = schema.safeParse(validComplete);
    expect(result.success).toBe(true);
  });

  it('should pass with minimal valid data (code, title, store)', () => {
    const result = schema.safeParse(validMinimal);
    expect(result.success).toBe(true);
  });

  // ── code ──────────────────────────────────────────────────

  it('should fail when code is empty', () => {
    const result = schema.safeParse({ ...validMinimal, code: '' });
    expect(result.success).toBe(false);
  });

  it('should fail when code exceeds 50 characters', () => {
    const result = schema.safeParse({ ...validMinimal, code: 'A'.repeat(51) });
    expect(result.success).toBe(false);
  });

  it('should trim whitespace from code', () => {
    const result = schema.safeParse({ ...validMinimal, code: '  PROMO  ' });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.code).toBe('PROMO');
    }
  });

  // ── title ─────────────────────────────────────────────────

  it('should fail when title is empty', () => {
    const result = schema.safeParse({ ...validMinimal, title: '' });
    expect(result.success).toBe(false);
  });

  it('should fail when title exceeds 255 characters', () => {
    const result = schema.safeParse({ ...validMinimal, title: 'T'.repeat(256) });
    expect(result.success).toBe(false);
  });

  // ── store ─────────────────────────────────────────────────

  it('should accept all 7 valid store values', () => {
    const stores = ['shopee', 'amazon', 'magalu', 'ml', 'shein', 'aliexpress', 'awin'] as const;
    for (const store of stores) {
      const result = schema.safeParse({ ...validMinimal, store });
      expect(result.success).toBe(true);
    }
  });

  it('should fail with invalid store value', () => {
    const result = schema.safeParse({ ...validMinimal, store: 'invalid_store' });
    expect(result.success).toBe(false);
  });

  // ── affiliate_link ────────────────────────────────────────

  it('should accept a valid URL for affiliate_link', () => {
    const result = schema.safeParse({ ...validMinimal, affiliate_link: 'https://example.com' });
    expect(result.success).toBe(true);
  });

  it('should fail with an invalid URL for affiliate_link', () => {
    const result = schema.safeParse({ ...validMinimal, affiliate_link: 'not-a-url' });
    expect(result.success).toBe(false);
  });

  it('should accept empty string for affiliate_link', () => {
    const result = schema.safeParse({ ...validMinimal, affiliate_link: '' });
    expect(result.success).toBe(true);
  });

  it('should accept null or undefined for affiliate_link', () => {
    const resultNull = schema.safeParse({ ...validMinimal, affiliate_link: null });
    const resultUndefined = schema.safeParse({ ...validMinimal, affiliate_link: undefined });
    expect(resultNull.success).toBe(true);
    expect(resultUndefined.success).toBe(true);
  });

  // ── discount_type ─────────────────────────────────────────

  it('should accept fixed and percent as discount_type', () => {
    const resultFixed = schema.safeParse({ ...validMinimal, discount_type: 'fixed' });
    const resultPercent = schema.safeParse({ ...validMinimal, discount_type: 'percent' });
    expect(resultFixed.success).toBe(true);
    expect(resultPercent.success).toBe(true);
  });

  // ── discount_value ────────────────────────────────────────

  it('should fail when discount_value is zero', () => {
    const result = schema.safeParse({ ...validMinimal, discount_value: 0 });
    expect(result.success).toBe(false);
  });

  it('should fail when discount_value is negative', () => {
    const result = schema.safeParse({ ...validMinimal, discount_value: -5 });
    expect(result.success).toBe(false);
  });

  // ── coupon_text ───────────────────────────────────────────

  it('should fail when coupon_text exceeds 500 characters', () => {
    const result = schema.safeParse({ ...validMinimal, coupon_text: 'X'.repeat(501) });
    expect(result.success).toBe(false);
  });
});
