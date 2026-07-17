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

/** Cargos que podem ser concedidos a um membro da equipe. */
export type TeamRole = 'admin' | 'support';

export interface UseTeamTabReturn {
  teamMembers: TeamMember[];
  loading: boolean;
  isDialogOpen: boolean;
  setIsDialogOpen: (open: boolean) => void;
  newAdminEmail: string;
  setNewAdminEmail: (email: string) => void;
  newMemberRole: TeamRole;
  setNewMemberRole: (role: TeamRole) => void;
  handleAddAdmin: () => Promise<void>;
  handleRemoveAdmin: (id: string, email: string, role: string) => Promise<void>;
}
