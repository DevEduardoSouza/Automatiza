import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { formatBRL } from '@/lib/formatters';
import type { PreviewOffer } from '@/types/automation';
import { FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';

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
      <DialogContent className="max-w-4xl xl:max-w-5xl 2xl:max-w-6xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('pages.offerAutomations.preview.title')}</DialogTitle>
        </DialogHeader>
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-center">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4" />
              <p className="text-muted-foreground">{t('pages.offerAutomations.preview.loading')}</p>
            </div>
          </div>
        ) : offers.length === 0 ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-center">
              <FileText className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
              <p className="text-muted-foreground">{t('pages.offerAutomations.preview.empty')}</p>
              <p className="text-sm text-muted-foreground mt-2">
                {t('pages.offerAutomations.preview.emptyHint')}
              </p>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4 lg:gap-5 xl:gap-6 mt-4">
            {offers.map((offer, index) => (
              <Card key={index} className="overflow-hidden">
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
                      <FileText className="h-12 w-12 text-muted-foreground" />
                    </div>
                  )}
                  {offer.discountPercent != null && offer.discountPercent > 0 && (
                    <Badge className="absolute top-2 right-2 bg-red-500" variant="destructive">
                      -{offer.discountPercent}%
                    </Badge>
                  )}
                </div>
                <CardContent className="p-4">
                  <h3 className="font-semibold text-sm mb-2 line-clamp-2">
                    {offer.title || t('pages.offerAutomations.preview.productNoName')}
                  </h3>
                  <div className="flex items-center justify-between mb-2">
                    <div>
                      <p className="text-lg font-bold text-primary">
                        {formatBRL(offer.finalPrice ?? 0)}
                      </p>
                      {offer.originalPrice != null &&
                        offer.finalPrice != null &&
                        offer.originalPrice > offer.finalPrice && (
                          <p className="text-xs text-muted-foreground line-through">
                            {formatBRL(offer.originalPrice)}
                          </p>
                        )}
                      {offer.commissionRate != null && offer.commissionRate > 0 && (
                        <p className="text-xs text-muted-foreground">
                          {t('pages.offerAutomations.preview.commission', {
                            value: (offer.commissionRate * 100).toFixed(0),
                          })}
                        </p>
                      )}
                    </div>
                  </div>
                  {offer.storeName && (
                    <p className="text-xs text-muted-foreground mb-1">{offer.storeName}</p>
                  )}
                  {offer.productUrl && (
                    <a
                      href={offer.productUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs text-primary hover:underline"
                    >
                      {t('pages.offerAutomations.preview.viewProduct')}
                    </a>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
