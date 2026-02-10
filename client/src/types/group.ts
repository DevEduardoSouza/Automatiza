/**
 * Represents a WhatsApp group managed by the system.
 */
export interface Group {
  id: string;
  name: string;
  members_count: number;
  member_limit: number;
  status: string;
  wa_group_id: string;
  invite_code: string | null;
  is_admin: boolean;
  is_favorite: boolean;
}

/**
 * Subset of Group for selection dialogs (without member counts).
 */
export type AvailableGroup = Pick<
  Group,
  'id' | 'name' | 'wa_group_id' | 'is_admin' | 'is_favorite'
>;

/**
 * Subset of Group for scheduling dialogs (without is_admin).
 */
export type ScheduleAvailableGroup = Pick<Group, 'id' | 'name' | 'wa_group_id' | 'is_favorite'>;
