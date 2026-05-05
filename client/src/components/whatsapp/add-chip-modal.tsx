import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { Progress } from '@/components/ui/progress';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { useWhatsAppConnect } from '@/hooks/use-whatsapp-connect';
import { Loader2, RefreshCw, Wifi } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

const COUNTDOWN_SECONDS = 30;
const POLL_INTERVAL_MS = 3000;

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface AddChipModalProps {
  open: boolean;
  onClose: () => void;
}

type Phase = 'creating' | 'awaiting_qr' | 'connected' | 'failed';

/**
 * Modal that lets the user pair a NEW WhatsApp chip without disturbing
 * any chip that's already connected. Owns its own state machine — does
 * NOT call useWhatsApp (which is singleton and would blow away an existing
 * connected chip via its delete-before-create handshake).
 *
 * Flow:
 *  1. On open, calls CreateInstance with no name (server auto-generates)
 *     and stores the returned QR code.
 *  2. Polls CheckStatus every 3s until state === 'open' or the modal closes.
 *  3. On the QR's 30s countdown reaching 0, refreshes the QR by recreating
 *     the same session (delete + create a fresh one).
 *  4. On manual close before connection, deletes the in-progress chip so
 *     it doesn't dangle in the user's instances list.
 */
export function AddChipModal({ open, onClose }: AddChipModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { createInstance, deleteInstance, checkStatus } = useWhatsAppConnect();
  const { refetch: refetchInstances } = useInstancesConnect();

  const [phase, setPhase] = useState<Phase>('creating');
  const [instanceName, setInstanceName] = useState<string | null>(null);
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [countdown, setCountdown] = useState(COUNTDOWN_SECONDS);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Latest values for async callbacks (avoid stale closures)
  const instanceNameRef = useRef<string | null>(null);
  instanceNameRef.current = instanceName;
  const phaseRef = useRef<Phase>(phase);
  phaseRef.current = phase;

  // Stabilise mutation callbacks: useWhatsAppConnect returns new function
  // identities on every render, which would otherwise re-fire the bootstrap
  // effect into an infinite create-instance loop on transient errors.
  const createInstanceRef = useRef(createInstance);
  createInstanceRef.current = createInstance;
  const deleteInstanceRef = useRef(deleteInstance);
  deleteInstanceRef.current = deleteInstance;
  const checkStatusRef = useRef(checkStatus);
  checkStatusRef.current = checkStatus;

  const reset = useCallback(() => {
    setPhase('creating');
    setInstanceName(null);
    setQrCode(null);
    setCountdown(COUNTDOWN_SECONDS);
    setErrorMessage(null);
  }, []);

  // Bootstrap: when the modal opens, create a fresh instance and grab the QR.
  // Intentionally depends only on `open` — bootstrap should run exactly once
  // per open/close cycle. createInstance/reset/t are read via refs to avoid
  // re-firing on every render (which would loop on transient errors).
  // biome-ignore lint/correctness/useExhaustiveDependencies: see comment above
  useEffect(() => {
    if (!open) {
      reset();
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        const data = await createInstanceRef.current();
        if (cancelled) return;

        if (!data.success) {
          setPhase('failed');
          setErrorMessage(data.error ?? t('pages.whatsapp.errors.connect'));
          return;
        }

        setInstanceName(data.instanceName ?? null);
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
  }, [open]);

  // Poll status every 3s while waiting for the QR scan. Async callbacks read
  // the ref-stabilised hooks so renders triggered by mutation pending state
  // do not restart the interval.
  // biome-ignore lint/correctness/useExhaustiveDependencies: refs intentional
  useEffect(() => {
    if (!open || phase !== 'awaiting_qr' || !instanceName) return;

    const interval = setInterval(async () => {
      if (phaseRef.current !== 'awaiting_qr') return;
      const name = instanceNameRef.current;
      if (!name) return;

      try {
        const data = await checkStatusRef.current(name);
        if (data.qrcodeBase64 && phaseRef.current === 'awaiting_qr') {
          setQrCode(data.qrcodeBase64);
        }
        if (data.state === 'open') {
          setPhase('connected');
          toast({
            variant: 'success',
            title: t('pages.whatsapp.success.connected'),
            description: t('pages.whatsapp.success.connectedMessage'),
          });
          await refetchInstances();
          setTimeout(() => {
            onClose();
          }, 1200);
        }
      } catch {
        // Silent — next poll may succeed.
      }
    }, POLL_INTERVAL_MS);

    return () => clearInterval(interval);
  }, [open, phase, instanceName]);

  // QR countdown — refreshes the chip when it hits zero.
  // biome-ignore lint/correctness/useExhaustiveDependencies: refs intentional
  useEffect(() => {
    if (!open || phase !== 'awaiting_qr' || !qrCode) return;

    const interval = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          (async () => {
            const name = instanceNameRef.current;
            if (!name) return;
            try {
              await deleteInstanceRef.current(name);
              const data = await createInstanceRef.current();
              if (!data.success) return;
              setInstanceName(data.instanceName ?? null);
              if (data.qrcodeBase64) setQrCode(data.qrcodeBase64);
            } catch {
              // Silent — countdown will retry next cycle.
            }
          })();
          return COUNTDOWN_SECONDS;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(interval);
  }, [open, phase, qrCode]);

  // Cleanup on cancel: if the user closes before the chip connected,
  // remove the dangling instance so it doesn't show up as "pending" forever.
  const handleClose = useCallback(async () => {
    const name = instanceNameRef.current;
    const isUnfinished = phaseRef.current === 'creating' || phaseRef.current === 'awaiting_qr';
    if (name && isUnfinished) {
      try {
        await deleteInstanceRef.current(name);
      } catch {
        // Best-effort cleanup; leave it to cleanupInvalidInstances if it fails.
      }
    }
    onClose();
  }, [onClose]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
      <DialogContent className="max-w-md">
        <DialogTitle>
          {t('pages.whatsapp.addChipModal.title', 'Adicionar novo número')}
        </DialogTitle>
        <DialogDescription>
          {t(
            'pages.whatsapp.addChipModal.description',
            'Escaneie o QR code com o WhatsApp do número que você quer adicionar. Os números já conectados continuam funcionando normalmente.'
          )}
        </DialogDescription>

        <div className="mt-4">
          {phase === 'creating' && (
            <div className="flex flex-col items-center py-8">
              <Loader2 className="h-12 w-12 animate-spin mb-3" style={{ color: colors.wine }} />
              <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                {t('pages.whatsapp.addChipModal.creating', 'Gerando QR code...')}
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
                {errorMessage ?? t('pages.whatsapp.errors.connect')}
              </p>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
