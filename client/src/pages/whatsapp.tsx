import { ErrorBoundary } from '@/components/error-boundary';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
      <div className="space-y-8">
        <div>
          <h1 className="text-4xl font-bold">{t('pages.whatsapp.title')}</h1>
          <p className="text-muted-foreground mt-2">{t('pages.whatsapp.subtitle')}</p>
        </div>

        <Alert variant="warning">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>{t('pages.whatsapp.warning.title')}</AlertTitle>
          <AlertDescription>{t('pages.whatsapp.warning.message')}</AlertDescription>
        </Alert>

        <div className="flex justify-center">
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

        <HowItWorksCard />
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
    <Card className="shadow-card border-0 w-full max-w-lg">
      <CardHeader className="pb-2">
        <div className="flex items-center gap-4 mb-2">
          <StatusIcon state={state} />
          <div>
            <CardTitle className="text-xl">
              <StatusTitle state={state} />
            </CardTitle>
            <CardDescription>
              <StatusDescription state={state} />
            </CardDescription>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-6">
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
      </CardContent>
    </Card>
  );
}

function StatusIcon({ state }: { state: ConnectionState }) {
  const iconWrapperClass = 'p-3 rounded-lg';

  switch (state) {
    case 'connected':
      return (
        <div className={`${iconWrapperClass} bg-green-500/10`}>
          <CheckCircle2 className="h-8 w-8 text-green-500" />
        </div>
      );
    case 'connecting':
      return (
        <div className={`${iconWrapperClass} bg-primary/10`}>
          <Smartphone className="h-8 w-8 text-primary" />
        </div>
      );
    case 'verifying':
      return (
        <div className={`${iconWrapperClass} bg-muted relative`}>
          <Wifi className="h-8 w-8 text-muted-foreground" />
          <div className="absolute -bottom-1 -right-1 rounded-full bg-background p-0.5">
            <Loader2 className="h-4 w-4 text-muted-foreground animate-spin" />
          </div>
        </div>
      );
    case 'disconnecting':
      return (
        <div className={`${iconWrapperClass} bg-destructive/10 relative`}>
          <Unplug className="h-8 w-8 text-destructive" />
          <div className="absolute -bottom-1 -right-1 rounded-full bg-background p-0.5">
            <Loader2 className="h-4 w-4 text-destructive animate-spin" />
          </div>
        </div>
      );
    case 'error':
      return (
        <div className={`${iconWrapperClass} bg-destructive/10`}>
          <XCircle className="h-8 w-8 text-destructive" />
        </div>
      );
    default:
      return (
        <div className={`${iconWrapperClass} bg-muted`}>
          <WifiOff className="h-8 w-8 text-muted-foreground" />
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
    <div className="flex flex-col items-center py-8">
      <Loader2 className="h-12 w-12 text-destructive animate-spin mb-4" />
      <p className="text-sm text-muted-foreground">
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
      <div className="flex flex-col items-center py-8">
        <Loader2 className="h-12 w-12 text-muted-foreground animate-spin mb-4" />
        <p className="text-sm text-muted-foreground">
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
        className="w-full gap-2 cursor-pointer relative z-10 text-red-700 hover:bg-red-700 hover:text-white"
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
      <Button onClick={onConnect} size="lg" className="w-full gap-2">
        <Wifi className="h-5 w-5" />
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
          <div className="bg-white p-4 rounded-xl shadow-lg">
            <img src={qrCode} alt="QR Code" className="w-64 h-64" />
          </div>

          <div className="mt-4 w-full">
            <div className="flex items-center justify-between text-sm mb-2">
              <span className="text-muted-foreground">
                {t('pages.whatsapp.qrCode.expiresIn', { seconds: qrCountdown })}
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={onRefreshQr}
                className="h-8 gap-1 text-xs cursor-pointer text-red-700 hover:bg-red-700 hover:text-white"
              >
                <RefreshCw className="h-3 w-3" />
                {t('common.refresh')}
              </Button>
            </div>
            <Progress value={(qrCountdown / 30) * 100} className="h-2 bg-muted" />
          </div>

          <p className="text-xs text-muted-foreground mt-4 text-center">
            {t('pages.whatsapp.qrCode.waiting')}
          </p>
        </div>
      ) : (
        <div className="flex flex-col items-center py-8">
          <Loader2 className="h-12 w-12 text-primary animate-spin mb-4" />
          <p className="text-sm text-muted-foreground">{t('pages.whatsapp.qrCode.generating')}</p>
        </div>
      )}

      <Button
        variant="outline"
        onClick={onDisconnect}
        className="w-full gap-2 cursor-pointer text-red-700 hover:bg-red-700 hover:text-white"
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
        <Badge className="bg-gradient-success w-full justify-center py-2">
          <CheckCircle2 className="h-4 w-4 mr-2" />
          {t('pages.whatsapp.success.connectedMessage')}
        </Badge>
        {lastSync && (
          <p className="text-xs text-muted-foreground text-center">
            {t('pages.whatsapp.connected.lastSync')}: {lastSync.toLocaleString()}
          </p>
        )}
      </div>

      {importing && <ImportingProgress progress={importProgress} importedGroups={importedGroups} />}

      <Button
        onClick={onImportGroups}
        variant="outline"
        className="w-full gap-2"
        disabled={importing}
      >
        <RefreshCw className={`h-4 w-4 ${importing ? 'animate-spin' : ''}`} />
        {importing
          ? t('pages.whatsapp.connected.importing')
          : t('pages.whatsapp.connected.syncGroupsManually')}
      </Button>

      <AutoSyncToggle autoSync={autoSync} onToggle={onAutoSyncToggle} />

      <div className="flex gap-2 pt-2">
        <Button
          variant="outline"
          onClick={onReconnect}
          disabled={importing}
          className="flex-1 gap-2 cursor-pointer text-red-700 hover:bg-red-700 hover:text-white"
        >
          <RefreshCw className="h-4 w-4" />
          {t('pages.whatsapp.connected.reconnect')}
        </Button>
        <Button
          variant="outline"
          onClick={onDisconnect}
          className="flex-1 gap-2 cursor-pointer text-red-700 hover:bg-red-700 hover:text-white"
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
        <div className="flex justify-between text-sm">
          <span className="text-muted-foreground">{t('pages.whatsapp.connected.importing')}</span>
          <span className="font-medium">{progress}%</span>
        </div>
        <Progress
          value={progress}
          className={`h-2 border border-border bg-gray-200 ${progress === 0 ? '[&>div]:bg-transparent' : ''}`}
        />
      </div>

      {importedGroups.length > 0 && (
        <div className="space-y-2 max-h-32 overflow-y-auto bg-muted/30 rounded-lg p-3">
          <p className="text-xs font-medium text-muted-foreground">
            {t('pages.whatsapp.connected.importGroups')} ({importedGroups.length}):
          </p>
          {importedGroups.map((group) => (
            <p key={group} className="text-xs text-muted-foreground truncate">
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
    <div className="flex items-center justify-between p-3 bg-muted/30 rounded-lg">
      <div className="space-y-0.5">
        <p className="text-sm font-medium">{t('pages.whatsapp.connected.autoSync.title')}</p>
        <p className="text-xs text-muted-foreground">
          {autoSync
            ? t('pages.whatsapp.connected.autoSync.enabled')
            : t('pages.whatsapp.connected.autoSync.disabled')}
        </p>
      </div>
      <Button
        variant={autoSync ? 'default' : 'outline'}
        size="sm"
        onClick={onToggle}
        className={`cursor-pointer hover:bg-red-700 hover:text-white ${autoSync ? 'text-white' : 'text-red-700'}`}
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
    <Card className="shadow-card border-0 w-full">
      <CardHeader>
        <CardTitle>{t('pages.whatsapp.howItWorks.title')}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {[1, 2, 3, 4, 5, 6].map((step) => (
            <div key={step} className="flex items-start gap-3">
              <div className="shrink-0 w-6 h-6 rounded-full bg-primary/10 flex items-center justify-center">
                <span className="text-xs font-semibold text-primary">{step}</span>
              </div>
              <p className="text-sm text-muted-foreground">
                {t(`pages.whatsapp.howItWorks.steps_${step}`)}
              </p>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
