import { ErrorBoundary } from '@/components/error-boundary';
import { useRedirectLink } from '@/hooks/use-redirect-link';
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable';
import { Link2, List } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  GroupSelectionDialog,
  HowItWorksCard,
  LinkFormCard,
  PriorityMobileCards,
  PriorityTable,
  SavedLinksCard,
} from './components';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

export default function RedirectLinkPage() {
  const { t } = useTranslation();
  const {
    slug,
    linkName,
    allGroups,
    groupPriorities,
    selectedGroupIds,
    savedLinks,
    loading,
    searchTerm,
    dialogOpen,
    distributionStrategy,
    editingLink,
    setSlug,
    setLinkName,
    setSearchTerm,
    setDialogOpen,
    setDistributionStrategy,
    setGroupPriorities,
    handleDragEnd,
    toggleGroupSelection,
    applyGroupSelection,
    moveGroup,
    handleCreateOrUpdate,
    deleteLink,
    startEditLink,
    cancelEdit,
    copyToClipboard,
  } = useRedirectLink();

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  const handleClickLimitChange = (groupId: string, limit: number) => {
    setGroupPriorities((prev) =>
      prev.map((g) => (g.id === groupId ? { ...g, click_limit: limit } : g))
    );
  };

  return (
    <ErrorBoundary>
      <div className="redirect-link-page flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            {/* Header */}
            <div className="flex items-center gap-2 xs:gap-3">
              <div
                className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <Link2 className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
              </div>
              <div>
                <h1
                  className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                  style={{ color: '#18181b' }}
                >
                  {t('pages.redirectLink.title')}
                </h1>
                <p
                  className="text-xs xs:text-sm md:text-base tracking-tight"
                  style={{ color: colors.wineTextMuted }}
                >
                  {t('pages.redirectLink.subtitle')}
                </p>
              </div>
            </div>

            {/* Form + Saved Links Grid */}
            <div className="grid gap-4 xs:gap-5 sm:gap-6 lg:grid-cols-2">
              <LinkFormCard
                linkName={linkName}
                slug={slug}
                distributionStrategy={distributionStrategy}
                selectedGroupsCount={selectedGroupIds.length}
                loading={loading}
                editingLink={editingLink}
                onLinkNameChange={setLinkName}
                onSlugChange={setSlug}
                onStrategyChange={setDistributionStrategy}
                onOpenGroupDialog={() => setDialogOpen(true)}
                onSubmit={handleCreateOrUpdate}
                onCancelEdit={cancelEdit}
                isSubmitDisabled={!linkName.trim() || !slug.trim() || groupPriorities.length === 0}
              />

              <SavedLinksCard
                savedLinks={savedLinks}
                onCopy={copyToClipboard}
                onEdit={startEditLink}
                onDelete={deleteLink}
              />
            </div>

            {/* Priority Card */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <List
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <span
                    className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
                    style={{ color: colors.wineTextLight }}
                  >
                    {t('pages.redirectLink.priority.title')}
                  </span>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    <span className="hidden md:inline">
                      {t('pages.redirectLink.priority.descriptionDesktop')}
                    </span>
                    <span className="md:hidden">
                      {t('pages.redirectLink.priority.descriptionMobile')}
                    </span>
                  </p>
                </div>
              </div>

              <DndContext
                sensors={sensors}
                collisionDetection={closestCenter}
                onDragEnd={handleDragEnd}
              >
                <PriorityTable
                  groups={groupPriorities}
                  distributionStrategy={distributionStrategy}
                  onMoveUp={(index) => moveGroup(index, 'up')}
                  onMoveDown={(index) => moveGroup(index, 'down')}
                  onClickLimitChange={handleClickLimitChange}
                />

                <PriorityMobileCards
                  groups={groupPriorities}
                  distributionStrategy={distributionStrategy}
                  onMoveUp={(index) => moveGroup(index, 'up')}
                  onMoveDown={(index) => moveGroup(index, 'down')}
                  onClickLimitChange={handleClickLimitChange}
                />
              </DndContext>
            </div>

            <HowItWorksCard />

            <GroupSelectionDialog
              open={dialogOpen}
              onOpenChange={setDialogOpen}
              groups={allGroups}
              selectedIds={selectedGroupIds}
              searchTerm={searchTerm}
              onSearchChange={setSearchTerm}
              onToggleSelection={toggleGroupSelection}
              onApply={applyGroupSelection}
            />
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
