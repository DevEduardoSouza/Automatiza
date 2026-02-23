import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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

// ============================================================
// Import after mocks
// ============================================================

import { HistoryClearDialog } from '../history-clear-dialog';

// ============================================================
// Tests
// ============================================================

describe('HistoryClearDialog', () => {
  const mockOnConfirm = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should render trigger button with Trash2 icon', () => {
    render(<HistoryClearDialog onConfirm={mockOnConfirm} loading={false} />);

    const button = screen.getByRole('button', { name: /pages\.history\.clearButton/i });
    expect(button).toBeInTheDocument();

    const icon = button.querySelector('svg');
    expect(icon).toBeInTheDocument();
  });

  it('should disable trigger button when disabled={true}', () => {
    render(<HistoryClearDialog onConfirm={mockOnConfirm} loading={false} disabled={true} />);

    const button = screen.getByRole('button', { name: /pages\.history\.clearButton/i });
    expect(button).toBeDisabled();
  });

  it('should disable trigger button when loading={true}', () => {
    render(<HistoryClearDialog onConfirm={mockOnConfirm} loading={true} />);

    const button = screen.getByRole('button', { name: /pages\.history\.clearButton/i });
    expect(button).toBeDisabled();
  });

  it('should open AlertDialog on trigger click', async () => {
    render(<HistoryClearDialog onConfirm={mockOnConfirm} loading={false} />);

    const button = screen.getByRole('button', { name: /pages\.history\.clearButton/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByRole('alertdialog')).toBeInTheDocument();
    });
  });

  it('should display title and description in dialog', async () => {
    render(<HistoryClearDialog onConfirm={mockOnConfirm} loading={false} />);

    fireEvent.click(screen.getByRole('button', { name: /pages\.history\.clearButton/i }));

    await waitFor(() => {
      expect(screen.getByText('pages.history.clearDialog.title')).toBeInTheDocument();
      expect(screen.getByText('pages.history.clearDialog.description')).toBeInTheDocument();
    });
  });

  it('should close dialog when cancel is clicked', async () => {
    render(<HistoryClearDialog onConfirm={mockOnConfirm} loading={false} />);

    fireEvent.click(screen.getByRole('button', { name: /pages\.history\.clearButton/i }));

    await waitFor(() => {
      expect(screen.getByRole('alertdialog')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('common.cancel'));

    await waitFor(() => {
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });
  });

  it('should call onConfirm when confirm button is clicked', async () => {
    render(<HistoryClearDialog onConfirm={mockOnConfirm} loading={false} />);

    fireEvent.click(screen.getByRole('button', { name: /pages\.history\.clearButton/i }));

    await waitFor(() => {
      expect(screen.getByRole('alertdialog')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('common.confirm'));

    expect(mockOnConfirm).toHaveBeenCalledTimes(1);
  });

  it('should show loading spinner when loading={true} inside dialog', async () => {
    render(<HistoryClearDialog onConfirm={mockOnConfirm} loading={true} disabled={false} />);

    // The trigger is disabled when loading, so we need to render while dialog is open
    // Rerender approach: first open, then switch to loading
    const { rerender } = render(
      <HistoryClearDialog onConfirm={mockOnConfirm} loading={false} />
    );

    // Open dialog
    const triggers = screen.getAllByRole('button', { name: /pages\.history\.clearButton/i });
    fireEvent.click(triggers[triggers.length - 1]);

    await waitFor(() => {
      expect(screen.getByRole('alertdialog')).toBeInTheDocument();
    });

    // Switch to loading state
    rerender(<HistoryClearDialog onConfirm={mockOnConfirm} loading={true} />);

    // Confirm button should be disabled during loading
    const confirmBtn = screen.getByText('common.confirm').closest('button');
    expect(confirmBtn).toBeDisabled();

    // Spinner should be visible (Loader2 with animate-spin)
    const spinner = document.querySelector('.animate-spin');
    expect(spinner).toBeInTheDocument();
  });

  it('should disable confirm button during loading', async () => {
    const { rerender } = render(
      <HistoryClearDialog onConfirm={mockOnConfirm} loading={false} />
    );

    fireEvent.click(screen.getByRole('button', { name: /pages\.history\.clearButton/i }));

    await waitFor(() => {
      expect(screen.getByRole('alertdialog')).toBeInTheDocument();
    });

    rerender(<HistoryClearDialog onConfirm={mockOnConfirm} loading={true} />);

    const confirmBtn = screen.getByText('common.confirm').closest('button');
    expect(confirmBtn).toBeDisabled();
  });
});
