import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { formatBRL } from '@/lib/formatters';
import type { PreviewOffer } from '@/types/automation';
import { FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface OffersPreviewDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  loading: boolean;
  offers: PreviewOffer[];
}

/**
 * Dialog for previewing Shopee offers
 */
export function OffersPreviewDialog({
  open,
  onOpenChange,
  loading,
  offers,
}: OffersPreviewDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[calc(100vw-2.5rem)] xs:max-w-[calc(100vw-3rem)] sm:max-w-[calc(100vw-4rem)] md:max-w-4xl xl:max-w-5xl 2xl:max-w-6xl max-h-[85vh] xs:max-h-[90vh] overflow-y-auto p-4 xs:p-5 sm:p-6 rounded-xl xs:rounded-2xl">
        <DialogHeader>
          <DialogTitle className="text-base xs:text-lg md:text-xl">
            {t('pages.offerAutomations.preview.title')}
          </DialogTitle>
          <DialogDescription className="text-xs xs:text-sm">
            {t('pages.offerAutomations.preview.description')}
          </DialogDescription>
        </DialogHeader>
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-center">
              <div
                className="animate-spin rounded-full h-8 w-8 border-b-2 mx-auto mb-4"
                style={{ borderColor: colors.wine }}
              />
              <p style={{ color: colors.wineTextMuted }}>
                {t('pages.offerAutomations.preview.loading')}
              </p>
            </div>
          </div>
        ) : offers.length === 0 ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-center">
              <div
                className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl"
                style={{ backgroundColor: colors.iconBg }}
              >
                <FileText className="h-8 w-8" style={{ color: colors.wine }} />
              </div>
              <p style={{ color: colors.wineTextMuted }}>
                {t('pages.offerAutomations.preview.empty')}
              </p>
              <p className="text-sm mt-2" style={{ color: colors.wineTextMuted }}>
                {t('pages.offerAutomations.preview.emptyHint')}
              </p>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4 lg:gap-5 xl:gap-6 mt-4">
            {offers.map((offer, index) => (
              <div
                key={index}
                className="overflow-hidden rounded-xl border transition-shadow hover:shadow-md"
                style={{ borderColor: colors.wineBorder }}
              >
                <div className="relative">
                  {offer.imageUrl ? (
                    <img
                      src={offer.imageUrl}
                      alt={offer.title || t('pages.offerAutomations.preview.productAlt')}
                      className="w-full h-48 lg:h-52 xl:h-56 object-cover"
                      onError={(e) => {
                        (e.target as HTMLImageElement).src =
                          'https://via.placeholder.com/300x300?text=Sem+Imagem';
                      }}
                    />
                  ) : (
                    <div className="w-full h-48 lg:h-52 xl:h-56 bg-muted flex items-center justify-center">
                      <FileText className="h-12 w-12" style={{ color: colors.wineTextMuted }} />
                    </div>
                  )}
                  {offer.discountPercent != null && offer.discountPercent > 0 && (
                    <Badge className="absolute top-2 right-2 bg-red-500" variant="destructive">
                      -{offer.discountPercent}%
                    </Badge>
                  )}
                </div>
                <div className="p-4">
                  <h3 className="font-semibold text-sm mb-2 line-clamp-2">
                    {offer.title || t('pages.offerAutomations.preview.productNoName')}
                  </h3>
                  <div className="flex items-center justify-between mb-2">
                    <div>
                      <p className="text-lg font-bold" style={{ color: colors.wine }}>
                        {formatBRL(offer.finalPrice ?? 0)}
                      </p>
                      {offer.originalPrice != null &&
                        offer.finalPrice != null &&
                        offer.originalPrice > offer.finalPrice && (
                          <p
                            className="text-xs line-through"
                            style={{ color: colors.wineTextMuted }}
                          >
                            {formatBRL(offer.originalPrice)}
                          </p>
                        )}
                      {offer.commissionRate != null && offer.commissionRate > 0 && (
                        <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                          {t('pages.offerAutomations.preview.commission', {
                            value: (offer.commissionRate * 100).toFixed(0),
                          })}
                        </p>
                      )}
                    </div>
                  </div>
                  {offer.storeName && (
                    <p className="text-xs mb-1" style={{ color: colors.wineTextMuted }}>
                      {offer.storeName}
                    </p>
                  )}
                  {offer.productUrl && (
                    <a
                      href={offer.productUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs hover:underline"
                      style={{ color: colors.wine }}
                    >
                      {t('pages.offerAutomations.preview.viewProduct')}
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
