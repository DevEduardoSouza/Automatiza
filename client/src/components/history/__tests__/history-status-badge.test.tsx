import { render, screen } from '@testing-library/react';
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

import type { ActionHistoryStatus } from '@/types/action-history';
import { HistoryStatusBadge } from '../history-status-badge';

// ============================================================
// Tests
// ============================================================

describe('HistoryStatusBadge', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should render pending status with Clock icon and correct label', () => {
    render(<HistoryStatusBadge status="pending" />);

    expect(screen.getByText('pages.history.status.pending')).toBeInTheDocument();
    // Icon should be present with aria-hidden
    const icon = document.querySelector('svg[aria-hidden="true"]');
    expect(icon).toBeInTheDocument();
  });

  it('should render running status with Loader2 icon and animate-pulse', () => {
    const { container } = render(<HistoryStatusBadge status="running" />);

    expect(screen.getByText('pages.history.status.running')).toBeInTheDocument();
    const badge = container.querySelector('span.animate-pulse');
    expect(badge).toBeInTheDocument();
  });

  it('should render completed status with CheckCircle2 icon', () => {
    render(<HistoryStatusBadge status="completed" />);

    expect(screen.getByText('pages.history.status.completed')).toBeInTheDocument();
  });

  it('should render failed status with XCircle icon', () => {
    render(<HistoryStatusBadge status="failed" />);

    expect(screen.getByText('pages.history.status.failed')).toBeInTheDocument();
  });

  it('should NOT have animate-pulse for non-running statuses', () => {
    const statuses: ActionHistoryStatus[] = ['pending', 'completed', 'failed'];

    for (const status of statuses) {
      const { container, unmount } = render(<HistoryStatusBadge status={status} />);
      const badge = container.querySelector('span.animate-pulse');
      expect(badge).not.toBeInTheDocument();
      unmount();
    }
  });

  it('should apply green color for completed status', () => {
    const { container } = render(<HistoryStatusBadge status="completed" />);

    const badge = container.querySelector('span');
    expect(badge).toHaveStyle({ color: '#16a34a' });
    expect(badge).toHaveStyle({ backgroundColor: 'rgba(34, 197, 94, 0.1)' });
  });

  it('should apply red color for failed status', () => {
    const { container } = render(<HistoryStatusBadge status="failed" />);

    const badge = container.querySelector('span');
    expect(badge).toHaveStyle({ color: '#dc2626' });
    expect(badge).toHaveStyle({ backgroundColor: 'rgba(239, 68, 68, 0.1)' });
  });

  it('should fallback to pending variant for unknown status', () => {
    // @ts-expect-error Testing invalid status
    render(<HistoryStatusBadge status="unknown_status" />);

    // Falls back to pending variant
    expect(screen.getByText('pages.history.status.pending')).toBeInTheDocument();
  });

  it('should render icon with aria-hidden="true"', () => {
    render(<HistoryStatusBadge status="completed" />);

    const icon = document.querySelector('svg[aria-hidden="true"]');
    expect(icon).toBeInTheDocument();
  });
});
