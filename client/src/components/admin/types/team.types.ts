/**
 * Types for the TeamTab component and useTeamTab hook.
 * Manages administrative team members (admins).
 */

export interface TeamMember {
  id: string;
  user_id: string;
  role: string;
  created_at: string;
  email: string;
}

export interface ApiError {
  message: string;
  code?: string;
}

export interface UseTeamTabReturn {
  teamMembers: TeamMember[];
  loading: boolean;
  isDialogOpen: boolean;
  setIsDialogOpen: (open: boolean) => void;
  newAdminEmail: string;
  setNewAdminEmail: (email: string) => void;
  handleAddAdmin: () => Promise<void>;
  handleRemoveAdmin: (id: string, email: string) => Promise<void>;
}
