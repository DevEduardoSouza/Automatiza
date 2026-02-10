import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface MismatchSample {
  dispatchedAt: string;
  userId: string;
  automationName: string;
  urlId: string;
  expectedId: string;
}

interface ShopeeAuditData {
  generatedAt: string;
  daysBack: number;
  scannedLinks: number;
  linksWithIdentifier: number;
  mismatchCount: number;
  noQueryCount: number;
  noShopeeCredsCount: number;
  mismatchSamples: MismatchSample[];
}

interface ShopeeAuditCardProps {
  data: ShopeeAuditData | null;
  loading: boolean;
  days: string;
  onDaysChange: (days: string) => void;
  onRefresh: () => void;
}

/**
 * Card de auditoria Shopee do dashboard.
 * Verifica inconsistências de afiliado vs appID do usuário.
 */
export function ShopeeAuditCard({
  data,
  loading,
  days,
  onDaysChange,
  onRefresh,
}: ShopeeAuditCardProps) {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between mb-3 xs:mb-4">
        <div>
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('admin.dashboard.shopeeAudit.title')}
          </span>
          <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.shopeeAudit.description')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select value={days} onValueChange={onDaysChange}>
            <SelectTrigger className="w-[80px]">
              <SelectValue placeholder={t('admin.dashboard.shopeeAudit.days')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="1">1</SelectItem>
              <SelectItem value="3">3</SelectItem>
              <SelectItem value="7">7</SelectItem>
              <SelectItem value="14">14</SelectItem>
            </SelectContent>
          </Select>
          <span className="text-sm" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.shopeeAudit.days')}
          </span>
          <Button variant="outline" onClick={onRefresh} disabled={loading}>
            {loading ? (
              <span className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t('admin.dashboard.loadBalancer.refreshing')}
              </span>
            ) : (
              t('admin.dashboard.loadBalancer.refresh')
            )}
          </Button>
        </div>
      </div>
      <div className="space-y-3 xs:space-y-4">
        {data ? (
          <div className="space-y-6">
            {/* Summary badges */}
            <div className="flex flex-wrap gap-2">
              <Badge variant="outline">
                {t('admin.dashboard.shopeeAudit.scanned')}: {data.scannedLinks}
              </Badge>
              <Badge variant="secondary">
                {t('admin.dashboard.shopeeAudit.withIdentifier')}: {data.linksWithIdentifier}
              </Badge>
              <Badge variant={data.mismatchCount > 0 ? 'destructive' : 'default'}>
                {t('admin.dashboard.shopeeAudit.mismatch')}: {data.mismatchCount}
              </Badge>
              <Badge variant="outline">
                {t('admin.dashboard.shopeeAudit.noQuery')}: {data.noQueryCount}
              </Badge>
              <Badge variant="outline">
                {t('admin.dashboard.shopeeAudit.noShopeeCredentials')}: {data.noShopeeCredsCount}
              </Badge>
            </div>

            {/* Samples table */}
            {data.mismatchSamples.length > 0 ? (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('admin.dashboard.shopeeAudit.date')}</TableHead>
                      <TableHead>{t('admin.dashboard.shopeeAudit.user')}</TableHead>
                      <TableHead>{t('admin.dashboard.shopeeAudit.expected')}</TableHead>
                      <TableHead>{t('admin.dashboard.shopeeAudit.actual')}</TableHead>
                      <TableHead>{t('admin.dashboard.shopeeAudit.automation')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.mismatchSamples.map((s) => (
                      <TableRow key={`${s.dispatchedAt}-${s.urlId}`}>
                        <TableCell className="whitespace-nowrap">
                          {format(new Date(s.dispatchedAt), 'dd/MM/yyyy HH:mm', { locale: ptBR })}
                        </TableCell>
                        <TableCell className="font-mono text-xs">{s.userId}</TableCell>
                        <TableCell className="font-mono text-xs">{s.expectedId}</TableCell>
                        <TableCell className="font-mono text-xs text-destructive">
                          {s.urlId}
                        </TableCell>
                        <TableCell className="text-xs" style={{ color: colors.wineTextMuted }}>
                          {s.automationName}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            ) : (
              <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                {t('admin.dashboard.shopeeAudit.noMismatch', { days })}
              </p>
            )}

            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.dashboard.shopeeAudit.shortLinkNote')}
            </p>
          </div>
        ) : (
          <div className="text-sm" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.shopeeAudit.noData')}
          </div>
        )}
      </div>
    </div>
  );
}
