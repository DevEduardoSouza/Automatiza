import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

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

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (params) return `${key} ${JSON.stringify(params)}`;
      return key;
    },
    i18n: { language: 'pt-BR' },
  }),
}));

vi.mock('@/components/error-boundary', () => ({
  ErrorBoundary: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// Mock child components to isolate page-level rendering
vi.mock('@/components/automations', () => ({
  AutomationFormDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="automation-form-dialog">Form Dialog</div> : null,
  AutomationTable: ({ automations }: { automations: unknown[] }) => (
    <div data-testid="automation-table">Table with {automations.length} rows</div>
  ),
  AutomationCards: ({ automations }: { automations: unknown[] }) => (
    <div data-testid="automation-cards">Cards with {automations.length} items</div>
  ),
  OffersPreviewDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="offers-preview-dialog">Preview Dialog</div> : null,
}));

const mockNavigationGuard = vi.fn();
vi.mock('@/hooks/use-navigation-guard', () => ({
  useNavigationGuard: (...args: unknown[]) => mockNavigationGuard(...args),
}));

// ============================================================
// Mock: useAutomations
// ============================================================

const defaultUseAutomationsReturn = {
  automations: [] as Array<{ id: string; name: string }>,
  maxAutomations: 5,
  groups: [],
  allGroups: [],
  loading: false,
  dialogOpen: false,
  editingId: null,
  whatsappConnected: true,
  activeStores: ['shopee'],
  runningId: null,
  resettingId: null,
  previewOffers: [],
  loadingPreview: false,
  previewDialogOpen: false,
  formData: {},
  monitorDelayUnit: 'seconds' as const,
  monitorDelayInput: '10',
  availableCoupons: [],
  dispatchTexts: [],
  mlLinkLists: [],
  loadingMlLinkLists: false,
  limitReached: false,
  nearLimit: false,
  refinementOpen: {
    eletronicos: false,
    eletrodomesticos: false,
    homens: false,
    esportes: false,
    moda: false,
    casa: false,
    beleza: false,
  },
  savingAutomation: false,
  deleteTargetId: null as string | null,
  setDeleteTargetId: vi.fn(),
  confirmDelete: vi.fn(),
  resetTargetId: null as string | null,
  resetTargetName: null as string | null,
  setResetTarget: vi.fn(),
  confirmResetHistory: vi.fn(),
  setDialogOpen: vi.fn(),
  setPreviewDialogOpen: vi.fn(),
  setFormData: vi.fn(),
  setMonitorDelayUnit: vi.fn(),
  setMonitorDelayInput: vi.fn(),
  setRefinementOpen: vi.fn(),
  toggleFavorite: vi.fn(),
  handleSave: vi.fn(),
  toggleStatus: vi.fn(),
  handleDelete: vi.fn(),
  openEdit: vi.fn(),
  resetForm: vi.fn(),
  handleRunNow: vi.fn(),
  handleResetHistory: vi.fn(),
  handlePreviewOffers: vi.fn(),
};

let mockUseAutomationsReturn = { ...defaultUseAutomationsReturn };

vi.mock('@/hooks/use-automations', () => ({
  useAutomations: () => mockUseAutomationsReturn,
}));

// ============================================================
// Import after mocks
// ============================================================

import OfferAutomations from '../offer-automations';

// ============================================================
// Tests
// ============================================================

describe('OfferAutomations page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAutomationsReturn = { ...defaultUseAutomationsReturn };
  });

  // ── Loading state ──────────────────────────────────────────

  describe('loading state', () => {
    it('should show spinner when loading', () => {
      mockUseAutomationsReturn.loading = true;

      render(<OfferAutomations />);

      expect(screen.getByText('common.loading')).toBeInTheDocument();
    });

    it('should show animated spinner element', () => {
      mockUseAutomationsReturn.loading = true;

      const { container } = render(<OfferAutomations />);

      const spinner = container.querySelector('.animate-spin');
      expect(spinner).toBeInTheDocument();
    });
  });

  // ── Empty state ────────────────────────────────────────────

  describe('empty state', () => {
    it('should show no automations message', () => {
      mockUseAutomationsReturn.automations = [];

      render(<OfferAutomations />);

      expect(
        screen.getByText('pages.offerAutomations.noAutomations')
      ).toBeInTheDocument();
    });

    it('should show create first button', () => {
      mockUseAutomationsReturn.automations = [];

      render(<OfferAutomations />);

      expect(
        screen.getByText('pages.offerAutomations.createFirst')
      ).toBeInTheDocument();
    });

    it('should disable create first button when limit reached', () => {
      mockUseAutomationsReturn.automations = [];
      mockUseAutomationsReturn.limitReached = true;

      render(<OfferAutomations />);

      const createBtn = screen.getByText('pages.offerAutomations.createFirst').closest('button');
      expect(createBtn).toBeDisabled();
    });
  });

  // ── Populated state ────────────────────────────────────────

  describe('populated state', () => {
    it('should render AutomationTable and AutomationCards', () => {
      mockUseAutomationsReturn.automations = [
        { id: 'a1', name: 'Auto 1' },
        { id: 'a2', name: 'Auto 2' },
      ] as typeof mockUseAutomationsReturn.automations;

      render(<OfferAutomations />);

      expect(screen.getByTestId('automation-table')).toBeInTheDocument();
      expect(screen.getByTestId('automation-cards')).toBeInTheDocument();
    });

    it('should show automation count badge', () => {
      mockUseAutomationsReturn.automations = [
        { id: 'a1', name: 'Auto 1' },
        { id: 'a2', name: 'Auto 2' },
      ] as typeof mockUseAutomationsReturn.automations;
      mockUseAutomationsReturn.maxAutomations = 5;

      render(<OfferAutomations />);

      expect(screen.getByText(/2\/5/)).toBeInTheDocument();
    });
  });

  // ── Alerts ─────────────────────────────────────────────────

  describe('alerts', () => {
    it('should show WhatsApp disconnected alert when not connected', () => {
      mockUseAutomationsReturn.whatsappConnected = false;

      render(<OfferAutomations />);

      expect(
        screen.getByText('pages.offerAutomations.whatsappDisconnected')
      ).toBeInTheDocument();
    });

    it('should NOT show WhatsApp alert when connected', () => {
      mockUseAutomationsReturn.whatsappConnected = true;

      render(<OfferAutomations />);

      expect(
        screen.queryByText('pages.offerAutomations.whatsappDisconnected')
      ).not.toBeInTheDocument();
    });

    it('should show limit reached alert', () => {
      mockUseAutomationsReturn.limitReached = true;
      mockUseAutomationsReturn.nearLimit = true;
      mockUseAutomationsReturn.maxAutomations = 5;

      render(<OfferAutomations />);

      expect(
        screen.getByText('pages.offerAutomations.limits.automationsReached')
      ).toBeInTheDocument();
    });

    it('should show near limit alert', () => {
      mockUseAutomationsReturn.nearLimit = true;
      mockUseAutomationsReturn.limitReached = false;
      mockUseAutomationsReturn.automations = [
        { id: 'a1', name: 'Auto 1' },
        { id: 'a2', name: 'Auto 2' },
        { id: 'a3', name: 'Auto 3' },
        { id: 'a4', name: 'Auto 4' },
      ] as typeof mockUseAutomationsReturn.automations;
      mockUseAutomationsReturn.maxAutomations = 5;

      render(<OfferAutomations />);

      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  // ── Delete AlertDialog ─────────────────────────────────────

  describe('delete AlertDialog', () => {
    it('should render delete dialog when deleteTargetId is set', () => {
      mockUseAutomationsReturn.deleteTargetId = 'auto-1';

      render(<OfferAutomations />);

      expect(
        screen.getByText('pages.offerAutomations.confirm.deleteTitle')
      ).toBeInTheDocument();
      expect(
        screen.getByText('pages.offerAutomations.confirm.deleteDescription')
      ).toBeInTheDocument();
    });

    it('should NOT render delete dialog when deleteTargetId is null', () => {
      mockUseAutomationsReturn.deleteTargetId = null;

      render(<OfferAutomations />);

      expect(
        screen.queryByText('pages.offerAutomations.confirm.deleteTitle')
      ).not.toBeInTheDocument();
    });

    it('should call confirmDelete when action button clicked', () => {
      mockUseAutomationsReturn.deleteTargetId = 'auto-1';

      render(<OfferAutomations />);

      const deleteBtn = screen.getByText('common.delete');
      fireEvent.click(deleteBtn);

      expect(mockUseAutomationsReturn.confirmDelete).toHaveBeenCalled();
    });

    it('should call setDeleteTargetId(null) when cancel clicked', () => {
      mockUseAutomationsReturn.deleteTargetId = 'auto-1';

      render(<OfferAutomations />);

      const cancelBtns = screen.getAllByText('common.cancel');
      fireEvent.click(cancelBtns[0]);

      expect(mockUseAutomationsReturn.setDeleteTargetId).toHaveBeenCalledWith(null);
    });
  });

  // ── Reset history AlertDialog ──────────────────────────────

  describe('reset history AlertDialog', () => {
    it('should render reset dialog when resetTargetId is set', () => {
      mockUseAutomationsReturn.resetTargetId = 'auto-1';
      mockUseAutomationsReturn.resetTargetName = 'Test Auto';

      render(<OfferAutomations />);

      expect(
        screen.getByText('pages.offerAutomations.confirm.resetHistoryTitle')
      ).toBeInTheDocument();
    });

    it('should include automation name in description', () => {
      mockUseAutomationsReturn.resetTargetId = 'auto-1';
      mockUseAutomationsReturn.resetTargetName = 'Test Auto';

      render(<OfferAutomations />);

      expect(
        screen.getByText(/Test Auto/)
      ).toBeInTheDocument();
    });

    it('should call confirmResetHistory when confirm clicked', () => {
      mockUseAutomationsReturn.resetTargetId = 'auto-1';
      mockUseAutomationsReturn.resetTargetName = 'Test Auto';

      render(<OfferAutomations />);

      const confirmBtn = screen.getByText('common.confirm');
      fireEvent.click(confirmBtn);

      expect(mockUseAutomationsReturn.confirmResetHistory).toHaveBeenCalled();
    });
  });

  // ── Navigation guard ───────────────────────────────────────

  describe('navigation guard', () => {
    it('should call useNavigationGuard with savingAutomation=false', () => {
      mockUseAutomationsReturn.savingAutomation = false;

      render(<OfferAutomations />);

      expect(mockNavigationGuard).toHaveBeenCalledWith(false);
    });

    it('should call useNavigationGuard with savingAutomation=true', () => {
      mockUseAutomationsReturn.savingAutomation = true;

      render(<OfferAutomations />);

      expect(mockNavigationGuard).toHaveBeenCalledWith(true);
    });
  });
});
