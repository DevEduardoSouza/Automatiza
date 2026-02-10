export type DistributionStrategy = 'member_limit' | 'click_limit';

export interface Group {
  id: string;
  name: string;
  wa_group_id: string;
  invite_code?: string;
  members_count: number;
  member_limit: number;
  priority?: number;
  click_limit?: number;
  is_favorite?: boolean;
}

export interface GroupPriority {
  id: string;
  name: string;
  wa_group_id: string;
  invite_code?: string;
  members_count: number;
  member_limit: number;
  priority?: number;
  click_limit?: number;
}

export interface SavedRedirectLink {
  id: string;
  user_id: string;
  name: string;
  slug: string;
  distribution_strategy: DistributionStrategy;
  group_priorities: GroupPriority[];
  is_active: boolean;
  total_clicks: number;
  created_at: string;
  updated_at: string;
}
