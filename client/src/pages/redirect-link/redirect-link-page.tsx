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
import { List } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  GroupSelectionDialog,
  HowItWorksCard,
  LinkFormCard,
  PriorityMobileCards,
  PriorityTable,
  SavedLinksCard,
} from './components';

const RedirectIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    className={className}
    style={style}
  >
    <path d="M10 13.229C10.1416 13.4609 10.3097 13.6804 10.5042 13.8828C11.7117 15.1395 13.5522 15.336 14.9576 14.4722C15.218 14.3121 15.4634 14.1157 15.6872 13.8828L18.9266 10.5114C20.3578 9.02184 20.3578 6.60676 18.9266 5.11718C17.4953 3.6276 15.1748 3.62761 13.7435 5.11718L13.03 5.85978" />
    <path d="M10.9703 18.14L10.2565 18.8828C8.82526 20.3724 6.50471 20.3724 5.07345 18.8828C3.64218 17.3932 3.64218 14.9782 5.07345 13.4886L8.31287 10.1172C9.74413 8.62761 12.0647 8.6276 13.4959 10.1172C13.6904 10.3195 13.8584 10.539 14 10.7708" />
  </svg>
);
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
    selectAllGroups,
    deselectAllGroups,
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
      <div className="redirect-link-page flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          {/* Header */}
          <div className="flex items-center gap-2 xs:gap-3 mb-5 xs:mb-6 sm:mb-8 lg:mb-10">
            <div
              className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
              style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
            >
              <RedirectIcon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
            </div>
            <div>
              <h2
                className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                style={{ color: '#18181b' }}
              >
                {t('pages.redirectLink.title')}
              </h2>
              <p
                className="text-xs xs:text-sm tracking-tight"
                style={{ color: colors.wineTextMuted }}
              >
                {t('pages.redirectLink.subtitle')}
              </p>
            </div>
          </div>

          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            {/* Form + Saved Links Grid */}
            <div className="grid gap-4 xs:gap-5 sm:gap-6 lg:grid-cols-[1.2fr_1fr]">
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

            {/* Priority Card - only shown when groups are selected */}
            {groupPriorities.length > 0 && (
              <>
                <div className="h-px" style={{ backgroundColor: colors.wineBorder }} />

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
              </>
            )}

            <div className="h-px" style={{ backgroundColor: colors.wineBorder }} />

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
              onSelectAll={selectAllGroups}
              onDeselectAll={deselectAllGroups}
            />
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
