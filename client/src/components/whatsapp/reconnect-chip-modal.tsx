import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { Progress } from '@/components/ui/progress';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import type { LocalInstance } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { Loader2, RefreshCw, Wifi } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

const COUNTDOWN_SECONDS = 30;
const POLL_INTERVAL_MS = 3000;

const colors = {
  wine: '#8D2D53',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface ReconnectChipModalProps {
  open: boolean;
  onClose: () => void;
  /** The disconnected chip we're trying to bring back. */
  instance: LocalInstance | null;
}

type Phase = 'requesting' | 'awaiting_qr' | 'connected' | 'failed';

/**
 * Modal that re-issues a QR for an EXISTING (disconnected) WhatsApp chip.
 * Unlike AddChipModal it never creates or deletes the chip — it just asks
 * WAHA for a fresh pairing QR via reconnectInstance and polls until the
 * session reaches state=open. Cancelling leaves the chip exactly where it
 * was (still disconnected) so the user can try again later.
 */
export function ReconnectChipModal({ open, onClose, instance }: ReconnectChipModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { reconnectInstance, checkInstanceStatus, upsertInstanceStatus, refetch } =
    useInstancesConnect();

  const [phase, setPhase] = useState<Phase>('requesting');
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [countdown, setCountdown] = useState(COUNTDOWN_SECONDS);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const phaseRef = useRef<Phase>(phase);
  phaseRef.current = phase;

  // Hook callbacks change identity per render. Stash in refs so the
  // polling/countdown effects don't restart on each parent re-render.
  const reconnectRef = useRef(reconnectInstance);
  reconnectRef.current = reconnectInstance;
  const checkStatusRef = useRef(checkInstanceStatus);
  checkStatusRef.current = checkInstanceStatus;
  const upsertStatusRef = useRef(upsertInstanceStatus);
  upsertStatusRef.current = upsertInstanceStatus;
  const refetchRef = useRef(refetch);
  refetchRef.current = refetch;

  const reset = useCallback(() => {
    setPhase('requesting');
    setQrCode(null);
    setCountdown(COUNTDOWN_SECONDS);
    setErrorMessage(null);
  }, []);

  // Bootstrap: when the modal opens, request a fresh QR for the chip.
  // Depends only on `open` + `instance.instance_id` so it doesn't re-fire
  // on transient hook re-renders.
  // biome-ignore lint/correctness/useExhaustiveDependencies: refs intentional
  useEffect(() => {
    if (!open || !instance) {
      reset();
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        const data = await reconnectRef.current(instance.instance_id);
        if (cancelled) return;

        if (!data.success) {
          setPhase('failed');
          setErrorMessage(data.error ?? t('pages.whatsapp.errors.reconnect', 'Falha ao reconectar'));
          return;
        }

        if (data.qrcodeBase64) {
          setQrCode(data.qrcodeBase64);
          setPhase('awaiting_qr');
          setCountdown(COUNTDOWN_SECONDS);
        } else {
          // No QR yet — polling will pick it up.
          setPhase('awaiting_qr');
        }
      } catch (err) {
        if (cancelled) return;
        setPhase('failed');
        setErrorMessage(err instanceof Error ? err.message : 'unknown');
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [open, instance?.instance_id]);

  // Poll status every 3s while waiting for the QR scan.
  // biome-ignore lint/correctness/useExhaustiveDependencies: refs intentional
  useEffect(() => {
    if (!open || phase !== 'awaiting_qr' || !instance) return;
    const name = instance.instance_id;

    const interval = setInterval(async () => {
      if (phaseRef.current !== 'awaiting_qr') return;

      try {
        const data = await checkStatusRef.current(name);
        if (data.state === 'open') {
          setPhase('connected');
          // Persist the new status so the list reflects it even before the
          // background poll picks it up. IMPORTANTE: usar o session name
          // (instance.instance_id) e não o PK (instance.id). O handler
          // UpsertInstanceStatus trata o campo como session name e o repo
          // tem `ON CONFLICT (user_id, instance_id) DO UPDATE` — passar o
          // PK numérico aqui cria um registro fantasma novo no banco em
          // vez de atualizar a instância existente.
          try {
            await upsertStatusRef.current(name, 'connected');
          } catch {
            // refetch below will reconcile from the server anyway.
          }
          await refetchRef.current();
          toast({
            variant: 'success',
            title: t('pages.whatsapp.success.connected', 'WhatsApp conectado'),
            description: t(
              'pages.whatsapp.success.connectedMessage',
              'Seu número foi conectado com sucesso.'
            ),
          });
          setTimeout(() => {
            onClose();
          }, 1200);
        }
      } catch {
        // Silent — next poll may succeed.
      }
    }, POLL_INTERVAL_MS);

    return () => clearInterval(interval);
  }, [open, phase, instance?.instance_id]);

  // QR countdown — refreshes the QR by asking WAHA to re-issue when zero.
  // biome-ignore lint/correctness/useExhaustiveDependencies: refs intentional
  useEffect(() => {
    if (!open || phase !== 'awaiting_qr' || !qrCode || !instance) return;
    const name = instance.instance_id;

    const interval = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          (async () => {
            try {
              const data = await reconnectRef.current(name);
              if (!data.success) return;
              if (data.qrcodeBase64) setQrCode(data.qrcodeBase64);
            } catch {
              // Silent — next countdown will retry.
            }
          })();
          return COUNTDOWN_SECONDS;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(interval);
  }, [open, phase, qrCode, instance?.instance_id]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md">
        <DialogTitle>
          {t('pages.whatsapp.reconnectChipModal.title', 'Reconectar número')}
        </DialogTitle>
        <DialogDescription>
          {t(
            'pages.whatsapp.reconnectChipModal.description',
            'Escaneie o QR code com o WhatsApp deste número para voltar a conectá-lo. Os outros números seguem funcionando normalmente.'
          )}
        </DialogDescription>

        <div className="mt-4">
          {phase === 'requesting' && (
            <div className="flex flex-col items-center py-8">
              <Loader2 className="h-12 w-12 animate-spin mb-3" style={{ color: colors.wine }} />
              <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                {t('pages.whatsapp.reconnectChipModal.requesting', 'Gerando QR code...')}
              </p>
            </div>
          )}

          {phase === 'awaiting_qr' && qrCode && (
            <div className="flex flex-col items-center">
              <div
                className="p-3 rounded-2xl border"
                style={{
                  borderColor: colors.wineBorder,
                  boxShadow: '0 4px 12px rgba(0,0,0,0.08)',
                  backgroundColor: '#ffffff',
                }}
              >
                <img src={qrCode} alt="QR Code" className="w-56 h-56" />
              </div>

              <div className="mt-4 w-full">
                <div className="flex items-center justify-between text-sm mb-1.5">
                  <span style={{ color: colors.wineTextMuted }}>
                    {t('pages.whatsapp.qrCode.expiresIn', { seconds: countdown })}
                  </span>
                  <RefreshCw className="h-3.5 w-3.5" style={{ color: colors.wine }} />
                </div>
                <Progress
                  value={(countdown / COUNTDOWN_SECONDS) * 100}
                  className="h-2"
                  style={{ backgroundColor: 'rgba(141,45,83,0.1)' }}
                />
              </div>

              <p className="text-xs mt-4 text-center" style={{ color: colors.wineTextMuted }}>
                {t('pages.whatsapp.qrCode.waiting')}
              </p>
            </div>
          )}

          {phase === 'awaiting_qr' && !qrCode && (
            <div className="flex flex-col items-center py-8">
              <Loader2 className="h-12 w-12 animate-spin mb-3" style={{ color: colors.wine }} />
              <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                {t('pages.whatsapp.qrCode.generating')}
              </p>
            </div>
          )}

          {phase === 'connected' && (
            <div className="flex flex-col items-center py-8">
              <div
                className="rounded-full p-3 mb-3"
                style={{
                  backgroundColor: 'rgba(34,197,94,0.1)',
                  border: '1px solid rgba(34,197,94,0.2)',
                }}
              >
                <Wifi className="h-8 w-8" style={{ color: '#16a34a' }} />
              </div>
              <p className="text-sm font-medium" style={{ color: '#16a34a' }}>
                {t('pages.whatsapp.success.connectedMessage')}
              </p>
            </div>
          )}

          {phase === 'failed' && (
            <div className="flex flex-col items-center py-8">
              <p className="text-sm" style={{ color: '#dc2626' }}>
                {errorMessage ?? t('pages.whatsapp.errors.reconnect', 'Falha ao reconectar')}
              </p>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
