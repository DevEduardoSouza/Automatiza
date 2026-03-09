import { ErrorBoundary } from '@/components/error-boundary';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
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
import { useCoupons } from '@/hooks/use-coupons';
import {
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

const CouponsIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    style={style}
    role="img"
    aria-hidden="true"
  >
    <title>Coupons</title>
    <path d="M10.9961 10H11.0111M10.9998 16H11.0148" />
    <path d="M7 13H15" />
    <circle cx="1.5" cy="1.5" r="1.5" transform="matrix(1 0 0 -1 16 8)" />
    <path d="M2.77423 11.1439C1.77108 12.2643 1.7495 13.9546 2.67016 15.1437C4.49711 17.5033 6.49674 19.5029 8.85633 21.3298C10.0454 22.2505 11.7357 22.2289 12.8561 21.2258C15.8979 18.5022 18.6835 15.6559 21.3719 12.5279C21.6377 12.2187 21.8039 11.8397 21.8412 11.4336C22.0062 9.63798 22.3452 4.46467 20.9403 3.05974C19.5353 1.65481 14.362 1.99377 12.5664 2.15876C12.1603 2.19608 11.7813 2.36233 11.472 2.62811C8.34412 5.31646 5.49781 8.10211 2.77423 11.1439Z" />
  </svg>
);

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

const inputClasses =
  'w-full rounded-2xl !border-2 !border-gray-300 bg-gray-50 px-4 py-2 text-xs xs:py-2.5 xs:text-sm lg:py-3 lg:text-base text-gray-900 placeholder-gray-400 outline-none transition-all hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30';

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
      <div className="coupons-page flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-10">
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <CouponsIcon
                    className="w-5 h-5 xs:w-6 xs:h-6"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <h2
                    className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.coupons.title')}
                  </h2>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.coupons.subtitle')}
                  </p>
                </div>
              </div>
              <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogTrigger asChild>
                  <button
                    type="button"
                    onClick={() => handleOpenDialog()}
                    className="auth-btn-glow relative w-full xs:w-auto xs:flex-1 md:flex-none inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all"
                  >
                    <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                      <div className="auth-btn-glow__glow absolute inset-0" />
                    </div>
                    <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                      <Plus className="h-4 w-4" aria-hidden="true" />
                      <span className="hidden sm:inline">{t('pages.coupons.newCoupon')}</span>
                      <span className="sm:hidden">{t('pages.coupons.newCoupon')}</span>
                    </span>
                  </button>
                </DialogTrigger>
                <DialogContent className="max-w-[calc(100vw-2.5rem)] xs:max-w-[calc(100vw-3rem)] sm:max-w-[calc(100vw-4rem)] md:max-w-2xl xl:max-w-3xl 2xl:max-w-4xl max-h-[85vh] xs:max-h-[90vh] overflow-y-auto p-4 xs:p-5 sm:p-6 rounded-xl xs:rounded-2xl">
                  <DialogHeader>
                    <DialogTitle className="text-base xs:text-lg md:text-xl">
                      {editingCoupon
                        ? t('pages.coupons.editTitle')
                        : t('pages.coupons.createTitle')}
                    </DialogTitle>
                    <DialogDescription className="text-xs xs:text-sm">
                      {editingCoupon
                        ? t('pages.coupons.editDescription')
                        : t('pages.coupons.createDescription')}
                    </DialogDescription>
                  </DialogHeader>
                  <form onSubmit={handleSubmit} className="space-y-4 lg:space-y-5 xl:space-y-6">
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:gap-6">
                      <div className="space-y-2">
                        <Label htmlFor="code">
                          {t('pages.coupons.form.code')} <span className="text-red-500">*</span>
                        </Label>
                        <input
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
                          className={inputClasses}
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
                              <SelectItem key={store.value} value={store.value} disabled={store.disabled}>
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
                      <input
                        id="title"
                        value={formData.title}
                        onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                        placeholder={t('pages.coupons.form.titlePlaceholder')}
                        required
                        className={inputClasses}
                      />
                    </div>

                    <div
                      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
                      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                    >
                      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                        <div
                          className="p-1.5 xs:p-2 rounded-lg"
                          style={{ backgroundColor: colors.iconBg }}
                        >
                          <Tag
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
                            {t('pages.coupons.form.type')}
                          </span>
                          <p
                            className="text-xs xs:text-sm tracking-tight"
                            style={{ color: colors.wineTextMuted }}
                          >
                            {t('pages.coupons.form.discountDescription')}
                          </p>
                        </div>
                      </div>
                      <div className="space-y-4">
                        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:gap-6">
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
                            <input
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
                              className={inputClasses}
                            />
                          </div>
                        </div>

                        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:gap-6">
                          <div className="space-y-2">
                            <Label>{t('pages.coupons.form.maxDiscount')}</Label>
                            <input
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
                              className={inputClasses}
                            />
                          </div>

                          <div className="space-y-2">
                            <Label>{t('pages.coupons.form.minProductPrice')}</Label>
                            <input
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
                              className={inputClasses}
                            />
                          </div>
                        </div>

                        <div className="space-y-2">
                          <Label>{t('pages.coupons.form.couponText')}</Label>
                          <textarea
                            value={formData.coupon_text ?? ''}
                            onChange={(e) =>
                              setFormData({
                                ...formData,
                                coupon_text: e.target.value,
                              })
                            }
                            rows={3}
                            className={`${inputClasses} resize-none`}
                          />
                        </div>
                      </div>
                    </div>

                    <div className="flex flex-col-reverse xs:flex-row justify-end gap-2 lg:gap-3 xl:gap-4">
                      <button
                        type="button"
                        onClick={handleCloseDialog}
                        className="inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
                      >
                        {t('common.cancel')}
                      </button>
                      <button
                        type="submit"
                        disabled={isPending}
                        className="auth-btn-glow relative inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:pointer-events-none disabled:opacity-50"
                      >
                        <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                          <div className="auth-btn-glow__glow absolute inset-0" />
                        </div>
                        <span className="relative z-[2] font-semibold select-none">
                          {editingCoupon ? t('common.update') : t('common.create')}
                        </span>
                      </button>
                    </div>
                  </form>
                </DialogContent>
              </Dialog>
            </div>

            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <Search
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
                    {t('common.filter')}
                  </span>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.coupons.filterDescription')}
                  </p>
                </div>
              </div>
              <div className="space-y-4">
                <div className="relative">
                  <Search
                    className="absolute left-3 xs:left-4 top-1/2 h-4 w-4 -translate-y-1/2"
                    style={{ color: colors.wineTextMuted }}
                  />
                  <input
                    placeholder={t('pages.coupons.searchPlaceholder')}
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className={`${inputClasses} pl-9 xs:pl-11`}
                  />
                </div>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:gap-6">
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
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <Tag
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
                    {t('pages.coupons.table.title')}
                  </span>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {filteredCoupons.length} {t('pages.coupons.title')}
                  </p>
                </div>
              </div>
              {loading ? (
                <div className="flex items-center justify-center py-12">
                  <div className="text-center space-y-4">
                    <div
                      className="h-8 w-8 xs:h-10 xs:w-10 sm:h-12 sm:w-12 animate-spin rounded-full border-[3px] xs:border-4 border-t-transparent mx-auto"
                      style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
                    />
                    <p className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
                      {t('pages.coupons.loading')}
                    </p>
                  </div>
                </div>
              ) : filteredCoupons.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12 lg:py-16 xl:py-20">
                  <Tag
                    className="h-12 w-12 lg:h-14 lg:w-14 mb-4 opacity-20"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                  <p className="text-sm md:text-base" style={{ color: colors.wineTextMuted }}>
                    {t('pages.coupons.empty')}
                  </p>
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
                                <button
                                  type="button"
                                  onClick={() => handleCopyCode(coupon.code)}
                                  aria-label={t('pages.coupons.actions.copyCode')}
                                  className="cursor-pointer rounded-full p-1.5 transition-colors hover:bg-accent"
                                >
                                  <Copy className="h-3 w-3" aria-hidden="true" />
                                </button>
                              </div>
                            </TableCell>
                            <TableCell className="max-w-xs xl:max-w-sm 2xl:max-w-md truncate">
                              {coupon.title}
                            </TableCell>
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
                              <div className="flex items-center justify-end gap-1">
                                <button
                                  type="button"
                                  onClick={() => handleToggleActive(coupon)}
                                  aria-label={
                                    coupon.is_active
                                      ? t('pages.coupons.actions.deactivate')
                                      : t('pages.coupons.actions.activate')
                                  }
                                  className="cursor-pointer rounded-full p-1.5 transition-colors hover:bg-accent"
                                >
                                  {coupon.is_active ? (
                                    <XCircle className="h-4 w-4" aria-hidden="true" />
                                  ) : (
                                    <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                                  )}
                                </button>
                                <button
                                  type="button"
                                  onClick={() => handleOpenDialog(coupon)}
                                  aria-label={t('pages.coupons.actions.edit')}
                                  className="cursor-pointer rounded-full p-1.5 transition-colors hover:bg-accent"
                                >
                                  <Edit className="h-4 w-4" aria-hidden="true" />
                                </button>
                                <button
                                  type="button"
                                  onClick={() => handleDelete(coupon.id)}
                                  aria-label={t('pages.coupons.actions.delete')}
                                  className="cursor-pointer rounded-full p-1.5 text-destructive transition-colors hover:bg-destructive/10"
                                >
                                  <Trash2 className="h-4 w-4" aria-hidden="true" />
                                </button>
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
                        className="rounded-xl border p-4 xs:p-5 overflow-hidden"
                        style={{ backgroundColor: '#ffffff', borderColor: colors.wineBorder }}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-2">
                              <code className="font-mono font-semibold text-lg xs:text-xl">
                                {coupon.code}
                              </code>
                              <button
                                type="button"
                                onClick={() => handleCopyCode(coupon.code)}
                                aria-label={t('pages.coupons.actions.copyCode')}
                                className="cursor-pointer rounded-full p-1.5 transition-colors hover:bg-accent"
                              >
                                <Copy className="h-3 w-3" aria-hidden="true" />
                              </button>
                            </div>
                            <p
                              className="text-sm xs:text-base truncate mb-3"
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
                          </div>
                        </div>
                        <div className="flex items-center justify-end gap-2 xs:gap-3 mt-4 pt-4 border-t">
                          <button
                            type="button"
                            onClick={() => handleToggleActive(coupon)}
                            className="flex-1 min-w-0 inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
                          >
                            {coupon.is_active ? (
                              <>
                                <XCircle className="h-4 w-4" aria-hidden="true" />
                                {t('pages.coupons.actions.deactivate')}
                              </>
                            ) : (
                              <>
                                <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                                {t('pages.coupons.actions.activate')}
                              </>
                            )}
                          </button>
                          <button
                            type="button"
                            onClick={() => handleOpenDialog(coupon)}
                            className="flex-1 min-w-0 inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
                          >
                            <Edit className="h-4 w-4" aria-hidden="true" />
                            {t('common.edit')}
                          </button>
                          <button
                            type="button"
                            onClick={() => handleDelete(coupon.id)}
                            className="inline-flex cursor-pointer items-center justify-center rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-red-300 bg-white text-destructive shadow-sm hover:-translate-y-0.5 hover:border-red-400 hover:shadow-md"
                          >
                            <Trash2 className="h-4 w-4" aria-hidden="true" />
                          </button>
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
