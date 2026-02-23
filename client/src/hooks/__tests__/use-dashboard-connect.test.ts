import { renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useDashboardConnect, useDashboardHistory } from '../use-dashboard-connect';

// ============================================================
// Mock Modules
// ============================================================

const mockUser = { id: 'user-123', email: 'test@example.com' };
let authUser: typeof mockUser | null = mockUser;

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: authUser,
    session: null,
    loading: false,
  }),
}));

// ============================================================
// Mock: ConnectRPC Query
// ============================================================

// Use string sentinels to avoid hoisting issues with vi.mock
vi.mock('@/gen/dashboard/v1/dashboard-DashboardService_connectquery', () => ({
  getDashboardStats: 'getDashboardStats',
  getAnnouncement: 'getAnnouncement',
  getDashboardHistory: 'getDashboardHistory',
}));

vi.mock('@/gen/dashboard/v1/dashboard_pb', () => ({
  Period: {
    WEEKLY: 1,
    MONTHLY: 2,
    YEARLY: 3,
  },
}));

type UseQueryCall = {
  descriptor: string;
  input: Record<string, unknown>;
  options: { enabled: boolean; staleTime: number };
};

const useQueryCalls: UseQueryCall[] = [];
const useQueryReturnMap = new Map<string, ReturnType<typeof createQueryReturn>>();

function createQueryReturn(data: unknown = undefined, isLoading = false, isError = false) {
  return { data, isLoading, isError };
}

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: (descriptor: string, input: unknown, options: unknown) => {
    useQueryCalls.push({
      descriptor,
      input: input as Record<string, unknown>,
      options: options as UseQueryCall['options'],
    });
    return useQueryReturnMap.get(descriptor) ?? createQueryReturn();
  },
}));

// ============================================================
// Tests: useDashboardConnect
// ============================================================

describe('useDashboardConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useQueryCalls.length = 0;
    useQueryReturnMap.clear();
    authUser = mockUser;
  });

  it('should return zero defaults when useQuery has no data', () => {
    const { result } = renderHook(() => useDashboardConnect());

    expect(result.current.stats).toEqual({
      totalGroups: 0,
      fullGroups: 0,
      totalMembers: 0,
      activeLinks: 0,
    });
    expect(result.current.announcement).toEqual({
      enabled: false,
      title: '',
      body: '',
    });
  });

  it('should map proto response correctly to DashboardStatsData', () => {
    useQueryReturnMap.set(
      'getDashboardStats',
      createQueryReturn({
        stats: {
          totalGroups: 12,
          fullGroups: 3,
          totalMembers: 1250,
          activeLinks: 8,
        },
      })
    );

    const { result } = renderHook(() => useDashboardConnect());

    expect(result.current.stats).toEqual({
      totalGroups: 12,
      fullGroups: 3,
      totalMembers: 1250,
      activeLinks: 8,
    });
  });

  it('should set announcement.enabled = false when title is empty', () => {
    useQueryReturnMap.set(
      'getAnnouncement',
      createQueryReturn({
        announcement: { enabled: true, title: '', body: 'Some body' },
      })
    );

    const { result } = renderHook(() => useDashboardConnect());
    expect(result.current.announcement.enabled).toBe(false);
  });

  it('should set announcement.enabled = false when body is empty', () => {
    useQueryReturnMap.set(
      'getAnnouncement',
      createQueryReturn({
        announcement: { enabled: true, title: 'Title', body: '' },
      })
    );

    const { result } = renderHook(() => useDashboardConnect());
    expect(result.current.announcement.enabled).toBe(false);
  });

  it('should set announcement.enabled = true when enabled + title + body filled', () => {
    useQueryReturnMap.set(
      'getAnnouncement',
      createQueryReturn({
        announcement: {
          enabled: true,
          title: 'Novidade!',
          body: 'Temos uma atualização importante',
        },
      })
    );

    const { result } = renderHook(() => useDashboardConnect());
    expect(result.current.announcement).toEqual({
      enabled: true,
      title: 'Novidade!',
      body: 'Temos uma atualização importante',
    });
  });

  it('should return isLoading = true when statsLoading is true', () => {
    useQueryReturnMap.set('getDashboardStats', createQueryReturn(undefined, true));

    const { result } = renderHook(() => useDashboardConnect());
    expect(result.current.isLoading).toBe(true);
  });

  it('should return isLoading = true when announcementLoading is true', () => {
    useQueryReturnMap.set('getAnnouncement', createQueryReturn(undefined, true));

    const { result } = renderHook(() => useDashboardConnect());
    expect(result.current.isLoading).toBe(true);
  });

  it('should return isLoading = false when both queries finished', () => {
    useQueryReturnMap.set('getDashboardStats', createQueryReturn({ stats: {} }, false));
    useQueryReturnMap.set('getAnnouncement', createQueryReturn({ announcement: {} }, false));

    const { result } = renderHook(() => useDashboardConnect());
    expect(result.current.isLoading).toBe(false);
  });

  it('should return isError = true when statsError is true', () => {
    useQueryReturnMap.set('getDashboardStats', createQueryReturn(undefined, false, true));

    const { result } = renderHook(() => useDashboardConnect());
    expect(result.current.isError).toBe(true);
  });

  it('should return isError = true when announcementError is true', () => {
    useQueryReturnMap.set('getAnnouncement', createQueryReturn(undefined, false, true));

    const { result } = renderHook(() => useDashboardConnect());
    expect(result.current.isError).toBe(true);
  });

  it('should pass enabled = false to useQuery when user is null', () => {
    authUser = null;

    renderHook(() => useDashboardConnect());

    const statsCall = useQueryCalls.find((c) => c.descriptor === 'getDashboardStats');
    const announcementCall = useQueryCalls.find((c) => c.descriptor === 'getAnnouncement');

    expect(statsCall?.options.enabled).toBe(false);
    expect(announcementCall?.options.enabled).toBe(false);
  });
});

// ============================================================
// Tests: useDashboardHistory
// ============================================================

describe('useDashboardHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useQueryCalls.length = 0;
    useQueryReturnMap.clear();
    authUser = mockUser;
  });

  it('should convert weekly to Period.WEEKLY', () => {
    renderHook(() => useDashboardHistory('weekly'));

    const historyCall = useQueryCalls.find((c) => c.descriptor === 'getDashboardHistory');
    expect(historyCall?.input.period).toBe(1); // Period.WEEKLY
  });

  it('should convert monthly to Period.MONTHLY', () => {
    renderHook(() => useDashboardHistory('monthly'));

    const historyCall = useQueryCalls.find((c) => c.descriptor === 'getDashboardHistory');
    expect(historyCall?.input.period).toBe(2); // Period.MONTHLY
  });

  it('should convert yearly to Period.YEARLY', () => {
    renderHook(() => useDashboardHistory('yearly'));

    const historyCall = useQueryCalls.find((c) => c.descriptor === 'getDashboardHistory');
    expect(historyCall?.input.period).toBe(3); // Period.YEARLY
  });

  it('should transform dataPoints from proto response', () => {
    useQueryReturnMap.set(
      'getDashboardHistory',
      createQueryReturn({
        dataPoints: [
          {
            label: 'Seg',
            members: 100,
            groups: 5,
            newMembers: 10,
            newGroups: 1,
          },
          {
            label: 'Ter',
            members: 120,
            groups: 6,
            newMembers: 20,
            newGroups: 1,
          },
        ],
      })
    );

    const { result } = renderHook(() => useDashboardHistory('weekly'));

    expect(result.current.data).toEqual([
      { label: 'Seg', members: 100, groups: 5, newMembers: 10, newGroups: 1 },
      { label: 'Ter', members: 120, groups: 6, newMembers: 20, newGroups: 1 },
    ]);
  });

  it('should return empty array when there are no data points', () => {
    const { result } = renderHook(() => useDashboardHistory('weekly'));
    expect(result.current.data).toEqual([]);
  });

  it('should pass enabled = false when user is null', () => {
    authUser = null;

    renderHook(() => useDashboardHistory('weekly'));

    const historyCall = useQueryCalls.find((c) => c.descriptor === 'getDashboardHistory');
    expect(historyCall?.options.enabled).toBe(false);
  });
});
