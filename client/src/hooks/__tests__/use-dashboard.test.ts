import { renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useDashboard } from '../use-dashboard';

// ============================================================
// Mock Modules
// ============================================================

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

const defaultConnectReturn = {
  stats: {
    totalGroups: 0,
    fullGroups: 0,
    totalMembers: 0,
    activeLinks: 0,
  },
  announcement: {
    enabled: false,
    title: '',
    body: '',
  },
  isLoading: false,
  isError: false,
};

let connectReturn = { ...defaultConnectReturn };

vi.mock('@/hooks/use-dashboard-connect', () => ({
  useDashboardConnect: () => connectReturn,
}));

// ============================================================
// Tests
// ============================================================

describe('useDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    connectReturn = {
      stats: { totalGroups: 0, fullGroups: 0, totalMembers: 0, activeLinks: 0 },
      announcement: { enabled: false, title: '', body: '' },
      isLoading: false,
      isError: false,
    };
  });

  it('should return zero stats by default', () => {
    const { result } = renderHook(() => useDashboard());

    expect(result.current.stats).toEqual({
      totalGroups: 0,
      fullGroups: 0,
      totalMembers: 0,
      activeLinks: 0,
    });
  });

  it('should return stats from connect hook', () => {
    connectReturn.stats = {
      totalGroups: 12,
      fullGroups: 3,
      totalMembers: 1250,
      activeLinks: 8,
    };

    const { result } = renderHook(() => useDashboard());

    expect(result.current.stats).toEqual({
      totalGroups: 12,
      fullGroups: 3,
      totalMembers: 1250,
      activeLinks: 8,
    });
  });

  it('should return announcement from connect hook', () => {
    connectReturn.announcement = {
      enabled: true,
      title: 'Novidade!',
      body: 'Uma atualização importante',
    };

    const { result } = renderHook(() => useDashboard());

    expect(result.current.announcement).toEqual({
      enabled: true,
      title: 'Novidade!',
      body: 'Uma atualização importante',
    });
  });

  it('should generate 4 statCards with correct i18n keys and values', () => {
    connectReturn.stats = {
      totalGroups: 12,
      fullGroups: 3,
      totalMembers: 1250,
      activeLinks: 8,
    };

    const { result } = renderHook(() => useDashboard());

    expect(result.current.statCards).toHaveLength(4);

    expect(result.current.statCards[0].title).toBe('pages.dashboard.stats.totalGroups');
    expect(result.current.statCards[0].value).toBe(12);

    expect(result.current.statCards[1].title).toBe('pages.dashboard.stats.fullGroups');
    expect(result.current.statCards[1].value).toBe(3);

    expect(result.current.statCards[2].title).toBe('pages.dashboard.stats.totalMembers');
    expect(result.current.statCards[2].value).toBe(1250);

    expect(result.current.statCards[3].title).toBe('pages.dashboard.stats.activeLinks');
    expect(result.current.statCards[3].value).toBe(8);
  });

  it('should propagate isLoading', () => {
    connectReturn.isLoading = true;

    const { result } = renderHook(() => useDashboard());
    expect(result.current.isLoading).toBe(true);
  });

  it('should propagate isError', () => {
    connectReturn.isError = true;

    const { result } = renderHook(() => useDashboard());
    expect(result.current.isError).toBe(true);
  });
});
