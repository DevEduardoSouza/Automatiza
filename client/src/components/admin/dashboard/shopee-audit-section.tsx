import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { useTranslation } from 'react-i18next';
import type { ShopeeAuditSectionProps } from '../types/dashboard.types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

/**
 * Seção de auditoria Shopee (Afiliados).
 * Verifica inconsistências de afiliado vs appID do usuário.
 */
export function ShopeeAuditSection({
  audit,
  auditLoading,
  auditDays,
  onAuditDaysChange,
  onRefresh,
}: ShopeeAuditSectionProps) {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between mb-3 xs:mb-4 sm:mb-5">
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
          <div className="flex items-center gap-2">
            <Label htmlFor="auditDays" className="text-xs">
              {t('admin.dashboard.shopeeAudit.days')}
            </Label>
            <Input
              id="auditDays"
              type="number"
              className="w-20"
              min={1}
              max={90}
              value={auditDays}
              onChange={(e) => onAuditDaysChange(Number(e.target.value || 7))}
              aria-label={t('admin.dashboard.shopeeAudit.days')}
            />
          </div>
          <Button variant="outline" onClick={() => onRefresh()} disabled={auditLoading}>
            {auditLoading
              ? t('admin.dashboard.loadBalancer.refreshing')
              : t('admin.dashboard.loadBalancer.refresh')}
          </Button>
        </div>
      </div>
      <div className="space-y-3 xs:space-y-4">
        {!audit ? (
          <Alert>
            <AlertTitle>{t('common.none')}</AlertTitle>
            <AlertDescription>{t('admin.dashboard.shopeeAudit.noData')}</AlertDescription>
          </Alert>
        ) : (
          <>
            <div className="grid grid-cols-2 md:grid-cols-5 gap-2 xs:gap-3">
              <div
                className="rounded-md border p-3"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.shopeeAudit.scanned')}
                </div>
                <div className="text-lg font-semibold">{audit.scanned}</div>
              </div>
              <div
                className="rounded-md border p-3"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.shopeeAudit.withIdentifier')}
                </div>
                <div className="text-lg font-semibold">{audit.withIdentifier}</div>
              </div>
              <div
                className="rounded-md border p-3"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.shopeeAudit.mismatch')}
                </div>
                <div className="text-lg font-semibold">{audit.mismatches}</div>
              </div>
              <div
                className="rounded-md border p-3"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.shopeeAudit.noQuery')}
                </div>
                <div className="text-lg font-semibold">{audit.unknown}</div>
              </div>
              <div
                className="rounded-md border p-3"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.shopeeAudit.noShopeeCredentials')}
                </div>
                <div className="text-lg font-semibold">{audit.missingCreds}</div>
              </div>
            </div>

            {audit.mismatches > 0 ? (
              <div className="space-y-2">
                <div className="text-sm font-medium">
                  {t('admin.dashboard.shopeeAudit.mismatchSamples')}
                </div>
                <div className="overflow-x-auto rounded-md border">
                  <Table aria-label={t('admin.dashboard.shopeeAudit.mismatchSamples')}>
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
                      {audit.samples.map((s) => (
                        <TableRow key={`${s.user_id}-${s.created_at}-${s.automation_id}`}>
                          <TableCell className="whitespace-nowrap">
                            {format(new Date(s.created_at), 'dd/MM HH:mm', {
                              locale: ptBR,
                            })}
                          </TableCell>
                          <TableCell className="min-w-[240px]">
                            <div className="text-sm font-medium">{s.email || s.user_id}</div>
                            {s.phone_number ? (
                              <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                                {s.phone_number}
                              </div>
                            ) : null}
                          </TableCell>
                          <TableCell className="whitespace-nowrap">
                            <Badge variant="secondary">{s.expected_mmp_pid}</Badge>
                          </TableCell>
                          <TableCell className="whitespace-nowrap">
                            <Badge variant="destructive">
                              {s.actual_mmp_pid || s.actual_af_siteid || '-'}
                            </Badge>
                          </TableCell>
                          <TableCell className="min-w-[240px]">
                            <div className="text-sm">{s.automation_name}</div>
                            <div
                              className="text-xs truncate max-w-[360px]"
                              style={{ color: colors.wineTextMuted }}
                            >
                              {s.url}
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </div>
            ) : (
              <Alert>
                <AlertTitle>{t('common.ok')}</AlertTitle>
                <AlertDescription>
                  {t('admin.dashboard.shopeeAudit.noMismatch', {
                    days: audit.window.days,
                  })}
                </AlertDescription>
              </Alert>
            )}

            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.dashboard.shopeeAudit.shortLinkNote')}
            </p>
          </>
        )}
      </div>
    </div>
  );
}
