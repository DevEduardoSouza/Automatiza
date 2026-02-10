import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useProfilePage } from '../use-profile-page';

// ============================================================
// Mock Modules
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

vi.mock('@/contexts', () => ({
  useAuth: vi.fn(() => ({
    user: mockUser,
    session: null,
    loading: false,
  })),
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

const mockTransport = {};

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: () => mockTransport,
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

// ============================================================
// Mock Error Info
// ============================================================

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
// Test Data Factories
// ============================================================

function createMockProfile(overrides = {}) {
  return {
    id: 'user-123',
    email: 'test@example.com',
    plan: 'pro',
    phoneNumber: '+5511999999999',
    maxAutomations: 10,
    billingNextPaymentAt: '2026-02-15T00:00:00Z',
    planAccessUntil: '2026-03-15T00:00:00Z',
    isBlocked: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-15T00:00:00Z',
    ...overrides,
  };
}

function createMockLimits(overrides = {}) {
  return {
    automationsCount: 5,
    maxAutomations: 10,
    connectedInstancesCount: 2,
    ...overrides,
  };
}

// ============================================================
// Test Setup
// ============================================================

describe('useProfilePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock implementations
    mockUseQuery.mockImplementation((options: QueryOptions) => {
      if (options.queryKey.includes('limits')) {
        return {
          data: null,
          isLoading: false,
        };
      }
      return {
        data: null,
        isLoading: false,
      };
    });

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
  // 1. Loading State Tests
  // ============================================================

  describe('loading state', () => {
    it('should return loading=true while profile is loading', () => {
      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: null, isLoading: false };
        }
        return { data: null, isLoading: true };
      });

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.loading).toBe(true);
    });

    it('should return loading=true while limits are loading', () => {
      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: null, isLoading: true };
        }
        return { data: createMockProfile(), isLoading: false };
      });

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.loading).toBe(true);
    });

    it('should return loading=false when both profile and limits are loaded', () => {
      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: createMockLimits(), isLoading: false };
        }
        return { data: createMockProfile(), isLoading: false };
      });

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.loading).toBe(false);
    });

    it('should fetch profile and limits on mount', () => {
      mockUseQuery.mockImplementation((options: QueryOptions) => ({
        data: null,
        isLoading: false,
      }));

      renderHook(() => useProfilePage());

      // Verify useQuery was called twice (profile and limits)
      expect(mockUseQuery).toHaveBeenCalledTimes(2);

      // Verify query keys
      const calls = mockUseQuery.mock.calls;
      const queryKeys = calls.map((call) => call[0].queryKey);

      expect(queryKeys).toContainEqual(['profile', 'user-123']);
      expect(queryKeys).toContainEqual(['profile', 'limits', 'user-123']);
    });
  });

  // ============================================================
  // 2. Profile Data Tests
  // ============================================================

  describe('profile data', () => {
    it('should return profile data when loaded', () => {
      const mockProfile = createMockProfile();

      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: createMockLimits(), isLoading: false };
        }
        return {
          data: {
            id: mockProfile.id,
            email: mockProfile.email,
            plan: mockProfile.plan,
            phone_number: mockProfile.phoneNumber,
            max_automations: mockProfile.maxAutomations,
            billing_next_payment_at: mockProfile.billingNextPaymentAt,
            plan_access_until: mockProfile.planAccessUntil,
            is_blocked: mockProfile.isBlocked,
            created_at: mockProfile.createdAt,
            updated_at: mockProfile.updatedAt,
          },
          isLoading: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.profile).not.toBeNull();
      expect(result.current.profile?.id).toBe('user-123');
      expect(result.current.profile?.email).toBe('test@example.com');
      expect(result.current.profile?.plan).toBe('pro');
    });

    it('should return null profile when not loaded', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.profile).toBeNull();
    });

    it('should sync phoneDraft with profile.phone_number', async () => {
      const mockProfile = createMockProfile({ phoneNumber: '+5511888888888' });

      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: createMockLimits(), isLoading: false };
        }
        return {
          data: {
            id: mockProfile.id,
            email: mockProfile.email,
            plan: mockProfile.plan,
            phone_number: mockProfile.phoneNumber,
            max_automations: mockProfile.maxAutomations,
            billing_next_payment_at: mockProfile.billingNextPaymentAt,
            plan_access_until: mockProfile.planAccessUntil,
            is_blocked: mockProfile.isBlocked,
            created_at: mockProfile.createdAt,
            updated_at: mockProfile.updatedAt,
          },
          isLoading: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      // The useEffect should sync phoneDraft with profile.phone_number
      await waitFor(() => {
        expect(result.current.phoneDraft).toBe('+5511888888888');
      });
    });
  });

  // ============================================================
  // 3. Phone Update Tests
  // ============================================================

  describe('phone update', () => {
    it('should call updateProfile mutation when savePhone is called', async () => {
      const mockProfile = createMockProfile();
      let onSuccessCallback: (() => void) | undefined;

      mockUseQuery.mockImplementation((options: QueryOptions) => {
        if (options.queryKey.includes('limits')) {
          return { data: createMockLimits(), isLoading: false };
        }
        return {
          data: {
            id: mockProfile.id,
            email: mockProfile.email,
            plan: mockProfile.plan,
            phone_number: mockProfile.phoneNumber,
            max_automations: mockProfile.maxAutomations,
            billing_next_payment_at: mockProfile.billingNextPaymentAt,
            plan_access_until: mockProfile.planAccessUntil,
            is_blocked: mockProfile.isBlocked,
            created_at: mockProfile.createdAt,
            updated_at: mockProfile.updatedAt,
          },
          isLoading: false,
        };
      });

      const mockMutateAsync = vi.fn().mockResolvedValue({
        id: mockProfile.id,
        email: mockProfile.email,
        plan: mockProfile.plan,
        phone_number: '+5511777777777',
        max_automations: mockProfile.maxAutomations,
        billing_next_payment_at: mockProfile.billingNextPaymentAt,
        plan_access_until: mockProfile.planAccessUntil,
        is_blocked: mockProfile.isBlocked,
        created_at: mockProfile.createdAt,
        updated_at: mockProfile.updatedAt,
      });

      mockUseMutation.mockImplementation((options) => {
        onSuccessCallback = options.onSuccess;
        return {
          mutateAsync: async (phone: string) => {
            const result = await mockMutateAsync(phone);
            if (onSuccessCallback) {
              onSuccessCallback();
            }
            return result;
          },
          isPending: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      // Update phone draft
      act(() => {
        result.current.setPhoneDraft('+5511777777777');
      });

      // Call savePhone
      await act(async () => {
        await result.current.savePhone();
      });

      expect(mockMutateAsync).toHaveBeenCalledWith('+5511777777777');
    });

    it('should invalidate cache after successful phone update', async () => {
      // Track which mutation is being set up (phone = first, password = second)
      let phoneMutationOnSuccess: (() => void) | undefined;
      let mutationCallIndex = 0;

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      mockUseMutation.mockImplementation((options) => {
        const currentIndex = mutationCallIndex++;
        if (currentIndex === 0) {
          // Phone mutation (first call)
          phoneMutationOnSuccess = options.onSuccess;
          return {
            mutateAsync: async () => {
              if (phoneMutationOnSuccess) {
                phoneMutationOnSuccess();
              }
              return {};
            },
            isPending: false,
          };
        }
        // Password mutation (second call) - return no-op
        return {
          mutateAsync: vi.fn().mockResolvedValue(true),
          isPending: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      await act(async () => {
        await result.current.savePhone();
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['profile'],
      });
    });

    it('should show success toast after phone update', async () => {
      let phoneMutationOnSuccess: (() => void) | undefined;
      let mutationCallIndex = 0;

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      mockUseMutation.mockImplementation((options) => {
        const currentIndex = mutationCallIndex++;
        if (currentIndex === 0) {
          // Phone mutation (first call)
          phoneMutationOnSuccess = options.onSuccess;
          return {
            mutateAsync: async () => {
              if (phoneMutationOnSuccess) {
                phoneMutationOnSuccess();
              }
              return {};
            },
            isPending: false,
          };
        }
        // Password mutation (second call) - return no-op
        return {
          mutateAsync: vi.fn().mockResolvedValue(true),
          isPending: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      await act(async () => {
        await result.current.savePhone();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.profile.success.phoneUpdated',
        description: 'pages.profile.success.phoneUpdatedMessage',
      });
    });

    it('should show error toast when phone update fails', async () => {
      let phoneMutationOnError: ((error: Error) => void) | undefined;
      let mutationCallIndex = 0;
      const testError = new Error('Network error');

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      mockUseMutation.mockImplementation((options) => {
        const currentIndex = mutationCallIndex++;
        if (currentIndex === 0) {
          // Phone mutation (first call)
          phoneMutationOnError = options.onError;
          return {
            mutateAsync: async () => {
              if (phoneMutationOnError) {
                phoneMutationOnError(testError);
              }
              throw testError;
            },
            isPending: false,
          };
        }
        // Password mutation (second call) - return no-op
        return {
          mutateAsync: vi.fn().mockResolvedValue(true),
          isPending: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      await act(async () => {
        try {
          await result.current.savePhone();
        } catch {
          // Expected to throw
        }
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.profile.errors.savePhone',
        description: 'Network error',
      });
    });
  });

  // ============================================================
  // 4. Password Change Tests
  // ============================================================

  describe('password change', () => {
    it('should validate that current password is required', async () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      // Set new password but not current
      act(() => {
        result.current.setNewPassword('newpassword123');
        result.current.setConfirmPassword('newpassword123');
      });

      await act(async () => {
        await result.current.savePassword();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.profile.errors.invalidPassword',
        description: 'pages.profile.errors.currentPasswordRequired',
        variant: 'destructive',
      });
    });

    it('should validate minimum password length (8 chars)', async () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setCurrentPassword('oldpass123');
        result.current.setNewPassword('short'); // Less than 8 chars
        result.current.setConfirmPassword('short');
      });

      await act(async () => {
        await result.current.savePassword();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.profile.errors.invalidPassword',
        description: 'pages.profile.errors.invalidPasswordMessage',
        variant: 'destructive',
      });
    });

    it('should validate that confirmation matches new password', async () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setCurrentPassword('oldpass123');
        result.current.setNewPassword('newpassword123');
        result.current.setConfirmPassword('differentpassword'); // Mismatch
      });

      await act(async () => {
        await result.current.savePassword();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.profile.errors.passwordMismatch',
        description: 'pages.profile.errors.passwordMismatchMessage',
        variant: 'destructive',
      });
    });

    it('should call changePassword mutation with valid data', async () => {
      const mockMutateAsync = vi.fn().mockResolvedValue(true);

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      mockUseMutation.mockImplementation((options) => ({
        mutateAsync: async (params: {
          currentPassword: string;
          newPassword: string;
        }) => {
          const result = await mockMutateAsync(params);
          if (options.onSuccess) {
            options.onSuccess(result);
          }
          return result;
        },
        isPending: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setCurrentPassword('oldpass123');
        result.current.setNewPassword('newpassword123');
        result.current.setConfirmPassword('newpassword123');
      });

      await act(async () => {
        await result.current.savePassword();
      });

      expect(mockMutateAsync).toHaveBeenCalledWith({
        currentPassword: 'oldpass123',
        newPassword: 'newpassword123',
      });
    });

    it('should clear password fields after successful change', async () => {
      let capturedOnSuccess: (() => void) | undefined;

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      mockUseMutation.mockImplementation((options) => {
        capturedOnSuccess = options.onSuccess;
        return {
          mutateAsync: async () => {
            if (capturedOnSuccess) {
              capturedOnSuccess();
            }
            return true;
          },
          isPending: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setCurrentPassword('oldpass123');
        result.current.setNewPassword('newpassword123');
        result.current.setConfirmPassword('newpassword123');
      });

      await act(async () => {
        await result.current.savePassword();
      });

      expect(result.current.currentPassword).toBe('');
      expect(result.current.newPassword).toBe('');
      expect(result.current.confirmPassword).toBe('');
    });

    it('should show success toast after password change', async () => {
      let capturedOnSuccess: (() => void) | undefined;

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      mockUseMutation.mockImplementation((options) => {
        capturedOnSuccess = options.onSuccess;
        return {
          mutateAsync: async () => {
            if (capturedOnSuccess) {
              capturedOnSuccess();
            }
            return true;
          },
          isPending: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setCurrentPassword('oldpass123');
        result.current.setNewPassword('newpassword123');
        result.current.setConfirmPassword('newpassword123');
      });

      await act(async () => {
        await result.current.savePassword();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.profile.success.passwordUpdated',
        description: 'pages.profile.success.passwordUpdatedMessage',
      });
    });

    it('should show error toast when password change fails', async () => {
      let capturedOnError: ((error: Error) => void) | undefined;
      const testError = new Error('Wrong current password');

      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      mockUseMutation.mockImplementation((options) => {
        capturedOnError = options.onError;
        return {
          mutateAsync: async () => {
            if (capturedOnError) {
              capturedOnError(testError);
            }
            throw testError;
          },
          isPending: false,
        };
      });

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setCurrentPassword('wrongpass');
        result.current.setNewPassword('newpassword123');
        result.current.setConfirmPassword('newpassword123');
      });

      await act(async () => {
        try {
          await result.current.savePassword();
        } catch {
          // Expected to throw
        }
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.profile.errors.updatePassword',
        description: 'Wrong current password',
      });
    });
  });

  // ============================================================
  // 5. Derived Values Tests
  // ============================================================

  describe('derived values', () => {
    describe('automationsLimitText', () => {
      it('should show count only when no max limit', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return {
              data: {
                automations_count: 5,
                max_automations: null,
                connected_instances_count: 2,
              },
              isLoading: false,
            };
          }
          return {
            data: {
              id: 'user-123',
              max_automations: null,
            },
            isLoading: false,
          };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.automationsLimitText).toBe('5 pages.profile.accountInfo.automations');
      });

      it('should show count/max format when limit exists', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return {
              data: {
                automations_count: 5,
                max_automations: 10,
                connected_instances_count: 2,
              },
              isLoading: false,
            };
          }
          return {
            data: {
              id: 'user-123',
              max_automations: 10,
            },
            isLoading: false,
          };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.automationsLimitText).toBe(
          '5/10 pages.profile.accountInfo.automations'
        );
      });

      it('should handle zero automations count', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return {
              data: {
                automations_count: 0,
                max_automations: 10,
                connected_instances_count: 0,
              },
              isLoading: false,
            };
          }
          return {
            data: {
              id: 'user-123',
              max_automations: 10,
            },
            isLoading: false,
          };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.automationsLimitText).toBe(
          '0/10 pages.profile.accountInfo.automations'
        );
      });
    });

    describe('formatDateBR', () => {
      it('should format valid ISO date to pt-BR locale', () => {
        mockUseQuery.mockImplementation(() => ({
          data: null,
          isLoading: false,
        }));

        const { result } = renderHook(() => useProfilePage());

        // The exact format depends on locale, but it should contain date parts
        const formatted = result.current.formatDateBR('2026-01-15T10:30:00Z');

        // Should contain date parts (not return dash)
        expect(formatted).not.toBe('-');
        expect(formatted).toContain('15');
      });

      it('should return dash for null input', () => {
        mockUseQuery.mockImplementation(() => ({
          data: null,
          isLoading: false,
        }));

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.formatDateBR(null)).toBe('-');
      });

      it('should return dash for undefined input', () => {
        mockUseQuery.mockImplementation(() => ({
          data: null,
          isLoading: false,
        }));

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.formatDateBR(undefined)).toBe('-');
      });

      it('should return dash for invalid date string', () => {
        mockUseQuery.mockImplementation(() => ({
          data: null,
          isLoading: false,
        }));

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.formatDateBR('not-a-date')).toBe('-');
      });

      it('should return dash for empty string', () => {
        mockUseQuery.mockImplementation(() => ({
          data: null,
          isLoading: false,
        }));

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.formatDateBR('')).toBe('-');
      });
    });

    describe('maxAutomationsEffective', () => {
      it('should return null when profile has no max_automations', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return { data: null, isLoading: false };
          }
          return {
            data: {
              id: 'user-123',
              max_automations: null,
            },
            isLoading: false,
          };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.maxAutomationsEffective).toBeNull();
      });

      it('should return floor value for positive max_automations', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return { data: null, isLoading: false };
          }
          return {
            data: {
              id: 'user-123',
              max_automations: 15,
            },
            isLoading: false,
          };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.maxAutomationsEffective).toBe(15);
      });

      it('should return null for zero max_automations', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return { data: null, isLoading: false };
          }
          return {
            data: {
              id: 'user-123',
              max_automations: 0,
            },
            isLoading: false,
          };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.maxAutomationsEffective).toBeNull();
      });

      it('should return null for negative max_automations', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return { data: null, isLoading: false };
          }
          return {
            data: {
              id: 'user-123',
              max_automations: -5,
            },
            isLoading: false,
          };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.maxAutomationsEffective).toBeNull();
      });
    });

    describe('connectedInstancesCount', () => {
      it('should return connected instances count from limits', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return {
              data: {
                automations_count: 5,
                max_automations: 10,
                connected_instances_count: 3,
              },
              isLoading: false,
            };
          }
          return { data: null, isLoading: false };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.connectedInstancesCount).toBe(3);
      });

      it('should return 0 when limits are null', () => {
        mockUseQuery.mockImplementation(() => ({
          data: null,
          isLoading: false,
        }));

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.connectedInstancesCount).toBe(0);
      });
    });

    describe('automationsCount', () => {
      it('should return automations count from limits', () => {
        mockUseQuery.mockImplementation((options: QueryOptions) => {
          if (options.queryKey.includes('limits')) {
            return {
              data: {
                automations_count: 7,
                max_automations: 10,
                connected_instances_count: 2,
              },
              isLoading: false,
            };
          }
          return { data: null, isLoading: false };
        });

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.automationsCount).toBe(7);
      });

      it('should return 0 when limits are null', () => {
        mockUseQuery.mockImplementation(() => ({
          data: null,
          isLoading: false,
        }));

        const { result } = renderHook(() => useProfilePage());

        expect(result.current.automationsCount).toBe(0);
      });
    });
  });

  // ============================================================
  // 6. State Setters Tests
  // ============================================================

  describe('state setters', () => {
    it('should update phoneDraft when setPhoneDraft is called', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setPhoneDraft('+5511123456789');
      });

      expect(result.current.phoneDraft).toBe('+5511123456789');
    });

    it('should update currentPassword when setCurrentPassword is called', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setCurrentPassword('myCurrentPass');
      });

      expect(result.current.currentPassword).toBe('myCurrentPass');
    });

    it('should update newPassword when setNewPassword is called', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setNewPassword('myNewPass123');
      });

      expect(result.current.newPassword).toBe('myNewPass123');
    });

    it('should update confirmPassword when setConfirmPassword is called', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      const { result } = renderHook(() => useProfilePage());

      act(() => {
        result.current.setConfirmPassword('myNewPass123');
      });

      expect(result.current.confirmPassword).toBe('myNewPass123');
    });
  });

  // ============================================================
  // 7. Saving States Tests
  // ============================================================

  describe('saving states', () => {
    it('should return savingPhone from mutation pending state', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      // First mutation call (updateProfile) returns isPending: true
      let callIndex = 0;
      mockUseMutation.mockImplementation(() => {
        const result = {
          mutateAsync: vi.fn().mockResolvedValue({}),
          isPending: callIndex === 0, // First call is phone mutation
        };
        callIndex++;
        return result;
      });

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.savingPhone).toBe(true);
    });

    it('should return savingPassword from mutation pending state', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
      }));

      // Second mutation call (changePassword) returns isPending: true
      let callIndex = 0;
      mockUseMutation.mockImplementation(() => {
        const result = {
          mutateAsync: vi.fn().mockResolvedValue({}),
          isPending: callIndex === 1, // Second call is password mutation
        };
        callIndex++;
        return result;
      });

      const { result } = renderHook(() => useProfilePage());

      expect(result.current.savingPassword).toBe(true);
    });
  });
});
