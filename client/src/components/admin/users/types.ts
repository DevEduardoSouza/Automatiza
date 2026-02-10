/**
 * Tipos e interfaces para o módulo de usuários admin.
 */

export type AutomationStatus = 'active' | 'paused' | 'inactive';

export interface UserAutomationRow {
  id: string;
  name: string;
  status: AutomationStatus;
  mode?: string | null;
  interval_minutes?: number | null;
  start_time?: string | null;
  end_time?: string | null;
  stores?: string[] | null;
  send_groups?: string[] | null;
  categories?: string[] | null;
  last_run_at?: string | null;
  next_run_at?: string | null;
  created_at?: string | null;
}

export interface UserInvestigation {
  profile: User;
  isAdmin: boolean;
  instances: Array<{
    instance_id: string;
    status: string;
    updated_at: string | null;
  }>;
  automationStats: { total: number; active: number; paused: number };
  automations: UserAutomationRow[];
}

export interface SystemEvent {
  id: string;
  created_at: string;
  user_id: string | null;
  instance_id: string | null;
  level: 'debug' | 'info' | 'warn' | 'error';
  scope: string;
  message: string;
  meta: Record<string, unknown>;
}

export interface User {
  id: string;
  email: string;
  plan: string;
  phone_number: string | null;
  max_automations?: number | null;
  billing_status?: string | null;
  billing_suspended_at?: string | null;
  billing_suspension_reason?: string | null;
  billing_suspension_kind?: string | null;
  billing_next_payment_at?: string | null;
  plan_access_until?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ProfileBlockUpdate {
  billing_status: string;
  billing_suspended_at: string;
  billing_suspension_reason: string;
  billing_suspension_kind: string;
  billing_last_event_at: string;
  billing_last_event_type: string;
  updated_at: string;
}

export interface ProfileUnblockUpdate {
  billing_status: null;
  billing_suspended_at: null;
  billing_suspension_reason: null;
  billing_suspension_kind: null;
  billing_last_event_at: string;
  billing_last_event_type: string;
  updated_at: string;
}

export interface AutomationStatusUpdate {
  status: 'active' | 'paused';
  updated_at: string;
}

export interface AutomationNextRunUpdate {
  next_run_at: string;
  updated_at: string;
}

export interface LoadBalancerMapping {
  userId: string;
  serverId: number;
  serverName: string;
  createdAt: string | null;
}

export interface CreateUserFormData {
  email: string;
  password: string;
  phone_number: string;
  plan: string;
}

export interface LoadBalancerServer {
  id: number;
  name: string;
  url: string;
}
