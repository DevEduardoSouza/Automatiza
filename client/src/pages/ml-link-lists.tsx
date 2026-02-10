import { ErrorBoundary } from '@/components/error-boundary';
import { ItemsVirtualTable } from '@/components/ml-link-lists';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useMlLinkLists } from '@/hooks/use-ml-link-lists';
import { formatBRL } from '@/lib/formatters';
import {
  AlertCircle,
  Check,
  Copy,
  Edit,
  Link as LinkIcon,
  List,
  Plus,
  RefreshCcw,
  Trash2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

/**
 * ML Link Lists page component.
 * Manages Mercado Livre link lists for automation use.
 */
export default function MlLinkLists() {
  const { t } = useTranslation();
  const {
    loadingLists,
    loadingItems,
    loadingMoreItems,
    lists,
    selectedListId,
    items,
    listDialogOpen,
    editingList,
    listForm,
    singleUrl,
    bulkText,
    importing,
    lastPreview,
    selectedList,
    availableCount,
    totalItemCount,
    hasMoreItems,
    deleteListTarget,
    deleteItemTarget,
    resetConsumedDialogOpen,
    setSelectedListId,
    setListDialogOpen,
    setListForm,
    setSingleUrl,
    setBulkText,
    loadLists,
    loadMoreItems,
    openCreateList,
    openEditList,
    saveList,
    requestDeleteList,
    cancelDeleteList,
    confirmDeleteList,
    requestDeleteItem,
    cancelDeleteItem,
    confirmDeleteItem,
    requestResetConsumed,
    cancelResetConsumed,
    confirmResetConsumed,
    copyToClipboard,
    addOneLink,
    importBulk,
    toggleItemActive,
    moveItem,
  } = useMlLinkLists();

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            {/* Header */}
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <LinkIcon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
                </div>
                <div>
                  <h1
                    className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.mlLinkLists.title')}
                  </h1>
                  <p
                    className="text-xs xs:text-sm md:text-base tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.mlLinkLists.subtitle')}
                  </p>
                </div>
              </div>

              <Dialog open={listDialogOpen} onOpenChange={setListDialogOpen}>
                <DialogTrigger asChild>
                  <Button
                    onClick={openCreateList}
                    className="bg-[#8D2D53] text-white hover:bg-[#7a2747]"
                  >
                    <Plus className="h-4 w-4 mr-2" aria-hidden="true" />
                    {t('pages.mlLinkLists.newList')}
                  </Button>
                </DialogTrigger>
                <DialogContent className="w-[95vw] max-w-2xl">
                  <DialogHeader>
                    <DialogTitle>
                      {editingList
                        ? t('pages.mlLinkLists.editTitle')
                        : t('pages.mlLinkLists.createTitle')}
                    </DialogTitle>
                  </DialogHeader>

                  <div className="space-y-4">
                    <div className="space-y-2">
                      <Label htmlFor="list-name">{t('pages.mlLinkLists.form.name')}</Label>
                      <Input
                        id="list-name"
                        value={listForm.name}
                        onChange={(e) => setListForm((p) => ({ ...p, name: e.target.value }))}
                        placeholder={t('pages.mlLinkLists.form.namePlaceholder')}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="list-description">
                        {t('pages.mlLinkLists.form.description')}
                      </Label>
                      <Textarea
                        id="list-description"
                        value={listForm.description}
                        onChange={(e) =>
                          setListForm((p) => ({
                            ...p,
                            description: e.target.value,
                          }))
                        }
                        placeholder={t('pages.mlLinkLists.form.descriptionPlaceholder')}
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <div className="space-y-1">
                        <div className="font-medium">{t('pages.mlLinkLists.form.active')}</div>
                        <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                          {t('pages.mlLinkLists.form.activeHint')}
                        </div>
                      </div>
                      <Switch
                        checked={listForm.is_active}
                        onCheckedChange={(v) =>
                          setListForm((p) => ({ ...p, is_active: Boolean(v) }))
                        }
                      />
                    </div>
                  </div>

                  <DialogFooter>
                    <Button variant="outline" onClick={() => setListDialogOpen(false)}>
                      {t('common.cancel')}
                    </Button>
                    <Button onClick={saveList}>
                      <Check className="h-4 w-4 mr-2" aria-hidden="true" />
                      {t('common.save')}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>

            {/* Info Alert - Brown Glassmorphism */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              role="alert"
              style={{
                backgroundColor: 'rgba(180, 120, 60, 0.08)',
                borderColor: 'rgba(180, 120, 60, 0.25)',
                backdropFilter: 'blur(12px) saturate(180%)',
                WebkitBackdropFilter: 'blur(12px) saturate(180%)',
              }}
            >
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg"
                  style={{ backgroundColor: 'rgba(180, 120, 60, 0.12)' }}
                >
                  <AlertCircle
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: 'rgb(160, 100, 40)' }}
                    aria-hidden="true"
                  />
                </div>
                <p
                  className="text-xs xs:text-sm leading-relaxed"
                  style={{ color: 'rgba(120, 75, 30, 0.8)' }}
                >
                  {t('pages.mlLinkLists.alert')}
                </p>
              </div>
            </div>

            {/* Lists Card */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between mb-4 xs:mb-5">
                <div className="flex items-center gap-2 xs:gap-3">
                  <div
                    className="p-1.5 xs:p-2 rounded-lg"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <List className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
                  </div>
                  <span
                    className="text-xs xs:text-sm font-semibold uppercase tracking-widest"
                    style={{ color: colors.wineText }}
                  >
                    {t('pages.mlLinkLists.lists.title')}
                  </span>
                </div>
                <Button variant="outline" onClick={loadLists} disabled={loadingLists}>
                  <RefreshCcw className="h-4 w-4 mr-2" aria-hidden="true" />
                  {t('pages.mlLinkLists.lists.refresh')}
                </Button>
              </div>

              <div className="space-y-4">
                {loadingLists ? (
                  <div className="flex items-center justify-center py-8">
                    <div
                      className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
                      style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
                    />
                  </div>
                ) : lists.length === 0 ? (
                  <p className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
                    {t('pages.mlLinkLists.lists.empty')}
                  </p>
                ) : (
                  <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                    <div className="w-full md:max-w-md">
                      <Label htmlFor="list-select">{t('pages.mlLinkLists.lists.select')}</Label>
                      <Select value={selectedListId} onValueChange={setSelectedListId}>
                        <SelectTrigger id="list-select" className="mt-2">
                          <SelectValue
                            placeholder={t('pages.mlLinkLists.lists.selectPlaceholder')}
                          />
                        </SelectTrigger>
                        <SelectContent>
                          {lists.map((l) => (
                            <SelectItem key={l.id} value={l.id}>
                              {l.name} {l.is_active ? '' : t('pages.mlLinkLists.lists.inactive')}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    {selectedList && (
                      <div className="flex items-center gap-2">
                        <Button variant="outline" onClick={() => openEditList(selectedList)}>
                          <Edit className="h-4 w-4 mr-2" aria-hidden="true" />
                          {t('pages.mlLinkLists.lists.edit')}
                        </Button>
                        <Button
                          variant="destructive"
                          onClick={() => requestDeleteList(selectedList)}
                        >
                          <Trash2 className="h-4 w-4 mr-2" aria-hidden="true" />
                          {t('pages.mlLinkLists.lists.delete')}
                        </Button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>

            {/* Items Card */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between mb-4 xs:mb-5">
                <div>
                  <div className="flex items-center gap-2 xs:gap-3">
                    <div
                      className="p-1.5 xs:p-2 rounded-lg"
                      style={{ backgroundColor: colors.iconBg }}
                    >
                      <LinkIcon className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
                    </div>
                    <span
                      className="text-xs xs:text-sm font-semibold uppercase tracking-widest"
                      style={{ color: colors.wineText }}
                    >
                      {t('pages.mlLinkLists.items.title')}
                    </span>
                  </div>
                  {selectedList && (
                    <div className="text-sm mt-1" style={{ color: colors.wineTextMuted }}>
                      {selectedList.description || t('pages.mlLinkLists.items.noDescription')}
                    </div>
                  )}
                </div>
                <Button variant="outline" onClick={requestResetConsumed} disabled={!selectedListId}>
                  <RefreshCcw className="h-4 w-4 mr-2" aria-hidden="true" />
                  {t('pages.mlLinkLists.items.resetSent')}
                </Button>
              </div>

              <div className="space-y-6">
                {!selectedListId ? (
                  <p className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
                    {t('pages.mlLinkLists.items.selectList')}
                  </p>
                ) : (
                  <>
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                      <div className="space-y-3">
                        <div className="flex items-center gap-2">
                          <LinkIcon className="h-4 w-4" aria-hidden="true" />
                          <div className="font-medium">{t('pages.mlLinkLists.items.addLink')}</div>
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="single-url">
                            {t('pages.mlLinkLists.items.linkLabel')}
                          </Label>
                          <Input
                            id="single-url"
                            value={singleUrl}
                            onChange={(e) => setSingleUrl(e.target.value)}
                            placeholder={t('pages.mlLinkLists.items.linkPlaceholder')}
                          />
                          <div className="flex gap-2">
                            <Button onClick={addOneLink} disabled={!singleUrl.trim()}>
                              <Plus className="h-4 w-4 mr-2" aria-hidden="true" />
                              {t('pages.mlLinkLists.items.add')}
                            </Button>
                            <Button
                              variant="outline"
                              onClick={() => setSingleUrl('')}
                              disabled={!singleUrl.trim()}
                            >
                              {t('pages.mlLinkLists.items.clear')}
                            </Button>
                          </div>
                          <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                            {t('pages.mlLinkLists.items.fragmentHint')}
                          </div>
                        </div>

                        {lastPreview && (
                          <div
                            className="rounded-lg border p-3 space-y-2"
                            style={{ borderColor: colors.wineBorder }}
                          >
                            <div className="text-sm font-medium">
                              {t('pages.mlLinkLists.items.preview')}
                            </div>
                            <div className="flex gap-3">
                              {lastPreview.thumbnail ? (
                                <img
                                  src={lastPreview.thumbnail}
                                  alt=""
                                  className="h-16 w-16 rounded object-cover"
                                />
                              ) : (
                                <div
                                  className="h-16 w-16 rounded"
                                  style={{ backgroundColor: colors.iconBg }}
                                />
                              )}
                              <div className="flex-1">
                                <div className="text-sm line-clamp-2">
                                  {lastPreview.title || t('pages.mlLinkLists.items.noTitle')}
                                </div>
                                <div className="text-sm font-semibold">
                                  {lastPreview.price != null && lastPreview.price > 0
                                    ? formatBRL(lastPreview.price)
                                    : ''}
                                </div>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  className="px-0"
                                  onClick={() => copyToClipboard(lastPreview.url)}
                                >
                                  <Copy className="h-4 w-4 mr-2" aria-hidden="true" />
                                  {t('pages.mlLinkLists.items.copyUrl')}
                                </Button>
                              </div>
                            </div>
                          </div>
                        )}
                      </div>

                      <div className="space-y-3">
                        <div className="flex items-center gap-2">
                          <LinkIcon className="h-4 w-4" aria-hidden="true" />
                          <div className="font-medium">
                            {t('pages.mlLinkLists.items.bulkImport')}
                          </div>
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="bulk-text">
                            {t('pages.mlLinkLists.items.onePerLine')}
                          </Label>
                          <Textarea
                            id="bulk-text"
                            value={bulkText}
                            onChange={(e) => setBulkText(e.target.value)}
                            placeholder={t('pages.mlLinkLists.items.bulkPlaceholder')}
                          />
                          <div className="flex gap-2">
                            <Button onClick={importBulk} disabled={importing || !bulkText.trim()}>
                              <Plus className="h-4 w-4 mr-2" aria-hidden="true" />
                              {t('pages.mlLinkLists.items.import')}
                            </Button>
                            <Button
                              variant="outline"
                              onClick={() => setBulkText('')}
                              disabled={importing || !bulkText.trim()}
                            >
                              {t('pages.mlLinkLists.items.clear')}
                            </Button>
                          </div>
                          <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                            {t('pages.mlLinkLists.items.bulkHint')}
                          </div>
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center justify-between">
                      <div className="text-sm" style={{ color: colors.wineTextMuted }}>
                        {t('pages.mlLinkLists.items.total')}{' '}
                        <b style={{ color: colors.wine }}>
                          {totalItemCount > 0 ? totalItemCount : items.length}
                        </b>
                        {totalItemCount > items.length && (
                          <span className="text-xs ml-1">
                            ({t('pages.mlLinkLists.items.loaded', { count: items.length })})
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <span
                          className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                          style={{ backgroundColor: colors.iconBg, color: colors.wine }}
                        >
                          {t('pages.mlLinkLists.items.available')} {availableCount}
                        </span>
                        <span
                          className="inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium"
                          style={{ borderColor: colors.wineBorder, color: colors.wineTextLight }}
                        >
                          {t('pages.mlLinkLists.items.sent')}{' '}
                          {(items || []).filter((i) => Boolean(i.consumed_at)).length}
                        </span>
                      </div>
                    </div>

                    {!loadingItems && items.length > 0 && availableCount === 0 && (
                      <div
                        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
                        role="alert"
                        style={{
                          backgroundColor: 'rgba(180, 120, 60, 0.08)',
                          borderColor: 'rgba(180, 120, 60, 0.25)',
                          backdropFilter: 'blur(12px) saturate(180%)',
                          WebkitBackdropFilter: 'blur(12px) saturate(180%)',
                        }}
                      >
                        <div className="flex items-center gap-2 xs:gap-3">
                          <div
                            className="p-1.5 xs:p-2 rounded-lg"
                            style={{ backgroundColor: 'rgba(180, 120, 60, 0.12)' }}
                          >
                            <AlertCircle
                              className="w-4 h-4 xs:w-5 xs:h-5"
                              style={{ color: 'rgb(160, 100, 40)' }}
                              aria-hidden="true"
                            />
                          </div>
                          <p
                            className="text-xs xs:text-sm leading-relaxed"
                            style={{ color: 'rgba(120, 75, 30, 0.8)' }}
                          >
                            {t('pages.mlLinkLists.items.allSentAlert')}
                          </p>
                        </div>
                      </div>
                    )}

                    {loadingItems ? (
                      <div className="flex items-center justify-center py-8">
                        <div
                          className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
                          style={{
                            borderColor: `${colors.wine} transparent transparent transparent`,
                          }}
                        />
                      </div>
                    ) : items.length === 0 ? (
                      <p
                        className="text-center py-8 text-sm"
                        style={{ color: colors.wineTextMuted }}
                      >
                        {t('pages.mlLinkLists.items.empty')}
                      </p>
                    ) : (
                      <ItemsVirtualTable
                        items={items}
                        onMoveUp={(item) => moveItem(item, 'up')}
                        onMoveDown={(item) => moveItem(item, 'down')}
                        onToggleActive={toggleItemActive}
                        onRemove={requestDeleteItem}
                        onCopyLink={copyToClipboard}
                        onLoadMore={loadMoreItems}
                        hasMore={hasMoreItems}
                        isLoadingMore={loadingMoreItems}
                      />
                    )}
                  </>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      <AlertDialog
        open={deleteListTarget !== null}
        onOpenChange={(open) => !open && cancelDeleteList()}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('pages.mlLinkLists.lists.confirmDeleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('pages.mlLinkLists.lists.confirmDeleteMessage', {
                name: deleteListTarget?.name ?? '',
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDeleteList}>{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={deleteItemTarget !== null}
        onOpenChange={(open) => !open && cancelDeleteItem()}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('pages.mlLinkLists.confirm.removeLinkTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('pages.mlLinkLists.confirm.removeLinkMessage')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDeleteItem}>{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={resetConsumedDialogOpen}
        onOpenChange={(open) => !open && cancelResetConsumed()}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('pages.mlLinkLists.confirm.resetSentTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('pages.mlLinkLists.confirm.resetSentMessage')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmResetConsumed}>
              {t('common.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ErrorBoundary>
  );
}
