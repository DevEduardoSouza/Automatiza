import { ErrorBoundary } from '@/components/error-boundary';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { useToast } from '@/hooks/use-toast';
import { useWhatsApp } from '@/hooks/use-whatsapp';
import {
  AlertCircle,
  CheckCircle2,
  Loader2,
  RefreshCw,
  Smartphone,
  Unplug,
  Wifi,
  WifiOff,
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

const statusColors = {
  connected: { iconBg: 'rgba(34,197,94,0.1)', iconBorder: 'rgba(34,197,94,0.2)', icon: '#22c55e' },
  error: { iconBg: 'rgba(239,68,68,0.1)', iconBorder: 'rgba(239,68,68,0.2)', icon: '#ef4444' },
};

type ConnectionState =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'error'
  | 'verifying'
  | 'disconnecting';

/**
 * WhatsApp connection page component.
 * Simplified UI focused on user experience.
 */
export default function WhatsApp() {
  const { t } = useTranslation();
  const { toast } = useToast();

  const {
    connecting,
    connected,
    verifying,
    disconnecting,
    qrCode,
    qrCountdown,
    importing,
    importProgress,
    importedGroups,
    autoSync,
    lastSync,
    setAutoSync,
    handleConnect,
    handleImportGroups,
    disconnectInstance,
    refreshQrCode,
  } = useWhatsApp();

  const getConnectionState = (): ConnectionState => {
    if (disconnecting) return 'disconnecting';
    if (verifying) return 'verifying';
    if (connected) return 'connected';
    if (connecting) return 'connecting';
    return 'disconnected';
  };

  const connectionState = getConnectionState();

  const handleAutoSyncToggle = () => {
    const newValue = !autoSync;
    setAutoSync(newValue);
    toast({
      title: newValue
        ? t('pages.whatsapp.success.autoSyncEnabled')
        : t('pages.whatsapp.success.autoSyncDisabled'),
      description: newValue
        ? t('pages.whatsapp.success.autoSyncEnabledMessage')
        : t('pages.whatsapp.success.autoSyncDisabledMessage'),
    });
  };

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          {/* Header */}
          <div className="flex items-center mb-5 xs:mb-6 sm:mb-8 lg:mb-10">
            <div className="flex items-center gap-2 xs:gap-3">
              <div
                className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <Smartphone className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
              </div>
              <div>
                <h2
                  className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                  style={{ color: '#18181b' }}
                >
                  {t('pages.whatsapp.title')}
                </h2>
                <p
                  className="text-xs xs:text-sm tracking-tight"
                  style={{ color: colors.wineTextMuted }}
                >
                  {t('pages.whatsapp.subtitle')}
                </p>
              </div>
            </div>
          </div>

          {/* Warning Alert */}
          <div
            className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 mb-5 xs:mb-6 sm:mb-8 lg:mb-10"
            style={{
              backgroundColor: 'rgba(234, 179, 8, 0.06)',
              borderColor: 'rgba(234, 179, 8, 0.2)',
            }}
          >
            <div className="flex items-center gap-2 xs:gap-3 mb-2 xs:mb-3">
              <div
                className="p-1.5 xs:p-2 rounded-lg"
                style={{ backgroundColor: 'rgba(234, 179, 8, 0.1)' }}
              >
                <AlertCircle
                  className="w-4 h-4 xs:w-5 xs:h-5"
                  style={{ color: '#ca8a04' }}
                />
              </div>
              <span
                className="text-base xs:text-lg font-semibold tracking-tight"
                style={{ color: '#18181b' }}
              >
                {t('pages.whatsapp.warning.title')}
              </span>
            </div>
            <p
              className="text-xs xs:text-sm leading-relaxed"
              style={{ color: '#71717a' }}
            >
              {t('pages.whatsapp.warning.message')}
            </p>
          </div>

          {/* Connection Card */}
          <div className="flex justify-center mb-5 xs:mb-6 sm:mb-8 lg:mb-10">
            <ConnectionCard
              state={connectionState}
              qrCode={qrCode}
              qrCountdown={qrCountdown}
              importing={importing}
              importProgress={importProgress}
              importedGroups={importedGroups}
              autoSync={autoSync}
              lastSync={lastSync}
              onConnect={handleConnect}
              onDisconnect={disconnectInstance}
              onReconnect={refreshQrCode}
              onRefreshQr={refreshQrCode}
              onImportGroups={() => handleImportGroups(false)}
              onAutoSyncToggle={handleAutoSyncToggle}
            />
          </div>

          {/* How It Works */}
          <HowItWorksCard />
        </div>
      </div>
    </ErrorBoundary>
  );
}

interface ConnectionCardProps {
  state: ConnectionState;
  qrCode: string | null;
  qrCountdown: number;
  importing: boolean;
  importProgress: number;
  importedGroups: string[];
  autoSync: boolean;
  lastSync: Date | null;
  onConnect: () => void;
  onDisconnect: () => void;
  onReconnect: () => void;
  onRefreshQr: () => void;
  onImportGroups: () => void;
  onAutoSyncToggle: () => void;
}

function ConnectionCard({
  state,
  qrCode,
  qrCountdown,
  importing,
  importProgress,
  importedGroups,
  autoSync,
  lastSync,
  onConnect,
  onDisconnect,
  onReconnect,
  onRefreshQr,
  onImportGroups,
  onAutoSyncToggle,
}: ConnectionCardProps) {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 w-full max-w-lg"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      {/* Header */}
      <div className="flex items-center gap-3 xs:gap-4 mb-3 xs:mb-4">
        <StatusIcon state={state} />
        <div>
          <div
            className="text-base xs:text-lg font-semibold tracking-tight"
            style={{ color: '#18181b' }}
          >
            <StatusTitle state={state} />
          </div>
          <div
            className="text-xs xs:text-sm tracking-tight"
            style={{ color: colors.wineTextMuted }}
          >
            <StatusDescription state={state} />
          </div>
        </div>
      </div>

      {/* Divider */}
      <div className="h-px mb-4 xs:mb-5" style={{ backgroundColor: colors.wineBorder }} />

      {/* Content */}
      <div className="space-y-4 xs:space-y-5 sm:space-y-6">
        {state === 'disconnecting' && <DisconnectingContent />}

        {state === 'verifying' && <VerifyingContent onDisconnect={onDisconnect} />}

        {state === 'disconnected' && <DisconnectedContent onConnect={onConnect} />}

        {state === 'connecting' && (
          <ConnectingContent
            qrCode={qrCode}
            qrCountdown={qrCountdown}
            onRefreshQr={onRefreshQr}
            onDisconnect={onDisconnect}
          />
        )}

        {state === 'connected' && (
          <ConnectedContent
            importing={importing}
            importProgress={importProgress}
            importedGroups={importedGroups}
            autoSync={autoSync}
            lastSync={lastSync}
            onImportGroups={onImportGroups}
            onAutoSyncToggle={onAutoSyncToggle}
            onReconnect={onReconnect}
            onDisconnect={onDisconnect}
          />
        )}
      </div>
    </div>
  );
}

function StatusIcon({ state }: { state: ConnectionState }) {
  const baseClass = 'p-2 xs:p-2.5 rounded-lg xs:rounded-xl border';

  switch (state) {
    case 'connected':
      return (
        <div
          className={baseClass}
          style={{
            backgroundColor: statusColors.connected.iconBg,
            borderColor: statusColors.connected.iconBorder,
          }}
        >
          <CheckCircle2
            className="h-6 w-6 xs:h-7 xs:w-7 sm:h-8 sm:w-8"
            style={{ color: statusColors.connected.icon }}
          />
        </div>
      );
    case 'connecting':
      return (
        <div
          className={baseClass}
          style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
        >
          <Smartphone
            className="h-6 w-6 xs:h-7 xs:w-7 sm:h-8 sm:w-8"
            style={{ color: colors.wine }}
          />
        </div>
      );
    case 'verifying':
      return (
        <div
          className={`${baseClass} relative`}
          style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
        >
          <Wifi
            className="h-6 w-6 xs:h-7 xs:w-7 sm:h-8 sm:w-8"
            style={{ color: colors.wineTextMuted }}
          />
          <div
            className="absolute -bottom-1 -right-1 rounded-full p-0.5"
            style={{ backgroundColor: '#ffffff' }}
          >
            <Loader2
              className="h-4 w-4 animate-spin"
              style={{ color: colors.wineTextMuted }}
            />
          </div>
        </div>
      );
    case 'disconnecting':
      return (
        <div
          className={`${baseClass} relative`}
          style={{
            backgroundColor: statusColors.error.iconBg,
            borderColor: statusColors.error.iconBorder,
          }}
        >
          <Unplug
            className="h-6 w-6 xs:h-7 xs:w-7 sm:h-8 sm:w-8"
            style={{ color: statusColors.error.icon }}
          />
          <div
            className="absolute -bottom-1 -right-1 rounded-full p-0.5"
            style={{ backgroundColor: '#ffffff' }}
          >
            <Loader2
              className="h-4 w-4 animate-spin"
              style={{ color: statusColors.error.icon }}
            />
          </div>
        </div>
      );
    case 'error':
      return (
        <div
          className={baseClass}
          style={{
            backgroundColor: statusColors.error.iconBg,
            borderColor: statusColors.error.iconBorder,
          }}
        >
          <XCircle
            className="h-6 w-6 xs:h-7 xs:w-7 sm:h-8 sm:w-8"
            style={{ color: statusColors.error.icon }}
          />
        </div>
      );
    default:
      return (
        <div
          className={baseClass}
          style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
        >
          <WifiOff
            className="h-6 w-6 xs:h-7 xs:w-7 sm:h-8 sm:w-8"
            style={{ color: colors.wineTextMuted }}
          />
        </div>
      );
  }
}

function StatusTitle({ state }: { state: ConnectionState }) {
  const { t } = useTranslation();

  switch (state) {
    case 'connected':
      return t('pages.whatsapp.connected.status');
    case 'connecting':
      return t('pages.whatsapp.disconnected.connecting');
    case 'verifying':
      return t('pages.whatsapp.verifying.title', 'Verificando...');
    case 'disconnecting':
      return t('pages.whatsapp.disconnecting.title', 'Desconectando...');
    case 'error':
      return t('pages.whatsapp.errors.connect');
    default:
      return t('pages.whatsapp.connect.title');
  }
}

function StatusDescription({ state }: { state: ConnectionState }) {
  const { t } = useTranslation();

  switch (state) {
    case 'connected':
      return t('pages.whatsapp.connected.description');
    case 'connecting':
      return t('pages.whatsapp.qrCode.description');
    case 'verifying':
      return t('pages.whatsapp.verifying.description', 'Verificando status da conexão...');
    case 'disconnecting':
      return t('pages.whatsapp.disconnecting.description', 'Removendo instância...');
    case 'error':
      return t('pages.whatsapp.errors.connectMessage');
    default:
      return t('pages.whatsapp.connect.description');
  }
}

function DisconnectingContent() {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col items-center py-6 xs:py-8">
      <Loader2
        className="h-12 w-12 animate-spin mb-4"
        style={{ color: colors.wine }}
      />
      <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
        {t('pages.whatsapp.disconnecting.message', 'Desconectando...')}
      </p>
    </div>
  );
}

interface VerifyingContentProps {
  onDisconnect: () => void;
}

function VerifyingContent({ onDisconnect }: VerifyingContentProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      <div className="flex flex-col items-center py-6 xs:py-8">
        <Loader2
          className="h-12 w-12 animate-spin mb-4"
          style={{ color: colors.wineTextMuted }}
        />
        <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
          {t('pages.whatsapp.verifying.message', 'Verificando status da conexão...')}
        </p>
      </div>

      <Button
        variant="outline"
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          onDisconnect();
        }}
        className="w-full gap-2 xs:gap-2.5 cursor-pointer relative z-10"
        style={{ color: colors.wine, borderColor: colors.wineBorder }}
      >
        <Unplug className="h-4 w-4" />
        {t('pages.whatsapp.connected.disconnect')}
      </Button>
    </div>
  );
}

interface DisconnectedContentProps {
  onConnect: () => void;
}

function DisconnectedContent({ onConnect }: DisconnectedContentProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      <Button onClick={onConnect} size="lg" className="w-full gap-2 xs:gap-2.5">
        <Wifi className="h-4 w-4 xs:h-5 xs:w-5" />
        {t('pages.whatsapp.disconnected.generateQR')}
      </Button>
    </div>
  );
}

interface ConnectingContentProps {
  qrCode: string | null;
  qrCountdown: number;
  onRefreshQr: () => void;
  onDisconnect: () => void;
}

function ConnectingContent({
  qrCode,
  qrCountdown,
  onRefreshQr,
  onDisconnect,
}: ConnectingContentProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      {qrCode ? (
        <div className="flex flex-col items-center">
          <div
            className="p-3 xs:p-4 rounded-xl xs:rounded-2xl border"
            style={{
              borderColor: colors.wineBorder,
              boxShadow: '0 4px 12px rgba(0,0,0,0.08)',
              backgroundColor: '#ffffff',
            }}
          >
            <img src={qrCode} alt="QR Code" className="w-56 h-56 xs:w-64 xs:h-64" />
          </div>

          <div className="mt-4 w-full">
            <div className="flex items-center justify-between text-xs xs:text-sm mb-2">
              <span style={{ color: colors.wineTextMuted }}>
                {t('pages.whatsapp.qrCode.expiresIn', { seconds: qrCountdown })}
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={onRefreshQr}
                className="h-8 gap-1 text-xs cursor-pointer"
                style={{ color: colors.wine }}
              >
                <RefreshCw className="h-3 w-3" />
                {t('common.refresh')}
              </Button>
            </div>
            <Progress
              value={(qrCountdown / 30) * 100}
              className={`h-2 ${(qrCountdown / 30) * 100 === 0 ? '[&>div]:bg-transparent' : ''}`}
              style={{ backgroundColor: 'rgba(141,45,83,0.1)' }}
            />
          </div>

          <p
            className="text-[10px] xs:text-xs mt-4 text-center tracking-tight"
            style={{ color: colors.wineTextMuted }}
          >
            {t('pages.whatsapp.qrCode.waiting')}
          </p>
        </div>
      ) : (
        <div className="flex flex-col items-center py-6 xs:py-8">
          <Loader2
            className="h-12 w-12 animate-spin mb-4"
            style={{ color: colors.wine }}
          />
          <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
            {t('pages.whatsapp.qrCode.generating')}
          </p>
        </div>
      )}

      <Button
        variant="outline"
        onClick={onDisconnect}
        className="w-full gap-2 xs:gap-2.5 cursor-pointer"
        style={{ color: colors.wine, borderColor: colors.wineBorder }}
      >
        <Unplug className="h-4 w-4" />
        {t('pages.whatsapp.connected.disconnect')}
      </Button>
    </div>
  );
}

interface ConnectedContentProps {
  importing: boolean;
  importProgress: number;
  importedGroups: string[];
  autoSync: boolean;
  lastSync: Date | null;
  onImportGroups: () => void;
  onAutoSyncToggle: () => void;
  onReconnect: () => void;
  onDisconnect: () => void;
}

function ConnectedContent({
  importing,
  importProgress,
  importedGroups,
  autoSync,
  lastSync,
  onImportGroups,
  onAutoSyncToggle,
  onReconnect,
  onDisconnect,
}: ConnectedContentProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <div
          className="flex items-center justify-center gap-2 w-full py-2 rounded-full border text-xs xs:text-sm font-medium"
          style={{
            backgroundColor: 'rgba(34,197,94,0.1)',
            borderColor: 'rgba(34,197,94,0.2)',
            color: '#16a34a',
          }}
        >
          <CheckCircle2 className="h-4 w-4" />
          {t('pages.whatsapp.success.connectedMessage')}
        </div>
        {lastSync && (
          <p
            className="text-[10px] xs:text-xs text-center tracking-tight"
            style={{ color: colors.wineTextMuted }}
          >
            {t('pages.whatsapp.connected.lastSync')}: {lastSync.toLocaleString()}
          </p>
        )}
      </div>

      {importing && <ImportingProgress progress={importProgress} importedGroups={importedGroups} />}

      <Button
        onClick={onImportGroups}
        variant="outline"
        className="w-full gap-2 xs:gap-2.5"
        disabled={importing}
        style={{ color: colors.wine, borderColor: colors.wineBorder }}
      >
        <RefreshCw className={`h-4 w-4 ${importing ? 'animate-spin' : ''}`} />
        {importing
          ? t('pages.whatsapp.connected.importing')
          : t('pages.whatsapp.connected.syncGroupsManually')}
      </Button>

      <AutoSyncToggle autoSync={autoSync} onToggle={onAutoSyncToggle} />

      <div className="flex gap-2 xs:gap-3 pt-2">
        <Button
          variant="outline"
          onClick={onReconnect}
          disabled={importing}
          className="flex-1 gap-2 xs:gap-2.5 cursor-pointer"
          style={{ color: colors.wine, borderColor: colors.wineBorder }}
        >
          <RefreshCw className="h-4 w-4" />
          {t('pages.whatsapp.connected.reconnect')}
        </Button>
        <Button
          variant="outline"
          onClick={onDisconnect}
          className="flex-1 gap-2 xs:gap-2.5 cursor-pointer"
          style={{ color: colors.wine, borderColor: colors.wineBorder }}
        >
          <Unplug className="h-4 w-4" />
          {t('pages.whatsapp.connected.disconnect')}
        </Button>
      </div>
    </div>
  );
}

interface ImportingProgressProps {
  progress: number;
  importedGroups: string[];
}

function ImportingProgress({ progress, importedGroups }: ImportingProgressProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <div className="flex justify-between text-xs xs:text-sm">
          <span style={{ color: colors.wineTextMuted }}>{t('pages.whatsapp.connected.importing')}</span>
          <span className="font-medium" style={{ color: colors.wine }}>{progress}%</span>
        </div>
        <Progress
          value={progress}
          className={`h-2 border ${progress === 0 ? '[&>div]:bg-transparent' : ''}`}
          style={{ backgroundColor: 'rgba(141,45,83,0.1)', borderColor: colors.wineBorder }}
        />
      </div>

      {importedGroups.length > 0 && (
        <div
          className="space-y-2 max-h-32 overflow-y-auto rounded-lg xs:rounded-xl border p-2.5 xs:p-3"
          style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
        >
          <p
            className="uppercase text-[10px] xs:text-xs tracking-[0.1em] font-medium"
            style={{ color: colors.wineTextLight }}
          >
            {t('pages.whatsapp.connected.importGroups')} ({importedGroups.length}):
          </p>
          {importedGroups.map((group) => (
            <p
              key={group}
              className="text-[10px] xs:text-xs truncate tracking-tight"
              style={{ color: colors.wineTextMuted }}
            >
              ✓ {group}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}

interface AutoSyncToggleProps {
  autoSync: boolean;
  onToggle: () => void;
}

function AutoSyncToggle({ autoSync, onToggle }: AutoSyncToggleProps) {
  const { t } = useTranslation();

  return (
    <div
      className="flex items-center justify-between border rounded-lg xs:rounded-xl p-3 xs:p-4"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="space-y-0.5">
        <p
          className="text-xs xs:text-sm font-semibold tracking-tight"
          style={{ color: '#18181b' }}
        >
          {t('pages.whatsapp.connected.autoSync.title')}
        </p>
        <p className="text-[10px] xs:text-xs tracking-tight" style={{ color: colors.wineTextMuted }}>
          {autoSync
            ? t('pages.whatsapp.connected.autoSync.enabled')
            : t('pages.whatsapp.connected.autoSync.disabled')}
        </p>
      </div>
      <Button
        variant={autoSync ? 'default' : 'outline'}
        size="sm"
        onClick={onToggle}
        className="cursor-pointer"
        style={
          autoSync
            ? { backgroundColor: colors.wine }
            : { color: colors.wine, borderColor: colors.wineBorder }
        }
      >
        {autoSync
          ? t('pages.whatsapp.connected.autoSync.activated')
          : t('pages.whatsapp.connected.autoSync.activate')}
      </Button>
    </div>
  );
}

function HowItWorksCard() {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      {/* Header */}
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
        <div
          className="p-1.5 xs:p-2 rounded-lg"
          style={{ backgroundColor: colors.iconBg }}
        >
          <AlertCircle className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
        </div>
        <span
          className="text-base xs:text-lg font-semibold tracking-tight"
          style={{ color: '#18181b' }}
        >
          {t('pages.whatsapp.howItWorks.title')}
        </span>
      </div>

      {/* Divider */}
      <div className="h-px mb-4 xs:mb-5" style={{ backgroundColor: colors.wineBorder }} />

      {/* Steps */}
      <div className="space-y-3 xs:space-y-4">
        {[1, 2, 3, 4, 5, 6].map((step) => (
          <div key={step} className="flex items-start gap-2.5 xs:gap-3">
            <div
              className="shrink-0 w-6 h-6 xs:w-7 xs:h-7 rounded-full flex items-center justify-center"
              style={{ backgroundColor: colors.iconBg }}
            >
              <span
                className="text-[10px] xs:text-xs font-semibold"
                style={{ color: colors.wine }}
              >
                {step}
              </span>
            </div>
            <p
              className="text-xs xs:text-sm tracking-tight leading-relaxed"
              style={{ color: '#71717a' }}
            >
              {t(`pages.whatsapp.howItWorks.steps_${step}`)}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}
