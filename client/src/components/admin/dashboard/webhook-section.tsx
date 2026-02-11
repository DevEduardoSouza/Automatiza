import { Badge } from '@/components/ui/badge';
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
import { useIsMobile } from '@/hooks/use-mobile';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { safeClipboard } from '@/lib/sentry';
import { CheckCircle2, Copy, Loader2 } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface WebhookLog {
  id: string;
  email: string;
  event: string;
  product: string | null;
  processed_at: string;
}

interface WebhookSectionProps {
  webhookLogs: WebhookLog[];
  onSimulate: (email: string, event: string, product: string) => Promise<void>;
}

/**
 * Seção de webhooks do dashboard.
 * Inclui URL do webhook, simulador e lista de eventos recentes.
 */
export function WebhookSection({ webhookLogs, onSimulate }: WebhookSectionProps) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [email, setEmail] = useState('');
  const [event, setEvent] = useState('');
  const [product, setProduct] = useState('');
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  const webhookUrl = `${window.location.origin}/api/v1/webhooks/kiwify`;

  const copyWebhookUrl = async () => {
    const success = await safeClipboard(webhookUrl);
    if (success) {
      setCopied(true);
      toast.success(t('admin.dashboard.webhook.copySuccess'));
      setTimeout(() => setCopied(false), 2000);
    } else {
      toast.error(t('common.clipboardUnavailable'));
    }
  };

  const handleSimulateWebhook = async () => {
    if (!email || !event) {
      toast.error(t('admin.dashboard.webhook.simulator.fillEmailAndEvent'));
      return;
    }

    setLoading(true);
    try {
      await onSimulate(email, event, product);
      setEmail('');
      setEvent('');
      setProduct('');
    } finally {
      setLoading(false);
    }
  };

  const getEventBadge = (eventName: string) => {
    const eventLower = eventName.toLowerCase();
    if (eventLower.includes('aprovada') || eventLower.includes('renovada')) {
      return <Badge className="bg-green-500">{t('common.active')}</Badge>;
    }
    if (eventLower.includes('cancelada')) {
      return <Badge variant="destructive">{t('admin.dashboard.webhook.badges.cancelled')}</Badge>;
    }
    if (eventLower.includes('atrasada')) {
      return <Badge className="bg-yellow-500">{t('admin.dashboard.webhook.badges.overdue')}</Badge>;
    }
    return <Badge variant="secondary">{eventName}</Badge>;
  };

  const getPlanFromProduct = (productName: string | null) => {
    if (!productName) return t('common.notAvailable');
    const productLower = productName.toLowerCase();
    if (productLower.includes('starter')) return t('common.planNames.starter');
    if (productLower.includes('pro')) return t('common.planNames.pro');
    if (productLower.includes('master')) return t('common.planNames.master');
    return productName;
  };

  return (
    <>
      {/* URL do Webhook */}
      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="mb-3 xs:mb-4">
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('admin.dashboard.webhook.urlTitle')}
          </span>
          <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.webhook.urlDescription')}
          </p>
        </div>
        <div className="space-y-3 xs:space-y-4">
          <div className="flex flex-col sm:flex-row gap-2">
            <Input value={webhookUrl} readOnly className="font-mono text-xs md:text-sm" />
            <button
              type="button"
              onClick={copyWebhookUrl}
              className="inline-flex shrink-0 cursor-pointer items-center justify-center rounded-full border border-gray-300 bg-white p-2 text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
            >
              {copied ? <CheckCircle2 className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
            </button>
          </div>
          <p className="text-xs md:text-sm" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.webhook.configureToken')}
          </p>
        </div>
      </div>

      {/* Simulador de Webhooks */}
      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="mb-3 xs:mb-4">
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('admin.dashboard.webhook.simulator.title')}
          </span>
          <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.webhook.simulator.description')}
          </p>
        </div>
        <div className="space-y-3 xs:space-y-4">
          <div className="grid grid-cols-1 gap-3 xs:gap-4">
            <div className="space-y-2">
              <Label htmlFor="email">{t('admin.dashboard.webhook.simulator.email')}</Label>
              <Input
                id="email"
                type="email"
                placeholder={t('admin.dashboard.webhook.simulator.emailPlaceholder')}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="event">{t('admin.dashboard.webhook.simulator.event')}</Label>
              <Select value={event} onValueChange={setEvent}>
                <SelectTrigger id="event">
                  <SelectValue
                    placeholder={t('admin.dashboard.webhook.simulator.eventPlaceholder')}
                  />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="assinatura aprovada">
                    {t('admin.dashboard.webhook.events.approved')}
                  </SelectItem>
                  <SelectItem value="assinatura renovada">
                    {t('admin.dashboard.webhook.events.renewed')}
                  </SelectItem>
                  <SelectItem value="assinatura cancelada">
                    {t('admin.dashboard.webhook.events.cancelled')}
                  </SelectItem>
                  <SelectItem value="assinatura atrasada">
                    {t('admin.dashboard.webhook.events.overdue')}
                  </SelectItem>
                  <SelectItem value="assinatura expirada">
                    {t('admin.dashboard.webhook.events.expired')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="product">{t('admin.dashboard.webhook.simulator.product')}</Label>
              <Select value={product} onValueChange={setProduct}>
                <SelectTrigger id="product">
                  <SelectValue
                    placeholder={t('admin.dashboard.webhook.simulator.productPlaceholder')}
                  />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="Plano Starter">
                    {t('admin.dashboard.webhook.plans.starter')}
                  </SelectItem>
                  <SelectItem value="Plano Pro">
                    {t('admin.dashboard.webhook.plans.pro')}
                  </SelectItem>
                  <SelectItem value="Plano Master">
                    {t('admin.dashboard.webhook.plans.master')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <button
            type="button"
            onClick={handleSimulateWebhook}
            disabled={loading}
            className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              {loading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t('common.processing')}
                </>
              ) : (
                t('admin.dashboard.webhook.simulator.simulate')
              )}
            </span>
          </button>
        </div>
      </div>

      {/* Últimos Eventos */}
      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="mb-3 xs:mb-4">
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('admin.dashboard.webhook.lastEvents.title')}
          </span>
          <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.webhook.lastEvents.description')}
          </p>
        </div>
        <div className="space-y-3 xs:space-y-4">
          {webhookLogs.length === 0 ? (
            <p className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
              {t('admin.dashboard.webhook.lastEvents.noEvents')}
            </p>
          ) : isMobile ? (
            <div className="space-y-3">
              {webhookLogs.map((log) => (
                <div
                  key={log.id}
                  className="rounded-lg xs:rounded-xl border p-2.5 xs:p-3 space-y-2"
                  style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1 min-w-0">
                      <p className="font-mono text-xs truncate">{log.email}</p>
                      <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                        {format(new Date(log.processed_at), 'dd/MM/yyyy HH:mm', { locale: ptBR })}
                      </p>
                    </div>
                    {getEventBadge(log.event)}
                  </div>
                  <div className="flex items-center gap-2 text-xs pt-2 border-t">
                    <span style={{ color: colors.wineTextMuted }}>
                      {t('admin.dashboard.webhook.lastEvents.plan')}:
                    </span>
                    <span className="font-medium">{getPlanFromProduct(log.product)}</span>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('admin.dashboard.webhook.lastEvents.dateTime')}</TableHead>
                    <TableHead>{t('common.email')}</TableHead>
                    <TableHead>{t('admin.dashboard.webhook.simulator.event')}</TableHead>
                    <TableHead>{t('admin.dashboard.webhook.lastEvents.plan')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {webhookLogs.map((log) => (
                    <TableRow key={log.id}>
                      <TableCell className="whitespace-nowrap">
                        {format(new Date(log.processed_at), 'dd/MM/yyyy HH:mm', { locale: ptBR })}
                      </TableCell>
                      <TableCell className="font-mono text-sm">{log.email}</TableCell>
                      <TableCell>{getEventBadge(log.event)}</TableCell>
                      <TableCell>{getPlanFromProduct(log.product)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>
      </div>
    </>
  );
}
