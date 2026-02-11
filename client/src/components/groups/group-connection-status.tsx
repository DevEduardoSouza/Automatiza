import { Loader2, Wifi, WifiOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';

type ConnectionStatus = 'checking' | 'connected' | 'disconnected';

interface GroupConnectionStatusProps {
  status: ConnectionStatus;
  instanceName: string;
  reconnecting: boolean;
  onReconnect: () => void;
}

/**
 * Alertas de status de conexão do WhatsApp.
 */
export function GroupConnectionStatus({
  status,
  instanceName,
  reconnecting,
  onReconnect,
}: GroupConnectionStatusProps) {
  const { t } = useTranslation();

  if (status === 'disconnected') {
    return (
      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 md:p-7 lg:p-8 xl:p-10"
        role="alert"
        style={{
          backgroundColor: 'rgba(239, 68, 68, 0.06)',
          borderColor: 'rgba(239, 68, 68, 0.25)',
          backdropFilter: 'blur(12px) saturate(180%)',
          WebkitBackdropFilter: 'blur(12px) saturate(180%)',
        }}
      >
        <div className="flex items-center gap-2 xs:gap-3 md:gap-4 mb-2 xs:mb-3 lg:mb-4">
          <div
            className="p-1.5 xs:p-2 rounded-lg"
            style={{ backgroundColor: 'rgba(239, 68, 68, 0.1)' }}
          >
            <WifiOff
              className="w-4 h-4 xs:w-5 xs:h-5"
              style={{ color: 'rgb(220, 50, 50)' }}
              aria-hidden="true"
            />
          </div>
          <span
            className="text-sm xs:text-base font-semibold tracking-tight"
            style={{ color: 'rgb(185, 40, 40)' }}
          >
            {t('pages.groups.connection.disconnected')}
          </span>
        </div>
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 md:gap-4 lg:gap-6">
          <p
            className="text-xs xs:text-sm leading-relaxed"
            style={{ color: 'rgba(185, 40, 40, 0.8)' }}
          >
            {t('pages.groups.connection.disconnectedMessage', { instanceName })}
          </p>
          <button
            type="button"
            onClick={onReconnect}
            disabled={reconnecting}
            className="inline-flex w-full sm:w-auto cursor-pointer items-center justify-center gap-2 rounded-full border border-red-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-red-600 shadow-sm transition-all hover:-translate-y-0.5 hover:border-red-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
          >
            {reconnecting
              ? t('pages.groups.connection.reconnecting')
              : t('pages.groups.connection.reconnectNow')}
          </button>
        </div>
      </div>
    );
  }

  if (status === 'connected') {
    return (
      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 md:p-7 lg:p-8 xl:p-10"
        role="alert"
        style={{
          backgroundColor: 'rgba(34, 197, 94, 0.06)',
          borderColor: 'rgba(34, 197, 94, 0.25)',
          backdropFilter: 'blur(12px) saturate(180%)',
          WebkitBackdropFilter: 'blur(12px) saturate(180%)',
        }}
      >
        <div className="flex items-center gap-2 xs:gap-3 md:gap-4 mb-2 xs:mb-3 lg:mb-4">
          <div
            className="p-1.5 xs:p-2 rounded-lg"
            style={{ backgroundColor: 'rgba(34, 197, 94, 0.1)' }}
          >
            <Wifi
              className="w-4 h-4 xs:w-5 xs:h-5"
              style={{ color: 'rgb(22, 163, 74)' }}
              aria-hidden="true"
            />
          </div>
          <span
            className="text-sm xs:text-base font-semibold tracking-tight"
            style={{ color: 'rgb(22, 128, 61)' }}
          >
            {t('pages.groups.connection.connected')}
          </span>
        </div>
        <p
          className="text-xs xs:text-sm leading-relaxed"
          style={{ color: 'rgba(22, 128, 61, 0.8)' }}
        >
          {t('pages.groups.connection.connectedMessage', { instanceName })}
        </p>
      </div>
    );
  }

  if (status === 'checking') {
    return (
      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 md:p-7 lg:p-8 xl:p-10 animate-pulse"
        role="status"
        aria-label={t('pages.groups.connection.checking')}
        style={{
          backgroundColor: 'rgba(141, 45, 83, 0.04)',
          borderColor: 'rgba(141, 45, 83, 0.15)',
          backdropFilter: 'blur(12px) saturate(180%)',
          WebkitBackdropFilter: 'blur(12px) saturate(180%)',
        }}
      >
        <div className="flex items-center gap-2 xs:gap-3 md:gap-4 mb-2 xs:mb-3 lg:mb-4">
          <div
            className="p-1.5 xs:p-2 rounded-lg"
            style={{ backgroundColor: 'rgba(141, 45, 83, 0.08)' }}
          >
            <Loader2
              className="w-4 h-4 xs:w-5 xs:h-5 animate-spin"
              style={{ color: 'rgba(141, 45, 83, 0.6)' }}
              aria-hidden="true"
            />
          </div>
          <span
            className="text-sm xs:text-base font-semibold tracking-tight"
            style={{ color: 'rgba(141, 45, 83, 0.7)' }}
          >
            {t('pages.groups.connection.checking')}
          </span>
        </div>
        <p
          className="text-xs xs:text-sm leading-relaxed"
          style={{ color: 'rgba(141, 45, 83, 0.5)' }}
        >
          {t('pages.groups.connection.checkingMessage')}
        </p>
      </div>
    );
  }

  return null;
}
