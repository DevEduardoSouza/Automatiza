import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useJobsRealtime } from '../use-jobs-realtime';

function dispatchJobEvent(type: string) {
  window.dispatchEvent(
    new CustomEvent('sse:job-event', { detail: { type } }),
  );
}

function dispatchPlainEvent() {
  window.dispatchEvent(new Event('sse:job-event'));
}

describe('useJobsRealtime', () => {
  let refetchJobs: ReturnType<typeof vi.fn>;
  let addSpy: ReturnType<typeof vi.spyOn>;
  let removeSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.useFakeTimers();
    refetchJobs = vi.fn();
    addSpy = vi.spyOn(window, 'addEventListener');
    removeSpy = vi.spyOn(window, 'removeEventListener');
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('registers sse:job-event listener on mount', () => {
    renderHook(() => useJobsRealtime(refetchJobs));

    expect(addSpy).toHaveBeenCalledWith(
      'sse:job-event',
      expect.any(Function),
    );
  });

  it('removes listener on unmount', () => {
    const { unmount } = renderHook(() => useJobsRealtime(refetchJobs));
    unmount();

    expect(removeSpy).toHaveBeenCalledWith(
      'sse:job-event',
      expect.any(Function),
    );
  });

  it('clears pending timeout on unmount', () => {
    const clearSpy = vi.spyOn(global, 'clearTimeout');
    const { unmount } = renderHook(() => useJobsRealtime(refetchJobs));

    act(() => dispatchJobEvent('job.status'));
    unmount();

    expect(clearSpy).toHaveBeenCalled();
    vi.advanceTimersByTime(500);
    expect(refetchJobs).not.toHaveBeenCalled();
  });

  it('calls refetchJobs after 500ms debounce for job.status events', () => {
    renderHook(() => useJobsRealtime(refetchJobs));

    act(() => dispatchJobEvent('job.status'));

    expect(refetchJobs).not.toHaveBeenCalled();

    act(() => vi.advanceTimersByTime(500));

    expect(refetchJobs).toHaveBeenCalledTimes(1);
  });

  it('ignores events with a different type', () => {
    renderHook(() => useJobsRealtime(refetchJobs));

    act(() => dispatchJobEvent('job.created'));
    act(() => vi.advanceTimersByTime(500));

    expect(refetchJobs).not.toHaveBeenCalled();
  });

  it('ignores events without detail', () => {
    renderHook(() => useJobsRealtime(refetchJobs));

    act(() => dispatchPlainEvent());
    act(() => vi.advanceTimersByTime(500));

    expect(refetchJobs).not.toHaveBeenCalled();
  });

  it('debounces multiple rapid events into one refetch', () => {
    renderHook(() => useJobsRealtime(refetchJobs));

    act(() => {
      dispatchJobEvent('job.status');
      dispatchJobEvent('job.status');
      dispatchJobEvent('job.status');
    });

    act(() => vi.advanceTimersByTime(500));

    expect(refetchJobs).toHaveBeenCalledTimes(1);
  });

  it('resets debounce timer on each new event', () => {
    renderHook(() => useJobsRealtime(refetchJobs));

    act(() => dispatchJobEvent('job.status'));
    act(() => vi.advanceTimersByTime(300));

    expect(refetchJobs).not.toHaveBeenCalled();

    act(() => dispatchJobEvent('job.status'));
    act(() => vi.advanceTimersByTime(300));

    expect(refetchJobs).not.toHaveBeenCalled();

    act(() => vi.advanceTimersByTime(200));

    expect(refetchJobs).toHaveBeenCalledTimes(1);
  });

  it('allows subsequent refetches after debounce completes', () => {
    renderHook(() => useJobsRealtime(refetchJobs));

    act(() => dispatchJobEvent('job.status'));
    act(() => vi.advanceTimersByTime(500));
    expect(refetchJobs).toHaveBeenCalledTimes(1);

    act(() => dispatchJobEvent('job.status'));
    act(() => vi.advanceTimersByTime(500));
    expect(refetchJobs).toHaveBeenCalledTimes(2);
  });
});
