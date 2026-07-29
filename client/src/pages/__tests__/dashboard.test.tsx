import { act, render, screen, waitFor } from '@testing-library/react';
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
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

vi.mock('@/components/error-boundary', () => ({
  ErrorBoundary: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// Mock recharts components to avoid jsdom rendering issues
vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  AreaChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="area-chart">{children}</div>
  ),
  Area: () => <div data-testid="area" />,
  BarChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  Bar: () => <div data-testid="bar" />,
  ReferenceLine: () => <div data-testid="reference-line" />,
  Legend: () => <div data-testid="legend" />,
  CartesianGrid: () => <div data-testid="cartesian-grid" />,
  XAxis: () => <div data-testid="x-axis" />,
  YAxis: () => <div data-testid="y-axis" />,
  Tooltip: () => <div data-testid="tooltip" />,
}));

// ============================================================
// Mock: useDashboard
// ============================================================

const defaultDashboardReturn = {
  stats: { totalGroups: 12, fullGroups: 3, totalMembers: 1250, activeLinks: 8 },
  announcement: { enabled: false, title: '', body: '' },
  statCards: [
    {
      title: 'pages.dashboard.stats.totalGroups',
      value: 12,
      icon: () => null,
      iconColor: 'bg-primary',
    },
    {
      title: 'pages.dashboard.stats.fullGroups',
      value: 3,
      icon: () => null,
      iconColor: 'bg-primary',
    },
    {
      title: 'pages.dashboard.stats.totalMembers',
      value: 1250,
      icon: () => null,
      iconColor: 'bg-primary',
    },
    {
      title: 'pages.dashboard.stats.activeLinks',
      value: 8,
      icon: () => null,
      iconColor: 'bg-primary',
    },
  ],
  isLoading: false,
  isError: false,
};

let dashboardReturn = { ...defaultDashboardReturn };

vi.mock('@/hooks/use-dashboard', () => ({
  useDashboard: () => dashboardReturn,
}));

// ============================================================
// Mock: useDashboardHistory
// ============================================================

const defaultHistoryReturn = {
  data: [
    { label: 'Seg', members: 100, groups: 5, newMembers: 10, newGroups: 1, joins: 10, leaves: 2 },
    { label: 'Ter', members: 120, groups: 6, newMembers: 20, newGroups: 1, joins: 20, leaves: 5 },
    { label: 'Qua', members: 150, groups: 7, newMembers: 30, newGroups: 1, joins: 30, leaves: 8 },
  ],
  isLoading: false,
  isError: false,
};

let historyReturn = { ...defaultHistoryReturn };

vi.mock('@/hooks/use-dashboard-connect', () => ({
  useDashboardHistory: () => historyReturn,
}));

// PlanGroupUsageCard (dentro do dashboard) chama useProfilesConnect -> useAuth,
// que exige o AuthProvider. Sem limite de plano o card renderiza null.
vi.mock('@/hooks/use-profiles-connect', () => ({
  useProfilesConnect: () => ({ limits: null, loading: false }),
}));

// ============================================================
// Import component AFTER mocks
// ============================================================

import Dashboard from '../dashboard';

// ============================================================
// Tests
// ============================================================

describe('Dashboard Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    dashboardReturn = {
      stats: { totalGroups: 12, fullGroups: 3, totalMembers: 1250, activeLinks: 8 },
      announcement: { enabled: false, title: '', body: '' },
      statCards: [
        {
          title: 'pages.dashboard.stats.totalGroups',
          value: 12,
          icon: () => null,
          iconColor: 'bg-primary',
        },
        {
          title: 'pages.dashboard.stats.fullGroups',
          value: 3,
          icon: () => null,
          iconColor: 'bg-primary',
        },
        {
          title: 'pages.dashboard.stats.totalMembers',
          value: 1250,
          icon: () => null,
          iconColor: 'bg-primary',
        },
        {
          title: 'pages.dashboard.stats.activeLinks',
          value: 8,
          icon: () => null,
          iconColor: 'bg-primary',
        },
      ],
      isLoading: false,
      isError: false,
    };
    historyReturn = {
      data: [
        {
          label: 'Seg',
          members: 100,
          groups: 5,
          newMembers: 10,
          newGroups: 1,
          joins: 10,
          leaves: 2,
        },
        {
          label: 'Ter',
          members: 120,
          groups: 6,
          newMembers: 20,
          newGroups: 1,
          joins: 20,
          leaves: 5,
        },
        {
          label: 'Qua',
          members: 150,
          groups: 7,
          newMembers: 30,
          newGroups: 1,
          joins: 30,
          leaves: 8,
        },
      ],
      isLoading: false,
      isError: false,
    };
  });

  it('should show 4 skeleton cards when isLoading', () => {
    dashboardReturn.isLoading = true;
    render(<Dashboard />);

    // When loading, skeletons are rendered instead of stat cards
    const skeletons = document.querySelectorAll('[class*="animate-pulse"], [data-slot="skeleton"]');
    expect(skeletons.length).toBeGreaterThanOrEqual(4);
  });

  it('should render 4 stat cards with formatted values', () => {
    render(<Dashboard />);

    // Check for the stat values rendered as text
    expect(screen.getByText('12')).toBeDefined();
    expect(screen.getByText('3')).toBeDefined();
    expect(screen.getByText('1,250')).toBeDefined();
    expect(screen.getByText('8')).toBeDefined();
  });

  it('should show announcement when enabled', () => {
    dashboardReturn.announcement = {
      enabled: true,
      title: 'Novidade!',
      body: 'Temos uma atualização importante',
    };

    render(<Dashboard />);

    expect(screen.getByText('Novidade!')).toBeDefined();
    expect(screen.getByText('Temos uma atualização importante')).toBeDefined();
  });

  it('should hide announcement when disabled', () => {
    dashboardReturn.announcement = {
      enabled: false,
      title: 'Novidade!',
      body: 'Corpo',
    };

    render(<Dashboard />);

    expect(screen.queryByText('Novidade!')).toBeNull();
  });

  it('should render welcome panel with 3 steps', () => {
    render(<Dashboard />);

    expect(screen.getByText('pages.dashboard.welcome.title')).toBeDefined();
    expect(screen.getByText('pages.dashboard.welcome.step1.title')).toBeDefined();
    expect(screen.getByText('pages.dashboard.welcome.step2.title')).toBeDefined();
    expect(screen.getByText('pages.dashboard.welcome.step3.title')).toBeDefined();
  });

  it('should render chart section with period selector', () => {
    render(<Dashboard />);

    expect(screen.getByText('pages.dashboard.chart.title')).toBeDefined();
    expect(screen.getByText('pages.dashboard.chart.period.weekly')).toBeDefined();
    expect(screen.getByText('pages.dashboard.chart.period.monthly')).toBeDefined();
    expect(screen.getByText('pages.dashboard.chart.period.yearly')).toBeDefined();
  });

  it('should show "no data" message when chart data is empty', () => {
    historyReturn.data = [];

    render(<Dashboard />);

    expect(screen.getByText('common.noData')).toBeDefined();
  });

  it('should show chart loading state', () => {
    historyReturn.isLoading = true;

    render(<Dashboard />);

    expect(screen.getByText('pages.dashboard.chart.loading')).toBeDefined();
  });

  it('should show toast when isError is true', async () => {
    dashboardReturn.isError = true;

    render(<Dashboard />);

    await waitFor(() => {
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'pages.dashboard.error.loadFailed',
        })
      );
    });
  });

  it('should render chart when data is available', () => {
    render(<Dashboard />);

    // Dois containers desde o f115b99: area (membros) + barras (entradas x saidas).
    expect(screen.getAllByTestId('responsive-container')).toHaveLength(2);
    expect(screen.getByTestId('area-chart')).toBeDefined();
    expect(screen.getByTestId('bar-chart')).toBeDefined();
  });

  it('should render chart footer stats', () => {
    render(<Dashboard />);

    expect(screen.getByText('pages.dashboard.chart.growthRate')).toBeDefined();
    expect(screen.getByText('pages.dashboard.chart.joins')).toBeDefined();
    expect(screen.getByText('pages.dashboard.chart.leaves')).toBeDefined();
    expect(screen.getByText('pages.dashboard.chart.activeGroups')).toBeDefined();

    // Somas do periodo, nao o ultimo ponto: 10+20+30 entraram, 2+5+8 sairam.
    expect(screen.getByText('60')).toBeDefined();
    expect(screen.getByText('15')).toBeDefined();
  });

  it('should show growth rate 0% when first data point has zero members', () => {
    historyReturn.data = [
      { label: 'Seg', members: 0, groups: 5, newMembers: 0, newGroups: 0, joins: 0, leaves: 0 },
      { label: 'Ter', members: 50, groups: 6, newMembers: 50, newGroups: 1, joins: 50, leaves: 0 },
    ];

    render(<Dashboard />);

    // Growth rate is 0 when firstMembers is 0 (division by zero guard)
    expect(screen.getByText('+0%')).toBeDefined();
  });

  it('should adjust chart font size on window resize', () => {
    render(<Dashboard />);

    // Simulate small window
    act(() => {
      Object.defineProperty(window, 'innerWidth', { value: 400, writable: true });
      window.dispatchEvent(new Event('resize'));
    });

    // Component re-renders with smaller font size internally
    // We verify the component doesn't crash on resize
    expect(screen.getByText('pages.dashboard.chart.title')).toBeDefined();
  });
});
