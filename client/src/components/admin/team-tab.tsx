import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { useIsMobile } from '@/hooks/use-mobile';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { Plus, Trash2, UserPlus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useTeamTab } from './hooks';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

/**
 * Aba de gerenciamento de equipe administrativa.
 * Permite adicionar e remover administradores do sistema.
 */
export default function TeamTab() {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const {
    teamMembers,
    loading,
    isDialogOpen,
    setIsDialogOpen,
    newAdminEmail,
    setNewAdminEmail,
    handleAddAdmin,
    handleRemoveAdmin,
  } = useTeamTab();

  return (
    <div className="space-y-4">
      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-3 xs:mb-4">
          <div>
            <span
              className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
              style={{ color: colors.wineText }}
            >
              {t('admin.team.title')}
            </span>
            <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
              {t('admin.team.description')}
            </p>
          </div>
          <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
            <DialogTrigger asChild>
              <Button className="w-full md:w-auto">
                <Plus className="h-4 w-4 mr-2" />
                <span className="hidden sm:inline">{t('admin.team.addAdmin')}</span>
                <span className="sm:hidden">{t('admin.team.add')}</span>
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-md" aria-describedby="add-admin-dialog-description">
              <DialogHeader>
                <DialogTitle>{t('admin.team.dialog.title')}</DialogTitle>
                <DialogDescription id="add-admin-dialog-description">
                  {t('admin.team.dialog.description')}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="email">{t('admin.team.dialog.emailLabel')}</Label>
                  <Input
                    id="email"
                    type="email"
                    placeholder={t('admin.team.dialog.emailPlaceholder')}
                    value={newAdminEmail}
                    onChange={(e) => setNewAdminEmail(e.target.value)}
                    aria-required="true"
                  />
                </div>
                <Button onClick={handleAddAdmin} className="w-full">
                  <UserPlus className="h-4 w-4 mr-2" />
                  {t('admin.team.dialog.addButton')}
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        </div>
        <div className="space-y-3 xs:space-y-4">
          {loading ? (
            <div className="text-center py-8 text-sm" role="status" aria-live="polite">
              {t('common.loading')}
            </div>
          ) : teamMembers.length === 0 ? (
            <div className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
              {t('admin.team.noAdmins')}
            </div>
          ) : isMobile ? (
            <div className="space-y-3">
              {teamMembers.map((member) => (
                <div
                  key={member.id}
                  className="rounded-xl border p-3 space-y-3"
                  style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1 min-w-0">
                      <p className="font-mono text-xs truncate">{member.email}</p>
                      <p className="text-xs mt-1" style={{ color: colors.wineTextMuted }}>
                        {format(new Date(member.created_at), 'dd/MM/yyyy HH:mm', {
                          locale: ptBR,
                        })}
                      </p>
                    </div>
                    <Badge className="text-xs">{t('admin.team.roles.admin')}</Badge>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => handleRemoveAdmin(member.id, member.email)}
                    className="w-full"
                    aria-label={t('admin.team.removeAriaLabel', { email: member.email })}
                  >
                    <Trash2 className="h-3 w-3 mr-1" />
                    {t('common.delete')}
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <div className="border rounded-lg">
              <Table aria-label={t('admin.team.tableAriaLabel')}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('admin.team.columns.email')}</TableHead>
                    <TableHead>{t('admin.team.columns.role')}</TableHead>
                    <TableHead>{t('admin.team.columns.addedAt')}</TableHead>
                    <TableHead>{t('admin.team.columns.actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {teamMembers.map((member) => (
                    <TableRow key={member.id}>
                      <TableCell className="font-mono text-sm">{member.email}</TableCell>
                      <TableCell>
                        <Badge>{t('admin.team.roles.admin')}</Badge>
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        {format(new Date(member.created_at), 'dd/MM/yyyy HH:mm', { locale: ptBR })}
                      </TableCell>
                      <TableCell>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleRemoveAdmin(member.id, member.email)}
                          aria-label={t('admin.team.removeAriaLabel', { email: member.email })}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
