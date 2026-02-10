import { ErrorBoundary } from '@/components/error-boundary';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { useCoupons } from '@/hooks/use-coupons';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import {
  Calendar,
  CheckCircle2,
  Copy,
  Edit,
  Plus,
  Search,
  Store,
  Tag,
  Trash2,
  XCircle,
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

type DiscountType = 'fixed' | 'percent';

/**
 * Coupons management page.
 * Allows creating, editing, and managing discount coupons.
 */
export default function Coupons() {
  const { t } = useTranslation();
  const {
    loading,
    isDialogOpen,
    editingCoupon,
    filterStore,
    filterStatus,
    searchQuery,
    formData,
    stores,
    categories,
    filteredCoupons,
    isPending,
    setFilterStore,
    setFilterStatus,
    setSearchQuery,
    setFormData,
    setIsDialogOpen,
    handleOpenDialog,
    handleCloseDialog,
    handleSubmit,
    handleDelete,
    handleToggleActive,
    handleCopyCode,
    isExpired,
  } = useCoupons();

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <Tag
                    className="w-5 h-5 xs:w-6 xs:h-6"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <h1
                    className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.coupons.title')}
                  </h1>
                  <p
                    className="text-xs xs:text-sm md:text-base tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.coupons.subtitle')}
                  </p>
                </div>
              </div>
              <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogTrigger asChild>
                  <Button onClick={() => handleOpenDialog()}>
                    <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
                    {t('pages.coupons.newCoupon')}
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
                  <DialogHeader>
                    <DialogTitle>
                      {editingCoupon
                        ? t('pages.coupons.editTitle')
                        : t('pages.coupons.createTitle')}
                    </DialogTitle>
                    <DialogDescription>
                      {editingCoupon
                        ? t('pages.coupons.editDescription')
                        : t('pages.coupons.createDescription')}
                    </DialogDescription>
                  </DialogHeader>
                  <form onSubmit={handleSubmit} className="space-y-4">
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <div className="space-y-2">
                        <Label htmlFor="code">
                          {t('pages.coupons.form.code')} <span className="text-red-500">*</span>
                        </Label>
                        <Input
                          id="code"
                          value={formData.code}
                          onChange={(e) =>
                            setFormData({
                              ...formData,
                              code: e.target.value.toUpperCase(),
                            })
                          }
                          placeholder={t('pages.coupons.form.codePlaceholder')}
                          required
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="store">
                          {t('pages.coupons.form.store')} <span className="text-red-500">*</span>
                        </Label>
                        <Select
                          value={formData.store}
                          onValueChange={(value) => setFormData({ ...formData, store: value })}
                        >
                          <SelectTrigger id="store">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {stores.map((store) => (
                              <SelectItem key={store.value} value={store.value}>
                                {store.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="title">
                        {t('common.description')} <span className="text-red-500">*</span>
                      </Label>
                      <Input
                        id="title"
                        value={formData.title}
                        onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                        placeholder={t('pages.coupons.form.titlePlaceholder')}
                        required
                      />
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="affiliate_link">
                        {t('pages.coupons.form.affiliateLink')}
                      </Label>
                      <Input
                        id="affiliate_link"
                        type="url"
                        value={formData.affiliate_link}
                        onChange={(e) =>
                          setFormData({ ...formData, affiliate_link: e.target.value })
                        }
                        placeholder={t('pages.coupons.form.affiliateLinkPlaceholder')}
                      />
                    </div>

                    <div
                      className="rounded-xl border p-4 xs:p-5 sm:p-6"
                      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                    >
                      <div className="mb-3">
                        <span className="text-base font-semibold" style={{ color: '#18181b' }}>
                          {t('pages.coupons.form.type')}
                        </span>
                      </div>
                      <div className="space-y-4">
                        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                          <div className="space-y-2">
                            <Label>{t('pages.coupons.form.type')}</Label>
                            <Select
                              value={formData.discount_type || 'fixed'}
                              onValueChange={(v) =>
                                setFormData({
                                  ...formData,
                                  discount_type: v as DiscountType,
                                })
                              }
                            >
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="fixed">
                                  {t('pages.coupons.form.typeFixed')}
                                </SelectItem>
                                <SelectItem value="percent">
                                  {t('pages.coupons.form.typePercent')}
                                </SelectItem>
                              </SelectContent>
                            </Select>
                          </div>

                          <div className="space-y-2">
                            <Label>{t('pages.coupons.form.discount')}</Label>
                            <Input
                              type="number"
                              inputMode="decimal"
                              value={formData.discount_value ?? ''}
                              onChange={(e) =>
                                setFormData({
                                  ...formData,
                                  discount_value: e.target.value ? Number(e.target.value) : null,
                                })
                              }
                              placeholder={t('pages.coupons.form.discountPlaceholder')}
                            />
                          </div>
                        </div>

                        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                          <div className="space-y-2">
                            <Label>{t('pages.coupons.form.maxDiscount')}</Label>
                            <Input
                              type="number"
                              inputMode="decimal"
                              value={formData.max_discount ?? ''}
                              onChange={(e) =>
                                setFormData({
                                  ...formData,
                                  max_discount: e.target.value ? Number(e.target.value) : null,
                                })
                              }
                              placeholder={t('pages.coupons.form.maxDiscountPlaceholder')}
                            />
                          </div>

                          <div className="space-y-2">
                            <Label>{t('pages.coupons.form.minProductPrice')}</Label>
                            <Input
                              type="number"
                              inputMode="decimal"
                              value={formData.min_product_price ?? ''}
                              onChange={(e) =>
                                setFormData({
                                  ...formData,
                                  min_product_price: e.target.value ? Number(e.target.value) : null,
                                })
                              }
                              placeholder={t('pages.coupons.form.minProductPricePlaceholder')}
                            />
                          </div>
                        </div>

                        <div className="space-y-2">
                          <Label>{t('pages.coupons.form.couponText')}</Label>
                          <Textarea
                            value={formData.coupon_text ?? ''}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                coupon_text: e.target.value,
                              })
                            }
                            rows={3}
                          />
                        </div>
                      </div>
                    </div>

                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <div className="space-y-2">
                        <Label htmlFor="expires_at">{t('pages.coupons.form.validUntil')}</Label>
                        <Input
                          id="expires_at"
                          type="datetime-local"
                          value={
                            formData.expires_at
                              ? format(new Date(formData.expires_at), "yyyy-MM-dd'T'HH:mm")
                              : ''
                          }
                          onChange={(e) =>
                            setFormData({
                              ...formData,
                              expires_at: e.target.value
                                ? new Date(e.target.value).toISOString()
                                : null,
                            })
                          }
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="category">{t('common.category')}</Label>
                        <Select
                          value={formData.category || undefined}
                          onValueChange={(value) =>
                            setFormData({
                              ...formData,
                              category: value || null,
                            })
                          }
                        >
                          <SelectTrigger id="category">
                            <SelectValue placeholder={t('common.select')} />
                          </SelectTrigger>
                          <SelectContent>
                            {categories.map((cat) => (
                              <SelectItem key={cat.value} value={cat.value}>
                                {cat.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>

                    <DialogFooter>
                      <Button type="button" variant="outline" onClick={handleCloseDialog}>
                        {t('common.cancel')}
                      </Button>
                      <Button type="submit" disabled={isPending}>
                        {editingCoupon ? t('common.update') : t('common.create')}
                      </Button>
                    </DialogFooter>
                  </form>
                </DialogContent>
              </Dialog>
            </div>

            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-4 xs:mb-5">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <Search
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
                  style={{ color: colors.wineText }}
                >
                  {t('common.filter')}
                </span>
              </div>
              <div className="space-y-4">
                <div className="relative">
                  <Search
                    className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2"
                    style={{ color: colors.wineTextMuted }}
                  />
                  <Input
                    placeholder={t('pages.coupons.searchPlaceholder')}
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="pl-9"
                  />
                </div>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t('common.store')}</Label>
                    <Select value={filterStore} onValueChange={setFilterStore}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">{t('common.all')}</SelectItem>
                        {stores.map((store) => (
                          <SelectItem key={store.value} value={store.value}>
                            {store.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>{t('common.status')}</Label>
                    <Select value={filterStatus} onValueChange={setFilterStatus}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">{t('common.all')}</SelectItem>
                        <SelectItem value="active">{t('common.active')}</SelectItem>
                        <SelectItem value="inactive">{t('common.inactive')}</SelectItem>
                        <SelectItem value="expired">{t('common.expired')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              </div>
            </div>

            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="mb-4 xs:mb-5">
                <span className="text-base font-semibold" style={{ color: '#18181b' }}>
                  {t('pages.coupons.table.title')}
                </span>
                <p className="text-sm mt-1" style={{ color: colors.wineTextMuted }}>
                  {filteredCoupons.length} {t('pages.coupons.title')}
                </p>
              </div>
              {loading ? (
                <div className="flex items-center justify-center py-12">
                  <div
                    className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
                    style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
                  />
                </div>
              ) : filteredCoupons.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12">
                  <Tag
                    className="h-12 w-12 mb-4"
                    style={{ color: colors.wineTextMuted }}
                    aria-hidden="true"
                  />
                  <p style={{ color: colors.wineTextMuted }}>{t('pages.coupons.empty')}</p>
                </div>
              ) : (
                <>
                  {/* Desktop Table */}
                  <div className="hidden md:block">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('pages.coupons.table.code')}</TableHead>
                          <TableHead>{t('common.description')}</TableHead>
                          <TableHead>{t('common.store')}</TableHead>
                          <TableHead>{t('common.category')}</TableHead>
                          <TableHead>{t('pages.coupons.table.validUntil')}</TableHead>
                          <TableHead>{t('pages.coupons.table.status')}</TableHead>
                          <TableHead className="text-right">{t('common.actions')}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {filteredCoupons.map((coupon) => (
                          <TableRow key={coupon.id}>
                            <TableCell>
                              <div className="flex items-center gap-2">
                                <code className="font-mono font-semibold">{coupon.code}</code>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleCopyCode(coupon.code)}
                                  aria-label={t('pages.coupons.actions.copyCode')}
                                >
                                  <Copy className="h-3 w-3" aria-hidden="true" />
                                </Button>
                              </div>
                            </TableCell>
                            <TableCell className="max-w-xs truncate">{coupon.title}</TableCell>
                            <TableCell>
                              <span
                                className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium border"
                                style={{ borderColor: colors.wineBorder, color: colors.wineText }}
                              >
                                <Store className="h-3 w-3" aria-hidden="true" />
                                {stores.find((s) => s.value === coupon.store)?.label ||
                                  coupon.store}
                              </span>
                            </TableCell>
                            <TableCell>
                              {coupon.category ? (
                                <span
                                  className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                                  style={{ backgroundColor: colors.iconBg, color: colors.wineText }}
                                >
                                  {categories.find((c) => c.value === coupon.category)?.label ||
                                    coupon.category}
                                </span>
                              ) : (
                                <span style={{ color: colors.wineTextMuted }}>-</span>
                              )}
                            </TableCell>
                            <TableCell>
                              {coupon.expires_at ? (
                                <div className="flex items-center gap-1">
                                  <Calendar
                                    className="h-3 w-3"
                                    style={{ color: colors.wineTextMuted }}
                                    aria-hidden="true"
                                  />
                                  <span className="text-sm">
                                    {format(new Date(coupon.expires_at), 'dd/MM/yyyy HH:mm', {
                                      locale: ptBR,
                                    })}
                                  </span>
                                </div>
                              ) : (
                                <span style={{ color: colors.wineTextMuted }}>-</span>
                              )}
                            </TableCell>
                            <TableCell>
                              {isExpired(coupon) ? (
                                <span className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium bg-red-100 text-red-700">
                                  {t('common.expired')}
                                </span>
                              ) : coupon.is_active ? (
                                <span
                                  className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium"
                                  style={{ backgroundColor: colors.iconBg, color: colors.wine }}
                                >
                                  <CheckCircle2 className="h-3 w-3" aria-hidden="true" />
                                  {t('common.active')}
                                </span>
                              ) : (
                                <span
                                  className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium"
                                  style={{
                                    backgroundColor: 'rgba(141, 45, 83, 0.06)',
                                    color: colors.wineTextLight,
                                  }}
                                >
                                  <XCircle className="h-3 w-3" aria-hidden="true" />
                                  {t('common.inactive')}
                                </span>
                              )}
                            </TableCell>
                            <TableCell className="text-right">
                              <div className="flex items-center justify-end gap-2">
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleToggleActive(coupon)}
                                  aria-label={
                                    coupon.is_active
                                      ? t('pages.coupons.actions.deactivate')
                                      : t('pages.coupons.actions.activate')
                                  }
                                >
                                  {coupon.is_active ? (
                                    <XCircle className="h-4 w-4" aria-hidden="true" />
                                  ) : (
                                    <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                                  )}
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleOpenDialog(coupon)}
                                  aria-label={t('pages.coupons.actions.edit')}
                                >
                                  <Edit className="h-4 w-4" aria-hidden="true" />
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleDelete(coupon.id)}
                                  aria-label={t('pages.coupons.actions.delete')}
                                >
                                  <Trash2 className="h-4 w-4 text-destructive" aria-hidden="true" />
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>

                  {/* Mobile Cards */}
                  <div className="md:hidden space-y-4">
                    {filteredCoupons.map((coupon) => (
                      <div
                        key={coupon.id}
                        className="rounded-xl border p-4 overflow-hidden"
                        style={{ backgroundColor: '#ffffff', borderColor: colors.wineBorder }}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-2">
                              <code className="font-mono font-semibold text-lg">{coupon.code}</code>
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => handleCopyCode(coupon.code)}
                                aria-label={t('pages.coupons.actions.copyCode')}
                              >
                                <Copy className="h-3 w-3" aria-hidden="true" />
                              </Button>
                            </div>
                            <p
                              className="text-sm truncate mb-3"
                              style={{ color: colors.wineTextMuted }}
                            >
                              {coupon.title}
                            </p>
                            <div className="flex flex-wrap gap-2 mb-3">
                              <span
                                className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium border"
                                style={{ borderColor: colors.wineBorder, color: colors.wineText }}
                              >
                                <Store className="h-3 w-3" aria-hidden="true" />
                                {stores.find((s) => s.value === coupon.store)?.label ||
                                  coupon.store}
                              </span>
                              {coupon.category && (
                                <span
                                  className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                                  style={{ backgroundColor: colors.iconBg, color: colors.wineText }}
                                >
                                  {categories.find((c) => c.value === coupon.category)?.label ||
                                    coupon.category}
                                </span>
                              )}
                              {isExpired(coupon) ? (
                                <span className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium bg-red-100 text-red-700">
                                  {t('common.expired')}
                                </span>
                              ) : coupon.is_active ? (
                                <span
                                  className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium"
                                  style={{ backgroundColor: colors.iconBg, color: colors.wine }}
                                >
                                  <CheckCircle2 className="h-3 w-3" aria-hidden="true" />
                                  {t('common.active')}
                                </span>
                              ) : (
                                <span
                                  className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium"
                                  style={{
                                    backgroundColor: 'rgba(141, 45, 83, 0.06)',
                                    color: colors.wineTextLight,
                                  }}
                                >
                                  <XCircle className="h-3 w-3" aria-hidden="true" />
                                  {t('common.inactive')}
                                </span>
                              )}
                            </div>
                            {coupon.expires_at && (
                              <div
                                className="flex items-center gap-1 text-sm"
                                style={{ color: colors.wineTextMuted }}
                              >
                                <Calendar className="h-3 w-3" aria-hidden="true" />
                                <span>
                                  {format(new Date(coupon.expires_at), 'dd/MM/yyyy HH:mm', {
                                    locale: ptBR,
                                  })}
                                </span>
                              </div>
                            )}
                          </div>
                        </div>
                        <div className="flex items-center justify-end gap-2 mt-4 pt-4 border-t">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleToggleActive(coupon)}
                          >
                            {coupon.is_active ? (
                              <>
                                <XCircle className="mr-1 h-4 w-4" aria-hidden="true" />
                                {t('pages.coupons.actions.deactivate')}
                              </>
                            ) : (
                              <>
                                <CheckCircle2 className="mr-1 h-4 w-4" aria-hidden="true" />
                                {t('pages.coupons.actions.activate')}
                              </>
                            )}
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleOpenDialog(coupon)}
                          >
                            <Edit className="mr-1 h-4 w-4" aria-hidden="true" />
                            {t('common.edit')}
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleDelete(coupon.id)}
                            className="text-destructive hover:text-destructive"
                          >
                            <Trash2 className="h-4 w-4" aria-hidden="true" />
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
