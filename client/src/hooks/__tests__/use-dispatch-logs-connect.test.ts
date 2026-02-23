import { describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules (BEFORE imports)
// ============================================================

vi.mock('@/gen/automations/v1/automations-AutomationsService_connectquery', () => ({
  listDispatchLogs: {},
}));

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: vi.fn(),
}));

// ============================================================
// Import after mocks
// ============================================================

import {
  fromProtoDispatchLog,
  fromProtoStatus,
  fromProtoTimestamp,
} from '../use-dispatch-logs-connect';

// ============================================================
// Tests: fromProtoTimestamp
// ============================================================

describe('fromProtoTimestamp', () => {
  it('should return current date when timestamp is undefined', () => {
    const before = Date.now();
    const result = fromProtoTimestamp(undefined);
    const after = Date.now();

    expect(result).toBeInstanceOf(Date);
    expect(result.getTime()).toBeGreaterThanOrEqual(before);
    expect(result.getTime()).toBeLessThanOrEqual(after);
  });

  it('should convert valid seconds and nanos to Date', () => {
    // 2026-01-15T12:30:00.500Z
    const timestamp = { seconds: 1768566600n, nanos: 500_000_000 };
    const result = fromProtoTimestamp(timestamp);

    expect(result).toBeInstanceOf(Date);
    expect(result.getTime()).toBe(1768566600 * 1000 + 500);
  });

  it('should handle epoch (0 seconds, 0 nanos)', () => {
    const timestamp = { seconds: 0n, nanos: 0 };
    const result = fromProtoTimestamp(timestamp);

    expect(result).toBeInstanceOf(Date);
    expect(result.getTime()).toBe(0);
    expect(result.toISOString()).toBe('1970-01-01T00:00:00.000Z');
  });

  it('should handle large seconds values', () => {
    // 2100-01-01T00:00:00Z = 4102444800
    const timestamp = { seconds: 4102444800n, nanos: 0 };
    const result = fromProtoTimestamp(timestamp);

    expect(result).toBeInstanceOf(Date);
    expect(result.getTime()).toBe(4102444800 * 1000);
    expect(result.getUTCFullYear()).toBe(2100);
  });

  it('should correctly convert nanos to milliseconds', () => {
    // 123_456_789 nanos = 123 ms (floored)
    const timestamp = { seconds: 0n, nanos: 123_456_789 };
    const result = fromProtoTimestamp(timestamp);

    expect(result.getTime()).toBe(123);
  });

  it('should handle nanos less than 1 millisecond', () => {
    // 999_999 nanos < 1ms, should floor to 0ms
    const timestamp = { seconds: 0n, nanos: 999_999 };
    const result = fromProtoTimestamp(timestamp);

    expect(result.getTime()).toBe(0);
  });

  it('should combine seconds and nanos correctly', () => {
    // 10 seconds + 250ms
    const timestamp = { seconds: 10n, nanos: 250_000_000 };
    const result = fromProtoTimestamp(timestamp);

    expect(result.getTime()).toBe(10_250);
  });

  it('should handle zero nanos with non-zero seconds', () => {
    const timestamp = { seconds: 1700000000n, nanos: 0 };
    const result = fromProtoTimestamp(timestamp);

    expect(result.getTime()).toBe(1700000000 * 1000);
  });
});

// ============================================================
// Tests: fromProtoStatus
// ============================================================

describe('fromProtoStatus', () => {
  it('should map "sent" to "sent"', () => {
    expect(fromProtoStatus('sent')).toBe('sent');
  });

  it('should map "reconciled" to "sent"', () => {
    expect(fromProtoStatus('reconciled')).toBe('sent');
  });

  it('should map "failed" to "error"', () => {
    expect(fromProtoStatus('failed')).toBe('error');
  });

  it('should map "error" to "error"', () => {
    expect(fromProtoStatus('error')).toBe('error');
  });

  it('should map "skipped" to "skipped"', () => {
    expect(fromProtoStatus('skipped')).toBe('skipped');
  });

  it('should map "blocked" to "skipped"', () => {
    expect(fromProtoStatus('blocked')).toBe('skipped');
  });

  it('should map "queued" to "queued"', () => {
    expect(fromProtoStatus('queued')).toBe('queued');
  });

  it('should map "pending" to "queued"', () => {
    expect(fromProtoStatus('pending')).toBe('queued');
  });

  it('should return "error" for unknown status', () => {
    expect(fromProtoStatus('unknown_status')).toBe('error');
  });

  it('should handle case insensitive input for "SENT"', () => {
    expect(fromProtoStatus('SENT')).toBe('sent');
  });

  it('should handle case insensitive input for "Reconciled"', () => {
    expect(fromProtoStatus('Reconciled')).toBe('sent');
  });

  it('should handle case insensitive input for "FAILED"', () => {
    expect(fromProtoStatus('FAILED')).toBe('error');
  });

  it('should handle case insensitive input for "SKIPPED"', () => {
    expect(fromProtoStatus('SKIPPED')).toBe('skipped');
  });

  it('should handle case insensitive input for "QUEUED"', () => {
    expect(fromProtoStatus('QUEUED')).toBe('queued');
  });

  it('should handle mixed case input like "pEnDiNg"', () => {
    expect(fromProtoStatus('pEnDiNg')).toBe('queued');
  });

  it('should return "error" for empty string', () => {
    expect(fromProtoStatus('')).toBe('error');
  });
});

// ============================================================
// Tests: fromProtoDispatchLog
// ============================================================

describe('fromProtoDispatchLog', () => {
  const fullProtoLog = {
    id: 'log-001',
    automationId: 'auto-123',
    groupId: '5511999999999@g.us',
    groupName: 'Ofertas Shopee',
    canonicalKey: 'shopee:product-abc',
    offerTitle: 'Fone Bluetooth TWS',
    offerUrl: 'https://shopee.com.br/product/123',
    offerPrice: 49.9,
    status: 'sent',
    errorMessage: '',
    createdAt: { seconds: 1768566600n, nanos: 500_000_000 },
    store: 'shopee',
    productCategory: 'eletronicos',
    productPrice: 59.9,
    couponId: 'coupon-456',
    couponCode: 'DESC10',
    couponApplied: true,
    couponSkipReason: '',
    productUrl: 'https://shopee.com.br/product/123',
    productPageUrl: 'https://shopee.com.br/product/123?page=1',
    affiliateLink: 'https://s.shopee.com.br/aff/123',
  };

  it('should convert all fields from proto to local type', () => {
    const result = fromProtoDispatchLog(fullProtoLog);

    expect(result.id).toBe('log-001');
    expect(result.automationId).toBe('auto-123');
    expect(result.groupId).toBe('5511999999999@g.us');
    expect(result.groupName).toBe('Ofertas Shopee');
    expect(result.canonicalKey).toBe('shopee:product-abc');
    expect(result.offerTitle).toBe('Fone Bluetooth TWS');
    expect(result.offerUrl).toBe('https://shopee.com.br/product/123');
    expect(result.offerPrice).toBe(49.9);
    expect(result.status).toBe('sent');
    expect(result.errorMessage).toBe('');
    expect(result.createdAt).toBeInstanceOf(Date);
    expect(result.store).toBe('shopee');
    expect(result.productCategory).toBe('eletronicos');
    expect(result.productPrice).toBe(59.9);
    expect(result.couponId).toBe('coupon-456');
    expect(result.couponCode).toBe('DESC10');
    expect(result.couponApplied).toBe(true);
    expect(result.couponSkipReason).toBe('');
    expect(result.productUrl).toBe('https://shopee.com.br/product/123');
    expect(result.productPageUrl).toBe('https://shopee.com.br/product/123?page=1');
    expect(result.affiliateLink).toBe('https://s.shopee.com.br/aff/123');
  });

  it('should handle minimal proto log with optional fields undefined', () => {
    const minimalLog = {
      id: 'log-002',
      automationId: 'auto-456',
      groupId: '5511888888888@g.us',
      groupName: 'Grupo Teste',
      canonicalKey: 'test:key',
      offerTitle: 'Produto Teste',
      offerUrl: undefined,
      offerPrice: undefined,
      status: 'queued',
      errorMessage: undefined,
      createdAt: undefined,
      store: undefined,
      productCategory: undefined,
      productPrice: undefined,
      couponId: undefined,
      couponCode: undefined,
      couponApplied: undefined,
      couponSkipReason: undefined,
      productUrl: undefined,
      productPageUrl: undefined,
      affiliateLink: undefined,
    };

    const result = fromProtoDispatchLog(minimalLog);

    expect(result.id).toBe('log-002');
    expect(result.automationId).toBe('auto-456');
    expect(result.groupId).toBe('5511888888888@g.us');
    expect(result.groupName).toBe('Grupo Teste');
    expect(result.status).toBe('queued');
    expect(result.offerUrl).toBeUndefined();
    expect(result.offerPrice).toBeUndefined();
    expect(result.errorMessage).toBeUndefined();
    expect(result.createdAt).toBeInstanceOf(Date);
    expect(result.store).toBeUndefined();
    expect(result.productCategory).toBeUndefined();
    expect(result.productPrice).toBeUndefined();
    expect(result.couponId).toBeUndefined();
    expect(result.couponCode).toBeUndefined();
    expect(result.couponApplied).toBeUndefined();
    expect(result.couponSkipReason).toBeUndefined();
    expect(result.productUrl).toBeUndefined();
    expect(result.productPageUrl).toBeUndefined();
    expect(result.affiliateLink).toBeUndefined();
  });

  it('should normalize status via fromProtoStatus', () => {
    const logWithReconciled = { ...fullProtoLog, status: 'reconciled' };
    expect(fromProtoDispatchLog(logWithReconciled).status).toBe('sent');

    const logWithBlocked = { ...fullProtoLog, status: 'blocked' };
    expect(fromProtoDispatchLog(logWithBlocked).status).toBe('skipped');

    const logWithPending = { ...fullProtoLog, status: 'pending' };
    expect(fromProtoDispatchLog(logWithPending).status).toBe('queued');

    const logWithFailed = { ...fullProtoLog, status: 'failed' };
    expect(fromProtoDispatchLog(logWithFailed).status).toBe('error');
  });

  it('should convert createdAt timestamp to valid Date', () => {
    const result = fromProtoDispatchLog(fullProtoLog);

    expect(result.createdAt).toBeInstanceOf(Date);
    expect(result.createdAt.getTime()).toBe(1768566600 * 1000 + 500);
  });

  it('should return current date when createdAt is undefined', () => {
    const before = Date.now();
    const logWithoutDate = { ...fullProtoLog, createdAt: undefined };
    const result = fromProtoDispatchLog(logWithoutDate);
    const after = Date.now();

    expect(result.createdAt).toBeInstanceOf(Date);
    expect(result.createdAt.getTime()).toBeGreaterThanOrEqual(before);
    expect(result.createdAt.getTime()).toBeLessThanOrEqual(after);
  });

  it('should preserve couponApplied as boolean false', () => {
    const logWithNoCoupon = { ...fullProtoLog, couponApplied: false };
    const result = fromProtoDispatchLog(logWithNoCoupon);

    expect(result.couponApplied).toBe(false);
  });

  it('should handle error status with errorMessage populated', () => {
    const errorLog = {
      ...fullProtoLog,
      status: 'failed',
      errorMessage: 'Group not found: 5511999999999@g.us',
    };
    const result = fromProtoDispatchLog(errorLog);

    expect(result.status).toBe('error');
    expect(result.errorMessage).toBe('Group not found: 5511999999999@g.us');
  });
});
