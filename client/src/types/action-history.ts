/**
 * Types and interfaces for action history
 */
import type { LucideIcon } from 'lucide-react';

/**
 * Possible status values for an action history entry
 */
export type ActionHistoryStatus = 'pending' | 'running' | 'completed' | 'failed';

/**
 * Represents a single action history record
 */
export interface ActionHistory {
  id: string;
  action_type: string;
  description: string;
  status: ActionHistoryStatus;
  target_count: number;
  success_count: number;
  error_count: number;
  error_message: string | null;
  created_at: string;
  completed_at: string | null;
}

/**
 * Status badge variant configuration
 */
export interface StatusVariant {
  label: string;
  className: string;
  icon: LucideIcon;
}

/**
 * Aggregated statistics for action history
 */
export interface HistoryStats {
  total: number;
  completed: number;
  running: number;
  failed: number;
}
