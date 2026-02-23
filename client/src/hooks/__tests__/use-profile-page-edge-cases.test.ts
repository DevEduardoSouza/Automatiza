import { renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useProfilePage } from '../use-profile-page';

// ============================================================
// Mock Modules (identical to use-profile-page.test.ts)
// ============================================================

const mockToast = vi.fn();

vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
    toasts: [],
    dismiss: vi.fn(),
  }),
}));

const mockUser = {
  id: 'user-123',
  email: 'test@example.com',
  displayName: 'Test User',
  role: 'user',
  emailVerified: true,
  createdAt: new Date(),
};

const mockUseAuth = vi.fn(() => ({
  user: mockUser,
  session: null,
  loading: false,
}));

vi.mock('@/contexts', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

// ============================================================
// Mock Transport and RPC Calls
// ============================================================

interface MockClientMethods {
  getProfile: ReturnType<typeof vi.fn>;
  getUserLimits: ReturnType<typeof vi.fn>;
  updateProfile: ReturnType<typeof vi.fn>;
  changePassword: ReturnType<typeof vi.fn>;
}

const mockClientMethods: MockClientMethods = {
  getProfile: vi.fn(),
  getUserLimits: vi.fn(),
  updateProfile: vi.fn(),
  changePassword: vi.fn(),
};

vi.mock('@connectrpc/connect', () => ({
  createClient: () => mockClientMethods,
}));

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: () => ({}),
}));

// ============================================================
// Mock React Query
// ============================================================

const mockQueryClient = {
  invalidateQueries: vi.fn(),
};

interface QueryOptions {
  queryKey: string[];
  queryFn: () => Promise<unknown>;
  enabled?: boolean;
  staleTime?: number;
}

interface MutationOptions<TData, TError, TVariables> {
  mutationFn: (variables: TVariables) => Promise<TData>;
  onSuccess?: (data: TData) => void;
  onError?: (error: TError) => void;
}

interface MutationResult<TData, TError, TVariables> {
  mutateAsync: (variables: TVariables) => Promise<TData>;
  isPending: boolean;
}

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();

vi.mock('@tanstack/react-query', () => ({
  useQuery: (options: QueryOptions) => mockUseQuery(options),
  useMutation: <TData, TError, TVariables>(
    options: MutationOptions<TData, TError, TVariables>
  ): MutationResult<TData, TError, TVariables> => mockUseMutation(options),
  useQueryClient: () => mockQueryClient,
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (error: unknown) => ({
    code: 'UNKNOWN',
    message: error instanceof Error ? error.message : 'Unknown error',
    userMessage: error instanceof Error ? error.message : 'Unknown error',
    action: 'toast',
    retryable: false,
  }),
}));

// ============================================================
// Test Setup
// ============================================================

describe('useProfilePage - edge cases', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockUseAuth.mockReturnValue({
      user: mockUser,
      session: null,
      loading: false,
    });

    mockUseQuery.mockImplementation(() => ({
      data: null,
      isLoading: false,
      isError: false,
    }));

    mockUseMutation.mockImplementation(
      <TData, TError, TVariables>(
        options: MutationOptions<TData, TError, TVariables>
      ): MutationResult<TData, TError, TVariables> => ({
        mutateAsync: async (variables: TVariables) => {
          const result = await options.mutationFn(variables);
          if (options.onSuccess) {
            options.onSuccess(result);
          }
          return result;
        },
        isPending: false,
      })
    );
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  // ============================================================
  // Error Toasts
  // ============================================================

  describe('error toasts', () => {
    it('shows error toast when profile isError=true', () => {
      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: null, isLoading: false, isError: false };
        }
        return { data: null, isLoading: false, isError: true };
      });

      renderHook(() => useProfilePage());

      // The useEffect triggers toast on profileError
      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.profile.errors.loadProfile',
        description: 'pages.profile.errors.loadProfileMessage',
      });
    });

    it('shows error toast when limits isError=true', () => {
      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: null, isLoading: false, isError: true };
        }
        return { data: null, isLoading: false, isError: false };
      });

      renderHook(() => useProfilePage());

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.profile.errors.loadProfile',
        description: 'pages.profile.errors.loadProfileMessage',
      });
    });
  });

  // ============================================================
  // Null Responses
  // ============================================================

  describe('null responses', () => {
    it('returns null profile when response.profile undefined', () => {
      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: null, isLoading: false, isError: false };
        }
        // Simulates queryFn returning null (when response.profile is undefined)
        return { data: null, isLoading: false, isError: false };
      });

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.profile).toBeNull();
    });

    it('returns null limits when response.limits undefined', () => {
      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: null, isLoading: false, isError: false };
        }
        return { data: null, isLoading: false, isError: false };
      });

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.automationsCount).toBe(0);
      expect(result.current.connectedInstancesCount).toBe(0);
    });
  });

  // ============================================================
  // Unauthenticated Mutations
  // ============================================================

  describe('unauthenticated mutations', () => {
    it('throws notAuthenticated on savePhone without user', async () => {
      mockUseAuth.mockReturnValue({
        user: null,
        session: null,
        loading: false,
      });

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
        isError: false,
      }));

      // Make mutation actually call mutationFn which checks user
      mockUseMutation.mockImplementation((options: MutationOptions<unknown, unknown, unknown>) => ({
        mutateAsync: async (variables: unknown) => {
          return options.mutationFn(variables);
        },
        isPending: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      await expect(result.current.savePhone()).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('throws notAuthenticated on savePassword without user', async () => {
      mockUseAuth.mockReturnValue({
        user: null,
        session: null,
        loading: false,
      });

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
        isError: false,
      }));

      mockUseMutation.mockImplementation((options: MutationOptions<unknown, unknown, unknown>) => ({
        mutateAsync: async (variables: unknown) => {
          return options.mutationFn(variables);
        },
        isPending: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      // Must set valid passwords to pass client-side validation
      const { act } = await import('@testing-library/react');
      act(() => {
        result.current.setCurrentPassword('oldpassword123');
        result.current.setNewPassword('newpassword123');
        result.current.setConfirmPassword('newpassword123');
      });

      await expect(result.current.savePassword()).rejects.toThrow('common.errors.notAuthenticated');
    });
  });

  // ============================================================
  // Phone Mutation Edge Cases
  // ============================================================

  describe('phone mutation edge cases', () => {
    it('trims whitespace from phoneDraft before sending', async () => {
      // Use a stable mock that persists across re-renders
      const phoneMutateAsync = vi.fn().mockResolvedValue({
        id: 'user-123',
        email: 'test@example.com',
        plan: 'pro',
        phone_number: '+5511999999999',
        max_automations: 10,
        is_blocked: false,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      });

      // Return the same stable mock across re-renders
      mockUseMutation.mockImplementation((options: MutationOptions<unknown, unknown, unknown>) => ({
        mutateAsync: phoneMutateAsync,
        isPending: false,
      }));

      const { result } = renderHook(() => useProfilePage());
      const { act } = await import('@testing-library/react');

      act(() => {
        result.current.setPhoneDraft('  +5511999999999  ');
      });

      await act(async () => {
        await result.current.savePhone();
      });

      // savePhone trims the draft before calling mutateAsync
      expect(phoneMutateAsync).toHaveBeenCalledWith('+5511999999999');
    });

    it('throws when updateProfile returns no profile', async () => {
      mockClientMethods.updateProfile.mockResolvedValue({ profile: undefined });

      mockUseMutation.mockImplementation((options: MutationOptions<unknown, unknown, unknown>) => ({
        mutateAsync: async (variables: unknown) => {
          return options.mutationFn(variables);
        },
        isPending: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      await expect(result.current.savePhone()).rejects.toThrow('pages.profile.errors.updateFailed');
    });
  });

  // ============================================================
  // Query Configuration
  // ============================================================

  describe('query configuration', () => {
    it('uses staleTime = 5min for profile', () => {
      renderHook(() => useProfilePage());

      const profileCall = mockUseQuery.mock.calls.find(
        (call: [QueryOptions]) => !call[0].queryKey.includes('limits')
      );
      expect(profileCall?.[0].staleTime).toBe(5 * 60 * 1000);
    });

    it('uses staleTime = 30s for limits', () => {
      renderHook(() => useProfilePage());

      const limitsCall = mockUseQuery.mock.calls.find((call: [QueryOptions]) =>
        call[0].queryKey.includes('limits')
      );
      expect(limitsCall?.[0].staleTime).toBe(30 * 1000);
    });

    it('disables queries when user.id is falsy', () => {
      mockUseAuth.mockReturnValue({
        user: null,
        session: null,
        loading: false,
      });

      renderHook(() => useProfilePage());

      for (const call of mockUseQuery.mock.calls) {
        expect((call[0] as QueryOptions).enabled).toBe(false);
      }
    });

    it('enables queries when user.id exists', () => {
      renderHook(() => useProfilePage());

      for (const call of mockUseQuery.mock.calls) {
        expect((call[0] as QueryOptions).enabled).toBe(true);
      }
    });
  });
});
