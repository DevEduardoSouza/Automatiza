import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

const mockT = (key: string, fallbackOrOptions?: string | Record<string, unknown>) => {
  if (typeof fallbackOrOptions === 'string') return fallbackOrOptions;
  return key;
};
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: mockT,
    i18n: { language: 'pt-BR' },
  }),
}));

// ============================================================
// Import after mocks
// ============================================================

import type { ActionHistory } from '@/types/action-history';
import { HistoryTable } from '../history-table';

// ============================================================
// Test data
// ============================================================

const MOCK_HISTORY: ActionHistory[] = [
  {
    id: 'h1',
    action_type: 'send_message',
    description: 'Sent message to Promo group',
    status: 'completed',
    target_count: 10,
    success_count: 9,
    error_count: 1,
    error_message: null,
    created_at: '2026-01-15T10:00:00Z',
    completed_at: '2026-01-15T10:00:30Z',
  },
  {
    id: 'h2',
    action_type: 'close_groups',
    description: 'Closed maintenance groups',
    status: 'failed',
    target_count: 5,
    success_count: 3,
    error_count: 2,
    error_message: 'Connection timeout',
    created_at: '2026-01-15T09:00:00Z',
    completed_at: '2026-01-15T09:00:10Z',
  },
  {
    id: 'h3',
    action_type: 'automation_run',
    description: 'Running automation',
    status: 'running',
    target_count: 0,
    success_count: 0,
    error_count: 0,
    error_message: null,
    created_at: '2026-01-15T08:00:00Z',
    completed_at: null,
  },
];

const defaultProps = {
  history: MOCK_HISTORY,
  currentPage: 1,
  pageSize: 20,
  totalCount: 3,
  totalPages: 1,
  onPageChange: vi.fn(),
};

// ============================================================
// Tests
// ============================================================

describe('HistoryTable', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // -- Column rendering --

  it('should render all column headers', () => {
    render(<HistoryTable {...defaultProps} />);

    expect(screen.getByText('pages.history.table.columns.dateTime')).toBeInTheDocument();
    expect(screen.getByText('pages.history.table.columns.action')).toBeInTheDocument();
    expect(screen.getByText('pages.history.table.columns.description')).toBeInTheDocument();
    expect(screen.getByText('pages.history.table.columns.status')).toBeInTheDocument();
    expect(screen.getByText('pages.history.table.columns.results')).toBeInTheDocument();
    expect(screen.getByText('pages.history.table.columns.duration')).toBeInTheDocument();
  });

  it('should render correct number of rows', () => {
    const { container } = render(<HistoryTable {...defaultProps} />);

    const rows = container.querySelectorAll('table tbody tr');
    expect(rows).toHaveLength(3);
  });

  // -- Date formatting --

  it('should format dates as dd/MM/yyyy HH:mm', () => {
    render(<HistoryTable {...defaultProps} />);

    // 2026-01-15T10:00:00Z → depends on timezone, check format pattern
    const dateElements = document.querySelectorAll('.font-mono');
    expect(dateElements.length).toBeGreaterThanOrEqual(1);
    // Verify it matches dd/MM/yyyy HH:mm pattern
    const dateText = dateElements[0].textContent || '';
    expect(dateText).toMatch(/\d{2}\/\d{2}\/\d{4} \d{2}:\d{2}/);
  });

  // -- Action labels --

  it('should display action type labels via i18n', () => {
    render(<HistoryTable {...defaultProps} />);

    // Both desktop table and mobile cards render action labels
    expect(screen.getAllByText('pages.history.actions.sendMessage').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('pages.history.actions.closeGroups').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('pages.history.actions.automationRun').length).toBeGreaterThanOrEqual(1);
  });

  it('should fallback to raw action_type for unknown actions', () => {
    const unknownHistory: ActionHistory[] = [
      {
        id: 'h-unknown',
        action_type: 'unknown_action_type',
        description: 'Unknown',
        status: 'completed',
        target_count: 0,
        success_count: 0,
        error_count: 0,
        error_message: null,
        created_at: '2026-01-15T10:00:00Z',
        completed_at: '2026-01-15T10:00:05Z',
      },
    ];

    render(<HistoryTable {...defaultProps} history={unknownHistory} totalCount={1} />);

    // Appears in both desktop and mobile views
    expect(screen.getAllByText('unknown_action_type').length).toBeGreaterThanOrEqual(1);
  });

  // -- Description --

  it('should display descriptions', () => {
    render(<HistoryTable {...defaultProps} />);

    expect(screen.getAllByText('Sent message to Promo group').length).toBeGreaterThanOrEqual(1);
  });

  it('should display error_message when present', () => {
    render(<HistoryTable {...defaultProps} />);

    expect(screen.getAllByText('Connection timeout').length).toBeGreaterThanOrEqual(1);
  });

  it('should truncate description with max-w-xs', () => {
    const { container } = render(<HistoryTable {...defaultProps} />);

    const truncatedEls = container.querySelectorAll('.max-w-xs.truncate');
    expect(truncatedEls.length).toBeGreaterThanOrEqual(1);
  });

  // -- Results --

  it('should display results when target_count > 0', () => {
    render(<HistoryTable {...defaultProps} />);

    // First item: target=10, success=9, error=1
    expect(screen.getAllByText(/10/).length).toBeGreaterThanOrEqual(1);
  });

  it('should display success count in green', () => {
    const { container } = render(<HistoryTable {...defaultProps} />);

    const greenElements = container.querySelectorAll('.text-green-600');
    expect(greenElements.length).toBeGreaterThanOrEqual(1);
  });

  it('should display error count in red', () => {
    const { container } = render(<HistoryTable {...defaultProps} />);

    const redElements = container.querySelectorAll('.text-red-600');
    expect(redElements.length).toBeGreaterThanOrEqual(1);
  });

  it('should hide results when target_count is 0', () => {
    const { container } = render(<HistoryTable {...defaultProps} />);

    // Third item (automation_run) has target_count=0, so no results div
    const rows = container.querySelectorAll('table tbody tr');
    const lastRowResults = within(rows[2] as HTMLElement).queryByText('pages.history.table.total');
    expect(lastRowResults).not.toBeInTheDocument();
  });

  // -- Duration --

  it('should calculate duration in seconds', () => {
    render(<HistoryTable {...defaultProps} />);

    // First item: 30 seconds difference
    expect(screen.getAllByText('30s').length).toBeGreaterThanOrEqual(1);
    // Second item: 10 seconds difference
    expect(screen.getAllByText('10s').length).toBeGreaterThanOrEqual(1);
  });

  it('should display dash when completed_at is null', () => {
    render(<HistoryTable {...defaultProps} />);

    // Third item has no completed_at
    const dashes = screen.getAllByText('-');
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  // -- Pagination controls --

  it('should render 4 pagination buttons', () => {
    render(<HistoryTable {...defaultProps} />);

    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.first/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.previous/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.next/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.last/i })).toBeInTheDocument();
  });

  it('should disable first/prev when on page 1', () => {
    render(<HistoryTable {...defaultProps} currentPage={1} />);

    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.first/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.previous/i })).toBeDisabled();
  });

  it('should disable next/last when on last page', () => {
    render(<HistoryTable {...defaultProps} currentPage={1} totalPages={1} />);

    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.next/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.last/i })).toBeDisabled();
  });

  it('should disable next/last when totalPages is 0', () => {
    render(<HistoryTable {...defaultProps} currentPage={1} totalPages={0} totalCount={0} history={[]} />);

    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.next/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.last/i })).toBeDisabled();
  });

  it('should enable all buttons when on middle page', () => {
    render(<HistoryTable {...defaultProps} currentPage={3} totalPages={5} totalCount={100} />);

    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.first/i })).toBeEnabled();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.previous/i })).toBeEnabled();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.next/i })).toBeEnabled();
    expect(screen.getByRole('button', { name: /pages\.history\.pagination\.last/i })).toBeEnabled();
  });

  it('should call onPageChange(1) when first button clicked', () => {
    const onPageChange = vi.fn();
    render(<HistoryTable {...defaultProps} currentPage={3} totalPages={5} totalCount={100} onPageChange={onPageChange} />);

    fireEvent.click(screen.getByRole('button', { name: /pages\.history\.pagination\.first/i }));
    expect(onPageChange).toHaveBeenCalledWith(1);
  });

  it('should call onPageChange(currentPage-1) when prev button clicked', () => {
    const onPageChange = vi.fn();
    render(<HistoryTable {...defaultProps} currentPage={3} totalPages={5} totalCount={100} onPageChange={onPageChange} />);

    fireEvent.click(screen.getByRole('button', { name: /pages\.history\.pagination\.previous/i }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it('should call onPageChange(currentPage+1) when next button clicked', () => {
    const onPageChange = vi.fn();
    render(<HistoryTable {...defaultProps} currentPage={3} totalPages={5} totalCount={100} onPageChange={onPageChange} />);

    fireEvent.click(screen.getByRole('button', { name: /pages\.history\.pagination\.next/i }));
    expect(onPageChange).toHaveBeenCalledWith(4);
  });

  it('should call onPageChange(totalPages) when last button clicked', () => {
    const onPageChange = vi.fn();
    render(<HistoryTable {...defaultProps} currentPage={3} totalPages={5} totalCount={100} onPageChange={onPageChange} />);

    fireEvent.click(screen.getByRole('button', { name: /pages\.history\.pagination\.last/i }));
    expect(onPageChange).toHaveBeenCalledWith(5);
  });

  // -- Pagination info --

  it('should display pagination info text', () => {
    render(<HistoryTable {...defaultProps} currentPage={1} totalPages={1} totalCount={3} />);

    // Shows "pages.history.pagination.showing" i18n key
    expect(screen.getByText('pages.history.pagination.showing')).toBeInTheDocument();
  });

  it('should display empty message when totalCount is 0', () => {
    render(<HistoryTable {...defaultProps} totalCount={0} history={[]} totalPages={0} />);

    expect(screen.getByText('pages.history.table.empty')).toBeInTheDocument();
  });

  it('should display page number info', () => {
    render(<HistoryTable {...defaultProps} currentPage={2} totalPages={5} totalCount={100} />);

    expect(screen.getByText('2 / 5')).toBeInTheDocument();
  });

  // -- Mobile cards --

  it('should render mobile cards', () => {
    const { container } = render(<HistoryTable {...defaultProps} />);

    // md:hidden container should exist with cards
    const mobileContainer = container.querySelector('.md\\:hidden');
    expect(mobileContainer).toBeInTheDocument();
    const cards = mobileContainer?.querySelectorAll(':scope > div');
    expect(cards?.length).toBe(3);
  });

  // -- Row hover --

  it('should change background on row hover', () => {
    const { container } = render(<HistoryTable {...defaultProps} />);

    const firstRow = container.querySelector('table tbody tr');
    expect(firstRow).toBeInTheDocument();

    fireEvent.mouseEnter(firstRow!);
    expect(firstRow).toHaveStyle({ backgroundColor: 'rgba(141, 45, 83, 0.04)' });

    fireEvent.mouseLeave(firstRow!);
    expect(firstRow?.style.backgroundColor).toBeFalsy();
  });
});
