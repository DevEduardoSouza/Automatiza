import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

const mockT = (key: string) => key;
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: mockT,
    i18n: { language: 'pt-BR' },
  }),
}));

// Mock radix-ui Select to render native-like elements for testing
vi.mock('@/components/ui/select', () => ({
  Select: ({ children, value, onValueChange }: { children: React.ReactNode; value: string; onValueChange: (v: string) => void }) => (
    <div data-testid="select-root" data-value={value}>
      {typeof children === 'function' ? children({ onValueChange }) : children}
      <select
        data-testid="select-native"
        value={value}
        onChange={(e) => onValueChange(e.target.value)}
        style={{ display: 'none' }}
      >
        {/* Options populated by SelectContent children */}
      </select>
    </div>
  ),
  SelectTrigger: ({ children }: { children: React.ReactNode }) => (
    <button type="button" role="combobox">{children}</button>
  ),
  SelectValue: ({ placeholder }: { placeholder?: string }) => (
    <span>{placeholder}</span>
  ),
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <div role="listbox">{children}</div>
  ),
  SelectItem: ({ children, value }: { children: React.ReactNode; value: string }) => (
    <div role="option" data-value={value}>{children}</div>
  ),
}));

vi.mock('@/components/ui/label', () => ({
  Label: ({ children, ...props }: { children: React.ReactNode; htmlFor?: string }) => (
    <label htmlFor={props.htmlFor}>{children}</label>
  ),
}));

// ============================================================
// Import after mocks
// ============================================================

import type { HistoryFilters as HistoryFiltersState } from '@/hooks/use-history';
import { HistoryFilters } from '../history-filters';

// ============================================================
// Tests
// ============================================================

describe('HistoryFilters', () => {
  const defaultFilters: HistoryFiltersState = { eventType: null, status: null };
  const mockOnFiltersChange = vi.fn();
  const mockOnClearFilters = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should render 2 filter selects with labels', () => {
    render(
      <HistoryFilters
        filters={defaultFilters}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    // Label + SelectValue placeholder both render the same text
    expect(screen.getAllByText('pages.history.filters.eventType').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('pages.history.filters.status').length).toBeGreaterThanOrEqual(1);
  });

  it('should render 10 event types plus "all" option', () => {
    render(
      <HistoryFilters
        filters={defaultFilters}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    // "all" option
    expect(screen.getByText('pages.history.filters.allTypes')).toBeInTheDocument();

    // The 10 event type options (rendered via i18n key mapping)
    const listboxes = screen.getAllByRole('listbox');
    const eventTypeListbox = listboxes[0];
    const options = within(eventTypeListbox).getAllByRole('option');
    // 1 "all" + 10 event types = 11
    expect(options).toHaveLength(11);
  });

  it('should render 4 statuses plus "all" option', () => {
    render(
      <HistoryFilters
        filters={defaultFilters}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    expect(screen.getByText('pages.history.filters.allStatuses')).toBeInTheDocument();

    const listboxes = screen.getAllByRole('listbox');
    const statusListbox = listboxes[1];
    const options = within(statusListbox).getAllByRole('option');
    // 1 "all" + 4 statuses = 5
    expect(options).toHaveLength(5);
  });

  it('should render status options for each status', () => {
    render(
      <HistoryFilters
        filters={defaultFilters}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    expect(screen.getByText('pages.history.status.pending')).toBeInTheDocument();
    expect(screen.getByText('pages.history.status.running')).toBeInTheDocument();
    expect(screen.getByText('pages.history.status.completed')).toBeInTheDocument();
    expect(screen.getByText('pages.history.status.failed')).toBeInTheDocument();
  });

  it('should hide clear filters button when no filters active', () => {
    render(
      <HistoryFilters
        filters={defaultFilters}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    expect(screen.queryByText('pages.history.filters.clear')).not.toBeInTheDocument();
  });

  it('should show clear filters button when eventType filter is active', () => {
    render(
      <HistoryFilters
        filters={{ eventType: 'send_message', status: null }}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    expect(screen.getByText('pages.history.filters.clear')).toBeInTheDocument();
  });

  it('should show clear filters button when status filter is active', () => {
    render(
      <HistoryFilters
        filters={{ eventType: null, status: 'completed' }}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    expect(screen.getByText('pages.history.filters.clear')).toBeInTheDocument();
  });

  it('should show clear filters button when both filters are active', () => {
    render(
      <HistoryFilters
        filters={{ eventType: 'send_message', status: 'completed' }}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    expect(screen.getByText('pages.history.filters.clear')).toBeInTheDocument();
  });

  it('should call onClearFilters when clear button is clicked', () => {
    render(
      <HistoryFilters
        filters={{ eventType: 'send_message', status: null }}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    fireEvent.click(screen.getByText('pages.history.filters.clear'));
    expect(mockOnClearFilters).toHaveBeenCalledTimes(1);
  });

  it('should render event type label via i18n', () => {
    render(
      <HistoryFilters
        filters={defaultFilters}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    // Label + placeholder both use the same i18n key
    const labels = screen.getAllByText('pages.history.filters.eventType');
    expect(labels.length).toBeGreaterThanOrEqual(1);
  });

  it('should render status label via i18n', () => {
    render(
      <HistoryFilters
        filters={defaultFilters}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    const labels = screen.getAllByText('pages.history.filters.status');
    expect(labels.length).toBeGreaterThanOrEqual(1);
  });

  it('should render clear button with X icon', () => {
    render(
      <HistoryFilters
        filters={{ eventType: 'send_message', status: null }}
        onFiltersChange={mockOnFiltersChange}
        onClearFilters={mockOnClearFilters}
      />
    );

    const clearButton = screen.getByText('pages.history.filters.clear').closest('button');
    expect(clearButton).toBeInTheDocument();
    const icon = clearButton?.querySelector('svg');
    expect(icon).toBeInTheDocument();
  });
});
