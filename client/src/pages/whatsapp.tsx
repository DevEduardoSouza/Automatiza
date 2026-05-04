import { ErrorBoundary } from '@/components/error-boundary';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { InstancesList } from '@/components/whatsapp/instances-list';
import { useToast } from '@/hooks/use-toast';
import { useWhatsApp } from '@/hooks/use-whatsapp';
import * as Sentry from '@sentry/react';
import {
  AlertCircle,
  Loader2,
  RefreshCw,
  Trash2,
  Unplug,
  Wifi,
  WifiOff,
  XCircle,
} from 'lucide-react';

const ConnectedIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
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
    <path d="M17 3.33782C15.5291 2.48697 13.8214 2 12 2C6.47715 2 2 6.47715 2 12C2 17.5228 6.47715 22 12 22C17.5228 22 22 17.5228 22 12C22 11.3151 21.9311 10.6462 21.8 10" />
    <path d="M8 12.5C8 12.5 9.5 12.5 11.5 16C11.5 16 17.0588 6.83333 22 5" />
  </svg>
);

const ConnectingIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    style={style}
  >
    <path d="M10.5 19H10.51" />
    <path d="M19 12.0337H12M17 9C17 9 19.0371 10.6469 19.8208 11.5533C19.9468 11.6991 20.0064 11.8627 19.9995 12.0253C19.9933 12.1711 19.9337 12.3162 19.8207 12.4468C19.0368 13.3531 17 15 17 15" />
    <path d="M17 6C16.9855 4.29344 16.8908 3.35264 16.2702 2.73223C15.5378 2 14.3591 2 12.0016 2H9.001C6.64351 2 5.46476 2 4.73238 2.73223C4 3.46447 4 4.64298 4 7V17C4 19.357 4 20.5355 4.73238 21.2678C5.46476 22 6.64351 22 9.001 22H12.0016C14.3591 22 15.5378 22 16.2702 21.2678C16.8908 20.6473 16.9855 19.7065 17 17.9999" />
  </svg>
);

const WhatsAppIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    style={style}
  >
    <path d="M12.0045 11H12.0135M8.00903 11H8.018" />
    <path d="M12 3C10.5209 3 9.09517 3.0309 7.7562 3.08819C5.3157 3.1926 4.09545 3.24481 3.13007 4.21745C2.16469 5.19009 2.12282 6.37683 2.03909 8.7503C2.01346 9.47679 2 10.2292 2 11C2 11.7708 2.01346 12.5232 2.03909 13.2497C2.12282 15.6232 2.16469 16.8099 3.13007 17.7825C4.09545 18.7552 5.31569 18.8074 7.75619 18.9118C7.83715 18.9153 7.91842 18.9186 8 18.9219V21.2701C8 21.6732 8.32679 22 8.72991 22C8.90419 22 9.07273 21.9376 9.20503 21.8242L11.3845 19.9553C11.9325 19.4855 12.2064 19.2506 12.532 19.1266C12.8576 19.0026 13.2282 18.9955 13.9693 18.9815C14.7498 18.9667 15.5098 18.9432 16.2437 18.9118C18.6843 18.8074 19.9046 18.7552 20.8699 17.7826C21.8353 16.8099 21.8772 15.6232 21.9609 13.2497C21.9865 12.5232 22 11.7708 22 11C22 10.2292 21.9865 9.47679 21.9609 8.7503C21.9372 8.07936 21.9169 7.50325 21.8781 7" />
    <path d="M15 5.33333C15 5.33333 16 5.33333 17 7C17 7 19.1765 2.83333 22 2" />
  </svg>
);
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
    <ErrorBoundary
      fallback={
        <WahaErrorFallback
          onReconnect={() => {
            refreshQrCode();
          }}
          onClearSession={() => {
            disconnectInstance();
          }}
        />
      }
    >
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          {/* Header */}
          <div className="flex items-center mb-5 xs:mb-6 sm:mb-8 lg:mb-10">
            <div className="flex items-center gap-2 xs:gap-3">
              <div
                className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <WhatsAppIcon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
              </div>
              <div>
                <h2
                  className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
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
            className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8 mb-5 xs:mb-6 sm:mb-8 lg:mb-10"
            role="alert"
            style={{
              backgroundColor: 'rgba(180, 120, 60, 0.08)',
              borderColor: 'rgba(180, 120, 60, 0.25)',
              backdropFilter: 'blur(12px) saturate(180%)',
              WebkitBackdropFilter: 'blur(12px) saturate(180%)',
            }}
          >
            <div className="flex items-center gap-2 xs:gap-3 mb-2 xs:mb-3">
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
              <span
                className="text-sm xs:text-base font-semibold tracking-tight"
                style={{ color: 'rgb(120, 75, 30)' }}
              >
                {t('pages.whatsapp.warning.title')}
              </span>
            </div>
            <p
              className="text-xs xs:text-sm leading-relaxed"
              style={{ color: 'rgba(120, 75, 30, 0.8)' }}
            >
              {t('pages.whatsapp.warning.message')}
            </p>
          </div>

          {/* Existing instances list (renders only when the user has at least one chip) */}
          <InstancesList />

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
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 w-full max-w-lg sm:max-w-xl"
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
          <ConnectedIcon
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
          <ConnectingIcon
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

      <button
        type="button"
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          onDisconnect();
        }}
        className="relative z-10 inline-flex w-full cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
      >
        <Unplug className="h-4 w-4" />
        {t('pages.whatsapp.connected.disconnect')}
      </button>
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
      <button
        type="button"
        onClick={onConnect}
        className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all"
      >
        <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
          <div className="auth-btn-glow__glow absolute inset-0" />
        </div>
        <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
          <Wifi className="h-4 w-4 xs:h-5 xs:w-5" />
          {t('pages.whatsapp.disconnected.generateQR')}
        </span>
      </button>
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
            className="p-2.5 xs:p-3 sm:p-4 rounded-xl xs:rounded-2xl border"
            style={{
              borderColor: colors.wineBorder,
              boxShadow: '0 4px 12px rgba(0,0,0,0.08)',
              backgroundColor: '#ffffff',
            }}
          >
            <img src={qrCode} alt="QR Code" className="w-48 h-48 xs:w-56 xs:h-56 sm:w-64 sm:h-64" />
          </div>

          <div className="mt-4 w-full">
            <div className="flex items-center justify-between text-xs xs:text-sm mb-2">
              <span style={{ color: colors.wineTextMuted }}>
                {t('pages.whatsapp.qrCode.expiresIn', { seconds: qrCountdown })}
              </span>
              <button
                type="button"
                onClick={onRefreshQr}
                className="inline-flex items-center gap-1 rounded-full px-2.5 xs:px-3 py-1 xs:py-1.5 text-[10px] xs:text-xs font-medium text-[#8D2D53] cursor-pointer transition-all hover:bg-[rgba(141,45,83,0.08)]"
              >
                <RefreshCw className="h-2.5 w-2.5 xs:h-3 xs:w-3" />
                {t('common.refresh')}
              </button>
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

      <button
        type="button"
        onClick={onDisconnect}
        className="inline-flex w-full cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
      >
        <Unplug className="h-4 w-4" />
        {t('pages.whatsapp.connected.disconnect')}
      </button>
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
          <ConnectedIcon className="h-4 w-4" />
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

      <button
        type="button"
        onClick={onImportGroups}
        disabled={importing}
        className="inline-flex w-full cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
      >
        <RefreshCw className={`h-4 w-4 ${importing ? 'animate-spin' : ''}`} />
        {importing
          ? t('pages.whatsapp.connected.importing')
          : t('pages.whatsapp.connected.syncGroupsManually')}
      </button>

      <AutoSyncToggle autoSync={autoSync} onToggle={onAutoSyncToggle} />

      <div className="flex flex-col xs:flex-row gap-2 xs:gap-3 pt-2">
        <button
          type="button"
          onClick={onReconnect}
          disabled={importing}
          className="inline-flex flex-1 cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
        >
          <RefreshCw className="h-4 w-4" />
          {t('pages.whatsapp.connected.reconnect')}
        </button>
        <button
          type="button"
          onClick={onDisconnect}
          className="inline-flex flex-1 cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
        >
          <Unplug className="h-4 w-4" />
          {t('pages.whatsapp.connected.disconnect')}
        </button>
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
          className="space-y-2 max-h-32 lg:max-h-48 overflow-y-auto rounded-lg xs:rounded-xl border p-2.5 xs:p-3"
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
      <button
        type="button"
        onClick={onToggle}
        className={`inline-flex cursor-pointer items-center justify-center rounded-full px-3 xs:px-4 py-1.5 xs:py-2 text-xs xs:text-sm font-semibold transition-all ${
          autoSync
            ? 'bg-[#8D2D53] text-white shadow-sm hover:bg-[#8D2D53]/90'
            : 'border border-gray-300 bg-white text-[#8D2D53] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md'
        }`}
      >
        {autoSync
          ? t('pages.whatsapp.connected.autoSync.activated')
          : t('pages.whatsapp.connected.autoSync.activate')}
      </button>
    </div>
  );
}

interface WahaErrorFallbackProps {
  onReconnect: () => void;
  onClearSession: () => void;
}

/**
 * WAHA-specific error fallback with recovery actions (Reconnect / Clear session).
 */
function WahaErrorFallback({ onReconnect, onClearSession }: WahaErrorFallbackProps) {
  const { t } = useTranslation();

  return (
    <div className="min-h-[400px] flex items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4">
            <WifiOff className="h-12 w-12 text-destructive" aria-hidden="true" />
          </div>
          <CardTitle>{t('pages.whatsapp.errorBoundary.title')}</CardTitle>
          <CardDescription>{t('pages.whatsapp.errorBoundary.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <Button
            onClick={() => {
              Sentry.addBreadcrumb({
                category: 'whatsapp',
                message: 'User initiated reconnect from error boundary',
                level: 'info',
              });
              onReconnect();
            }}
            className="w-full"
            aria-label={t('pages.whatsapp.errorBoundary.reconnect')}
          >
            <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
            {t('pages.whatsapp.errorBoundary.reconnect')}
          </Button>
          <Button
            variant="outline"
            onClick={() => {
              Sentry.addBreadcrumb({
                category: 'whatsapp',
                message: 'User initiated clear session from error boundary',
                level: 'info',
              });
              onClearSession();
            }}
            className="w-full"
            aria-label={t('pages.whatsapp.errorBoundary.clearSession')}
          >
            <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
            {t('pages.whatsapp.errorBoundary.clearSession')}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function HowItWorksCard() {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8 xl:p-10"
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
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 xs:gap-4">
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
