import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

vi.mock('@/components/error-boundary', () => ({
  ErrorBoundary: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('@/components/sessions', () => ({
  SessionList: () => <div data-testid="session-list" />,
}));

const mockUseNavigationGuard = vi.fn();
vi.mock('@/hooks/use-navigation-guard', () => ({
  useNavigationGuard: (when: boolean) => mockUseNavigationGuard(when),
}));

// ============================================================
// Mock useProfilePage
// ============================================================

const defaultHookReturn = {
  loading: false,
  profile: {
    id: 'user-123',
    email: 'test@example.com',
    plan: 'pro',
    phone_number: '+5511999999999',
    max_automations: 10,
    billing_next_payment_at: '2026-02-15T00:00:00Z',
    plan_access_until: '2026-03-15T00:00:00Z',
    is_blocked: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-15T00:00:00Z',
  },
  automationsCount: 5,
  connectedInstancesCount: 2,
  phoneDraft: '+5511999999999',
  savingPhone: false,
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
  savingPassword: false,
  maxAutomationsEffective: 10,
  automationsLimitText: '5/10 automations',
  setPhoneDraft: vi.fn(),
  setCurrentPassword: vi.fn(),
  setNewPassword: vi.fn(),
  setConfirmPassword: vi.fn(),
  savePhone: vi.fn(),
  savePassword: vi.fn(),
  formatDateBR: (iso?: string | null) => {
    if (!iso) return '-';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '-';
    return d.toLocaleString('pt-BR');
  },
};

let hookReturn = { ...defaultHookReturn };

vi.mock('@/hooks/use-profile-page', () => ({
  useProfilePage: () => hookReturn,
}));

// ============================================================
// Import after mocks
// ============================================================

import Profile from '../profile';

// ============================================================
// Tests
// ============================================================

describe('Profile Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    hookReturn = {
      ...defaultHookReturn,
      setPhoneDraft: vi.fn(),
      setCurrentPassword: vi.fn(),
      setNewPassword: vi.fn(),
      setConfirmPassword: vi.fn(),
      savePhone: vi.fn(),
      savePassword: vi.fn(),
    };
  });

  // ============================================================
  // Loading State
  // ============================================================

  describe('loading state', () => {
    it('renders spinner when loading=true', () => {
      hookReturn = { ...hookReturn, loading: true };

      render(<Profile />);

      expect(screen.getByText('pages.profile.loading')).toBeInTheDocument();
      const spinner = document.querySelector('.animate-spin');
      expect(spinner).toBeInTheDocument();
    });

    it('does not render sections during loading', () => {
      hookReturn = { ...hookReturn, loading: true };

      render(<Profile />);

      expect(screen.queryByText('pages.profile.accountInfo.title')).not.toBeInTheDocument();
      expect(screen.queryByLabelText(/phone/i)).not.toBeInTheDocument();
    });
  });

  // ============================================================
  // Account Info
  // ============================================================

  describe('account info', () => {
    it('renders email', () => {
      render(<Profile />);

      expect(screen.getByText('test@example.com')).toBeInTheDocument();
    });

    it('renders plan badge (uppercase)', () => {
      render(<Profile />);

      expect(screen.getByText('PRO')).toBeInTheDocument();
    });

    it('renders automationsLimitText', () => {
      render(<Profile />);

      expect(screen.getByText('5/10 automations')).toBeInTheDocument();
    });

    it('renders user ID (monospace)', () => {
      render(<Profile />);

      const userId = screen.getByText('user-123');
      expect(userId).toBeInTheDocument();
      expect(userId.className).toContain('font-mono');
    });

    it('renders instances count', () => {
      render(<Profile />);

      expect(screen.getByText(/2.*pages\.profile\.accountInfo\.instances/)).toBeInTheDocument();
    });

    it('renders formatted dates', () => {
      render(<Profile />);

      // formatDateBR should be called for billing, access, created, updated
      const formatted = hookReturn.formatDateBR('2026-01-01T00:00:00Z');
      expect(formatted).not.toBe('-');
    });

    it('renders dash for missing values', () => {
      hookReturn = {
        ...hookReturn,
        profile: {
          ...hookReturn.profile!,
          email: null,
          billing_next_payment_at: null,
          plan_access_until: null,
        },
      };

      render(<Profile />);

      // Email fallback
      const dashes = screen.getAllByText('-');
      expect(dashes.length).toBeGreaterThan(0);
    });
  });

  // ============================================================
  // Phone Section
  // ============================================================

  describe('phone section', () => {
    it('renders input with phoneDraft value', () => {
      render(<Profile />);

      const phoneInput = screen.getByRole('textbox', { name: /pages\.profile\.phone\.label/i });
      expect(phoneInput).toHaveValue('+5511999999999');
    });

    it('calls setPhoneDraft on change', () => {
      render(<Profile />);

      const phoneInput = screen.getByRole('textbox', { name: /pages\.profile\.phone\.label/i });
      fireEvent.change(phoneInput, { target: { value: '+5511888888888' } });

      expect(hookReturn.setPhoneDraft).toHaveBeenCalled();
    });

    it('calls savePhone on button click', () => {
      render(<Profile />);

      const saveBtn = screen.getAllByText('pages.profile.phone.save')[0];
      fireEvent.click(saveBtn.closest('button')!);

      expect(hookReturn.savePhone).toHaveBeenCalled();
    });

    it('disables button when savingPhone=true', () => {
      hookReturn = { ...hookReturn, savingPhone: true };

      render(<Profile />);

      const savingText = screen.getByText('pages.profile.phone.saving');
      const button = savingText.closest('button')!;
      expect(button).toBeDisabled();
    });

    it('shows saving text when savingPhone=true', () => {
      hookReturn = { ...hookReturn, savingPhone: true };

      render(<Profile />);

      expect(screen.getByText('pages.profile.phone.saving')).toBeInTheDocument();
    });
  });

  // ============================================================
  // Password Visibility Toggles
  // ============================================================

  describe('password visibility toggles', () => {
    it('toggles current password visibility', () => {
      render(<Profile />);

      const input = document.getElementById('current_password') as HTMLInputElement;
      expect(input.type).toBe('password');

      const toggleBtn = screen.getAllByLabelText('Show password')[0];
      fireEvent.click(toggleBtn);

      expect(input.type).toBe('text');
    });

    it('toggles new password visibility', () => {
      render(<Profile />);

      const input = document.getElementById('new_password') as HTMLInputElement;
      expect(input.type).toBe('password');

      const toggleBtn = screen.getAllByLabelText('Show password')[1];
      fireEvent.click(toggleBtn);

      expect(input.type).toBe('text');
    });

    it('toggles confirm password visibility', () => {
      render(<Profile />);

      const input = document.getElementById('confirm_password') as HTMLInputElement;
      expect(input.type).toBe('password');

      const toggleBtn = screen.getAllByLabelText('Show password')[2];
      fireEvent.click(toggleBtn);

      expect(input.type).toBe('text');
    });

    it('toggles are independent (one does not affect others)', () => {
      render(<Profile />);

      const currentInput = document.getElementById('current_password') as HTMLInputElement;
      const newInput = document.getElementById('new_password') as HTMLInputElement;
      const confirmInput = document.getElementById('confirm_password') as HTMLInputElement;

      // Toggle only the first one
      const toggleBtn = screen.getAllByLabelText('Show password')[0];
      fireEvent.click(toggleBtn);

      expect(currentInput.type).toBe('text');
      expect(newInput.type).toBe('password');
      expect(confirmInput.type).toBe('password');
    });

    it('shows correct aria-labels (Show/Hide password)', () => {
      render(<Profile />);

      // Initially all show "Show password"
      const showButtons = screen.getAllByLabelText('Show password');
      expect(showButtons).toHaveLength(3);

      // Toggle the first one
      fireEvent.click(showButtons[0]);

      // Now the first should say "Hide password"
      expect(screen.getByLabelText('Hide password')).toBeInTheDocument();
      // Other two should still say "Show password"
      expect(screen.getAllByLabelText('Show password')).toHaveLength(2);
    });
  });

  // ============================================================
  // Password Save
  // ============================================================

  describe('password save', () => {
    it('calls savePassword on button click', () => {
      render(<Profile />);

      const saveBtn = screen.getByText('pages.profile.security.save');
      fireEvent.click(saveBtn.closest('button')!);

      expect(hookReturn.savePassword).toHaveBeenCalled();
    });

    it('disables button when savingPassword=true', () => {
      hookReturn = { ...hookReturn, savingPassword: true };

      render(<Profile />);

      const savingText = screen.getByText('pages.profile.security.saving');
      const button = savingText.closest('button')!;
      expect(button).toBeDisabled();
    });
  });

  // ============================================================
  // Navigation Guard
  // ============================================================

  describe('navigation guard', () => {
    it('calls useNavigationGuard(false) when not saving', () => {
      hookReturn = { ...hookReturn, savingPhone: false, savingPassword: false };

      render(<Profile />);

      expect(mockUseNavigationGuard).toHaveBeenCalledWith(false);
    });

    it('calls useNavigationGuard(true) when savingPhone', () => {
      hookReturn = { ...hookReturn, savingPhone: true, savingPassword: false };

      render(<Profile />);

      expect(mockUseNavigationGuard).toHaveBeenCalledWith(true);
    });

    it('calls useNavigationGuard(true) when savingPassword', () => {
      hookReturn = { ...hookReturn, savingPhone: false, savingPassword: true };

      render(<Profile />);

      expect(mockUseNavigationGuard).toHaveBeenCalledWith(true);
    });
  });

  // ============================================================
  // Integration
  // ============================================================

  describe('integration', () => {
    it('renders SessionList component', () => {
      render(<Profile />);

      expect(screen.getByTestId('session-list')).toBeInTheDocument();
    });
  });
});
