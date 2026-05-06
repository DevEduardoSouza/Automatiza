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
  // Database id of the WhatsApp instance that owns this group (string form).
  // Optional because not every code path that produces a Group fills it in;
  // the chip filter UI degrades gracefully when missing (group never matches).
  instance_id?: string;
  // Every chip whose sync has seen this group (string form, matches instance_id).
  // instance_id is the "owner" (first to discover); this list is the full
  // participation set used by the chip filter and per-row chip badges. Always
  // includes the owner. Optional for compat — pre-44 rows lack the column,
  // and Pick<> sub-types don't propagate it.
  participating_instance_ids?: string[];
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
