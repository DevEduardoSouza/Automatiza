import { fireEvent, render, screen } from '@testing-library/react';
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

import type { HistoryStats } from '@/types/action-history';
import { HistoryStatsCards } from '../history-stats-cards';

// ============================================================
// Test data
// ============================================================

const DEFAULT_STATS: HistoryStats = {
  total: 150,
  completed: 100,
  running: 30,
  failed: 20,
};

const ZERO_STATS: HistoryStats = {
  total: 0,
  completed: 0,
  running: 0,
  failed: 0,
};

// ============================================================
// Tests
// ============================================================

describe('HistoryStatsCards', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should render 4 stat cards', () => {
    const { container } = render(<HistoryStatsCards stats={DEFAULT_STATS} />);

    const cards = container.querySelectorAll('.grid > div');
    expect(cards).toHaveLength(4);
  });

  it('should display correct values for each stat', () => {
    render(<HistoryStatsCards stats={DEFAULT_STATS} />);

    expect(screen.getByText('150')).toBeInTheDocument();
    expect(screen.getByText('100')).toBeInTheDocument();
    expect(screen.getByText('30')).toBeInTheDocument();
    expect(screen.getByText('20')).toBeInTheDocument();
  });

  it('should display correct i18n labels for each stat', () => {
    render(<HistoryStatsCards stats={DEFAULT_STATS} />);

    expect(screen.getByText('pages.history.stats.total')).toBeInTheDocument();
    expect(screen.getByText('pages.history.stats.completed')).toBeInTheDocument();
    expect(screen.getByText('pages.history.stats.running')).toBeInTheDocument();
    expect(screen.getByText('pages.history.stats.failed')).toBeInTheDocument();
  });

  it('should apply correct color classes to stat values', () => {
    render(<HistoryStatsCards stats={DEFAULT_STATS} />);

    // completed has text-green-600
    const completedValue = screen.getByText('100');
    expect(completedValue.className).toContain('text-green-600');

    // running has text-blue-600
    const runningValue = screen.getByText('30');
    expect(runningValue.className).toContain('text-blue-600');

    // failed has text-red-600
    const failedValue = screen.getByText('20');
    expect(failedValue.className).toContain('text-red-600');
  });

  it('should render icons with aria-hidden="true"', () => {
    render(<HistoryStatsCards stats={DEFAULT_STATS} />);

    const icons = document.querySelectorAll('svg[aria-hidden="true"]');
    expect(icons).toHaveLength(4);
  });

  it('should change background on hover', () => {
    const { container } = render(<HistoryStatsCards stats={DEFAULT_STATS} />);

    const firstCard = container.querySelectorAll('.grid > div')[0];

    // Before hover
    expect(firstCard).toHaveStyle({ backgroundColor: 'rgba(141, 45, 83, 0.08)' });

    // Hover
    fireEvent.mouseEnter(firstCard);
    expect(firstCard).toHaveStyle({ backgroundColor: 'rgba(141, 45, 83, 0.12)' });

    // Leave
    fireEvent.mouseLeave(firstCard);
    expect(firstCard).toHaveStyle({ backgroundColor: 'rgba(141, 45, 83, 0.08)' });
  });

  it('should render zero stats correctly', () => {
    render(<HistoryStatsCards stats={ZERO_STATS} />);

    const zeros = screen.getAllByText('0');
    expect(zeros).toHaveLength(4);
  });
});
