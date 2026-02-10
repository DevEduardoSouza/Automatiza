import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination';
import { useAuth } from '@/contexts';
import { useIsMobile } from '@/hooks/use-mobile';
import { Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useUsersTab } from './hooks';
import { UserCreateDialog, UserDetailsDialog, UserTable } from './users';

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
 * Tab de gerenciamento de usuários do painel admin.
 * Permite listar, criar, editar e gerenciar usuários.
 */
export default function UsersTab() {
  const { t } = useTranslation();
  const { user: authUser } = useAuth();
  const isMobile = useIsMobile();

  const {
    filteredUsers,
    setFilteredUsers,
    adminIds,
    searchTerm,
    setSearchTerm,
    statusFilter,
    setStatusFilter,
    loading,
    isDialogOpen,
    setIsDialogOpen,
    creating,
    currentPage,
    setCurrentPage,
    totalUsers,
    totalPages,
    formData,
    setFormData,
    detailsOpen,
    setDetailsOpen,
    detailsLoading,
    details,
    bulkActionLoading,
    spreadMinutes,
    setSpreadMinutes,
    blockReason,
    setBlockReason,
    blockKind,
    setBlockKind,
    lbUserLoading,
    lbUserServer,
    lbTargetServerId,
    setLbTargetServerId,
    lbMigrating,
    systemEvents,
    systemEventsLoading,
    systemEventsSinceMinutes,
    setSystemEventsSinceMinutes,
    handleWhatsAppCall,
    handleCreateUser,
    makeAdmin,
    revokeAdmin,
    updateMaxAutomations,
    openUserDetails,
    fetchSystemEvents,
    blockUserAccount,
    unblockUserAccount,
    handleResetPassword,
    pauseAllActiveAutomations,
    activatePausedAutomations,
    spreadNextRunAt,
    migrateUserServer,
    handleRefreshDetails,
  } = useUsersTab();

  const ITEMS_PER_PAGE = 10;

  return (
    <div className="space-y-4">
      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-3 xs:mb-4 sm:mb-5">
          <div>
            <span
              className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
              style={{ color: colors.wineText }}
            >
              {t('admin.users.title')}
            </span>
            <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
              {t('admin.users.description')}
            </p>
          </div>
          <UserCreateDialog
            open={isDialogOpen}
            onOpenChange={setIsDialogOpen}
            formData={formData}
            onFormDataChange={setFormData}
            onSubmit={handleCreateUser}
            creating={creating}
          />
        </div>
        <div className="space-y-3 xs:space-y-4">
          <div className="flex flex-col gap-4">
            <div className="flex-1 relative">
              <Search
                className="absolute left-3 top-3 h-4 w-4"
                style={{ color: colors.wineTextMuted }}
              />
              <Input
                placeholder={t('admin.users.searchPlaceholder')}
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-10 text-sm"
                aria-label={t('admin.users.searchPlaceholder')}
              />
            </div>
            <div
              className="flex gap-2 flex-wrap"
              role="group"
              aria-label={t('admin.users.statusFilter')}
            >
              <Button
                variant={statusFilter === 'all' ? 'default' : 'outline'}
                onClick={() => {
                  setStatusFilter('all');
                  setCurrentPage(1);
                }}
                size="sm"
                aria-pressed={statusFilter === 'all'}
              >
                {t('common.all')}
              </Button>
              <Button
                variant={statusFilter === 'active' ? 'default' : 'outline'}
                onClick={() => {
                  setStatusFilter('active');
                  setCurrentPage(1);
                }}
                size="sm"
                aria-pressed={statusFilter === 'active'}
              >
                {t('common.active')}
              </Button>
              <Button
                variant={statusFilter === 'inactive' ? 'default' : 'outline'}
                onClick={() => {
                  setStatusFilter('inactive');
                  setCurrentPage(1);
                }}
                size="sm"
                aria-pressed={statusFilter === 'inactive'}
              >
                {t('common.inactive')}
              </Button>
            </div>
          </div>

          {loading ? (
            <div className="text-center py-6 xs:py-8 text-sm" role="status" aria-live="polite">
              {t('common.loading')}
            </div>
          ) : filteredUsers.length === 0 ? (
            <div
              className="text-center py-6 xs:py-8 text-sm"
              style={{ color: colors.wineTextMuted }}
            >
              {t('admin.users.noUsers')}
            </div>
          ) : (
            <UserTable
              users={filteredUsers}
              adminIds={adminIds}
              currentUserId={authUser?.id}
              isMobile={isMobile}
              onOpenDetails={openUserDetails}
              onWhatsAppCall={handleWhatsAppCall}
              onMakeAdmin={makeAdmin}
              onRevokeAdmin={revokeAdmin}
              onUpdateMaxAutomations={updateMaxAutomations}
              onUsersChange={setFilteredUsers}
            />
          )}

          {!searchTerm && totalPages > 1 && (
            <UsersPagination
              currentPage={currentPage}
              totalPages={totalPages}
              totalUsers={totalUsers}
              itemsPerPage={ITEMS_PER_PAGE}
              onPageChange={setCurrentPage}
              t={t}
            />
          )}
        </div>
      </div>

      <UserDetailsDialog
        open={detailsOpen}
        onOpenChange={setDetailsOpen}
        loading={detailsLoading}
        details={details}
        bulkActionLoading={bulkActionLoading}
        spreadMinutes={spreadMinutes}
        onSpreadMinutesChange={setSpreadMinutes}
        blockReason={blockReason}
        onBlockReasonChange={setBlockReason}
        blockKind={blockKind}
        onBlockKindChange={setBlockKind}
        lbUserLoading={lbUserLoading}
        lbUserServer={lbUserServer}
        lbTargetServerId={lbTargetServerId}
        onLbTargetServerIdChange={setLbTargetServerId}
        lbMigrating={lbMigrating}
        systemEvents={systemEvents}
        systemEventsLoading={systemEventsLoading}
        systemEventsSinceMinutes={systemEventsSinceMinutes}
        onSystemEventsSinceMinutesChange={setSystemEventsSinceMinutes}
        onBlockUser={blockUserAccount}
        onUnblockUser={unblockUserAccount}
        onResetPassword={handleResetPassword}
        onPauseAllAutomations={pauseAllActiveAutomations}
        onActivatePausedAutomations={activatePausedAutomations}
        onSpreadNextRunAt={spreadNextRunAt}
        onMigrateServer={migrateUserServer}
        onFetchSystemEvents={fetchSystemEvents}
        onRefreshDetails={handleRefreshDetails}
      />
    </div>
  );
}

interface UsersPaginationProps {
  currentPage: number;
  totalPages: number;
  totalUsers: number;
  itemsPerPage: number;
  onPageChange: (page: number) => void;
  t: (key: string, options?: Record<string, unknown>) => string;
}

function UsersPagination({
  currentPage,
  totalPages,
  totalUsers,
  itemsPerPage,
  onPageChange,
  t,
}: UsersPaginationProps) {
  return (
    <div className="flex flex-col sm:flex-row items-center justify-between gap-3 xs:gap-4 pt-3 xs:pt-4">
      <p className="text-xs md:text-sm" style={{ color: colors.wineTextMuted }}>
        {t('admin.users.pagination.showing', {
          from: (currentPage - 1) * itemsPerPage + 1,
          to: Math.min(currentPage * itemsPerPage, totalUsers),
          total: totalUsers,
        })}
      </p>
      <Pagination>
        <PaginationContent>
          <PaginationItem>
            <PaginationPrevious
              onClick={() => onPageChange(Math.max(1, currentPage - 1))}
              className={currentPage === 1 ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
              aria-disabled={currentPage === 1}
            />
          </PaginationItem>

          {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
            let pageNum: number;
            if (totalPages <= 5) {
              pageNum = i + 1;
            } else if (currentPage <= 3) {
              pageNum = i + 1;
            } else if (currentPage >= totalPages - 2) {
              pageNum = totalPages - 4 + i;
            } else {
              pageNum = currentPage - 2 + i;
            }

            return (
              <PaginationItem key={pageNum}>
                <PaginationLink
                  onClick={() => onPageChange(pageNum)}
                  isActive={currentPage === pageNum}
                  className="cursor-pointer"
                  aria-current={currentPage === pageNum ? 'page' : undefined}
                >
                  {pageNum}
                </PaginationLink>
              </PaginationItem>
            );
          })}

          <PaginationItem>
            <PaginationNext
              onClick={() => onPageChange(Math.min(totalPages, currentPage + 1))}
              className={
                currentPage === totalPages ? 'pointer-events-none opacity-50' : 'cursor-pointer'
              }
              aria-disabled={currentPage === totalPages}
            />
          </PaginationItem>
        </PaginationContent>
      </Pagination>
    </div>
  );
}
