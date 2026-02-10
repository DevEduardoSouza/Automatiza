import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { Eye, Mail, Phone } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { User } from './types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface UserTableProps {
  users: User[];
  adminIds: Set<string>;
  currentUserId: string | undefined;
  isMobile: boolean;
  onOpenDetails: (user: User) => void;
  onWhatsAppCall: (phoneNumber: string | null) => void;
  onMakeAdmin: (userId: string) => void;
  onRevokeAdmin: (userId: string) => void;
  onUpdateMaxAutomations: (userId: string, value: number | null) => void;
  onUsersChange: (users: User[]) => void;
}

/**
 * Tabela de usuários com versão mobile (cards) e desktop (table).
 */
export function UserTable({
  users,
  adminIds,
  currentUserId,
  isMobile,
  onOpenDetails,
  onWhatsAppCall,
  onMakeAdmin,
  onRevokeAdmin,
  onUpdateMaxAutomations,
  onUsersChange,
}: UserTableProps) {
  const { t } = useTranslation();

  const isUserActive = (updatedAt: string) => {
    const thirtyDaysAgo = new Date();
    thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);
    return new Date(updatedAt) > thirtyDaysAgo;
  };

  const isAdmin = (userId: string) => adminIds.has(userId);

  const getPlanBadge = (plan: string) => {
    const badgeColors: Record<string, string> = {
      free: 'secondary',
      starter: 'default',
      pro: 'default',
      master: 'default',
    };
    return (
      <Badge variant={badgeColors[plan] as 'default' | 'secondary' | 'destructive' | 'outline'}>
        {plan.toUpperCase()}
      </Badge>
    );
  };

  if (isMobile) {
    return (
      <div className="space-y-3">
        {users.map((user) => (
          <div
            key={user.id}
            className="rounded-xl border p-3 space-y-3"
            style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <Mail className="h-3 w-3 shrink-0" style={{ color: colors.wineTextMuted }} />
                  <p className="font-mono text-xs truncate">{user.email}</p>
                </div>
                {user.phone_number && (
                  <div className="flex items-center gap-2 mt-1">
                    <Phone className="h-3 w-3 shrink-0" style={{ color: colors.wineTextMuted }} />
                    <p className="text-xs">{user.phone_number}</p>
                  </div>
                )}
              </div>
              {getPlanBadge(user.plan)}
            </div>

            <div className="flex items-center gap-2 text-xs pt-2 border-t">
              {isUserActive(user.updated_at) ? (
                <Badge className="bg-green-500 text-xs">{t('common.active')}</Badge>
              ) : (
                <Badge variant="secondary" className="text-xs">
                  {t('common.inactive')}
                </Badge>
              )}
              <span style={{ color: colors.wineTextMuted }}>•</span>
              <span style={{ color: colors.wineTextMuted }}>
                {t('admin.users.registration')}:{' '}
                {format(new Date(user.created_at), 'dd/MM/yyyy', {
                  locale: ptBR,
                })}
              </span>
            </div>

            <div className="flex gap-2 pt-2 border-t">
              <Button
                size="sm"
                variant="outline"
                onClick={() => onOpenDetails(user)}
                className="flex-1 text-xs"
              >
                <Eye className="h-3 w-3 mr-1" />
                {t('admin.users.actions.details')}
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => onWhatsAppCall(user.phone_number)}
                disabled={!user.phone_number}
                className="flex-1 text-xs"
              >
                <Phone className="h-3 w-3 mr-1" />
                {t('admin.users.actions.call')}
              </Button>
              {isAdmin(user.id) ? (
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => onRevokeAdmin(user.id)}
                  disabled={currentUserId === user.id}
                  className="flex-1 text-xs"
                >
                  {t('admin.users.actions.revokeAdmin')}
                </Button>
              ) : (
                <Button size="sm" onClick={() => onMakeAdmin(user.id)} className="flex-1 text-xs">
                  {t('admin.users.actions.makeAdmin')}
                </Button>
              )}
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="border rounded-lg">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('admin.users.columns.email')}</TableHead>
            <TableHead>{t('admin.users.columns.phone')}</TableHead>
            <TableHead>{t('admin.users.columns.plan')}</TableHead>
            <TableHead>{t('admin.users.columns.automationLimit')}</TableHead>
            <TableHead>{t('admin.users.columns.status')}</TableHead>
            <TableHead>{t('admin.users.columns.registration')}</TableHead>
            <TableHead>{t('admin.users.columns.actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => (
            <TableRow key={user.id}>
              <TableCell className="font-mono text-sm">
                <div className="flex items-center gap-2">
                  <Mail className="h-4 w-4" style={{ color: colors.wineTextMuted }} />
                  {user.email}
                </div>
              </TableCell>
              <TableCell>
                {user.phone_number ? (
                  <div className="flex items-center gap-2">
                    <Phone className="h-4 w-4" style={{ color: colors.wineTextMuted }} />
                    {user.phone_number}
                  </div>
                ) : (
                  <span style={{ color: colors.wineTextMuted }}>-</span>
                )}
              </TableCell>
              <TableCell>{getPlanBadge(user.plan)}</TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    className="w-20"
                    value={
                      user.max_automations === null || typeof user.max_automations === 'undefined'
                        ? ''
                        : String(user.max_automations)
                    }
                    placeholder={t('admin.users.automationLimitPlaceholder')}
                    onChange={(e) => {
                      const raw = e.target.value;
                      const n = raw === '' ? null : Number(raw);
                      const newMax =
                        n === null ? null : Number.isFinite(n) ? Math.max(1, Math.floor(n)) : null;
                      onUsersChange(
                        users.map(
                          (u): User => (u.id === user.id ? { ...u, max_automations: newMax } : u)
                        )
                      );
                    }}
                  />
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      onUpdateMaxAutomations(
                        user.id,
                        user.max_automations === undefined ? null : user.max_automations
                      )
                    }
                  >
                    {t('common.save')}
                  </Button>
                </div>
                <p className="text-xs mt-1" style={{ color: colors.wineTextMuted }}>
                  {t('admin.users.automationLimitDefault')}
                </p>
              </TableCell>
              <TableCell>
                {isUserActive(user.updated_at) ? (
                  <Badge className="bg-green-500">{t('common.active')}</Badge>
                ) : (
                  <Badge variant="secondary">{t('common.inactive')}</Badge>
                )}
              </TableCell>
              <TableCell className="whitespace-nowrap">
                {format(new Date(user.created_at), 'dd/MM/yyyy', {
                  locale: ptBR,
                })}
              </TableCell>
              <TableCell className="space-x-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => onOpenDetails(user)}
                  title={t('admin.users.actions.viewDetails')}
                >
                  <Eye className="h-4 w-4 mr-2" />
                  {t('admin.users.actions.details')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => onWhatsAppCall(user.phone_number)}
                  disabled={!user.phone_number}
                >
                  <Phone className="h-4 w-4 mr-2" />
                  {t('admin.users.actions.call')}
                </Button>
                {isAdmin(user.id) ? (
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => onRevokeAdmin(user.id)}
                    disabled={currentUserId === user.id}
                    title={
                      currentUserId === user.id ? t('admin.users.errors.cannotRemoveOwnAdmin') : ''
                    }
                  >
                    {t('admin.users.actions.revokeAdmin')}
                  </Button>
                ) : (
                  <Button size="sm" onClick={() => onMakeAdmin(user.id)}>
                    {t('admin.users.actions.makeAdmin')}
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
