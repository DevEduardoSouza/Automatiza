import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { useWhatsAppConnect } from '@/hooks/use-whatsapp-connect';
import { KeyRound, Loader2, QrCode, RefreshCw, Wifi } from 'lucide-react';
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
  const { createInstance, deleteInstance, checkStatus, requestPairingCode } = useWhatsAppConnect();
  const { refetch: refetchInstances } = useInstancesConnect();

  const [phase, setPhase] = useState<Phase>('creating');
  const [instanceName, setInstanceName] = useState<string | null>(null);
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [countdown, setCountdown] = useState(COUNTDOWN_SECONDS);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Pairing-by-code (alternative to QR). QR stays the default so the existing
  // flow — and any cached older frontend — is untouched.
  const [authMode, setAuthMode] = useState<'qr' | 'code'>('qr');
  const [phoneNumber, setPhoneNumber] = useState('');
  const [pairingCode, setPairingCode] = useState<string | null>(null);
  const [codeError, setCodeError] = useState<string | null>(null);
  const [requestingCode, setRequestingCode] = useState(false);

  // Latest values for async callbacks (avoid stale closures)
  const instanceNameRef = useRef<string | null>(null);
  instanceNameRef.current = instanceName;
  const phaseRef = useRef<Phase>(phase);
  phaseRef.current = phase;
  // Read inside the QR-recycle interval: in code mode we must NOT recycle the
  // session, or WAHA invalidates the pairing code the user is typing.
  const authModeRef = useRef<'qr' | 'code'>(authMode);
  authModeRef.current = authMode;

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
    setAuthMode('qr');
    setPhoneNumber('');
    setPairingCode(null);
    setCodeError(null);
    setRequestingCode(false);
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
        // In code mode, skip the QR fetch: we only need the connection state,
        // and re-reading the QR each poll would churn the pairing session.
        const data = await checkStatusRef.current(name, authModeRef.current === 'code');
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
  useEffect(() => {
    // QR mode only: the recycle recreates the session (which would invalidate a
    // pairing code mid-entry), and code mode has no countdown to tick — so the
    // interval must not even be created there (no wasted 1s timer).
    if (!open || phase !== 'awaiting_qr' || !qrCode || authMode === 'code') return;

    const interval = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          (async () => {
            const name = instanceNameRef.current;
            if (!name) return;
            try {
              // Guarda de corrida: o usuário pode ter escaneado o QR no exato
              // segundo em que o contador zera. Sem esta checagem, o recycle
              // deletava uma sessão recém-PAREADA (o waha-router bloqueia e o
              // número é salvo, mas o fluxo quebra e dispara alerta de unpair).
              // Se já conectou, não recicla — o poll de status assume e leva a
              // fase para "connected".
              try {
                const st = await checkStatusRef.current(name);
                if (st.state === 'open') return;
              } catch {
                // Status indisponível — segue o recycle normal.
              }
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
  }, [open, phase, qrCode, authMode]);

  // Cleanup on cancel: if the user closes before the chip connected,
  // remove the dangling instance so it doesn't show up as "pending" forever.
  const handleClose = useCallback(async () => {
    const name = instanceNameRef.current;
    const isUnfinished = phaseRef.current === 'creating' || phaseRef.current === 'awaiting_qr';
    if (name && isUnfinished) {
      try {
        // Mesma guarda de corrida do recycle: se o scan acabou de conectar
        // (fase ainda não atualizou), fechar o modal NÃO pode deletar o chip
        // recém-pareado.
        let connected = false;
        try {
          const st = await checkStatusRef.current(name);
          connected = st.state === 'open';
        } catch {
          // Status indisponível — mantém o cleanup best-effort.
        }
        if (!connected) {
          await deleteInstanceRef.current(name);
        }
      } catch {
        // Best-effort cleanup; leave it to cleanupInvalidInstances if it fails.
      }
    }
    onClose();
  }, [onClose]);

  // Request a phone pairing code for the already-created session (code mode).
  // The status poll keeps running and flips the phase to "connected" once the
  // user confirms the code in WhatsApp — same completion path as the QR scan.
  const handleRequestCode = useCallback(async () => {
    const name = instanceNameRef.current;
    if (!name) return;
    const digits = phoneNumber.replace(/\D/g, '');
    if (digits.length < 8) {
      setCodeError(
        t(
          'pages.whatsapp.addChipModal.codeInvalidPhone',
          'Informe o número com DDI + DDD (ex.: 55 31 99999-8888).'
        )
      );
      return;
    }
    setRequestingCode(true);
    setCodeError(null);
    try {
      const res = await requestPairingCode({ instanceName: name, phoneNumber: digits });
      if (res.success && res.code) {
        setPairingCode(res.code);
      } else {
        setCodeError(
          res.error ??
            t(
              'pages.whatsapp.addChipModal.codeError',
              'Não foi possível gerar o código. Tente novamente.'
            )
        );
      }
    } catch {
      setCodeError(
        t(
          'pages.whatsapp.addChipModal.codeError',
          'Não foi possível gerar o código. Tente novamente.'
        )
      );
    } finally {
      setRequestingCode(false);
    }
  }, [phoneNumber, requestPairingCode, t]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
      <DialogContent className="max-w-md">
        <DialogTitle>{t('pages.whatsapp.addChipModal.title', 'Adicionar novo número')}</DialogTitle>
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

          {phase === 'awaiting_qr' && (
            <div className="flex flex-col items-center">
              {authMode === 'qr' ? (
                qrCode ? (
                  <>
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
                  </>
                ) : (
                  <div className="flex flex-col items-center py-8">
                    <Loader2
                      className="h-12 w-12 animate-spin mb-3"
                      style={{ color: colors.wine }}
                    />
                    <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                      {t('pages.whatsapp.qrCode.generating')}
                    </p>
                  </div>
                )
              ) : pairingCode ? (
                <div className="flex flex-col items-center py-2 w-full">
                  <p className="text-sm mb-3 text-center" style={{ color: colors.wineTextMuted }}>
                    {t(
                      'pages.whatsapp.addChipModal.codeInstructions',
                      'No celular abra WhatsApp › Aparelhos conectados › Conectar com número de telefone, e digite:'
                    )}
                  </p>
                  <div
                    className="px-5 py-3 rounded-xl border font-mono text-2xl font-semibold tracking-[0.25em]"
                    style={{
                      borderColor: colors.wineBorder,
                      backgroundColor: colors.wineBg,
                      color: colors.wine,
                    }}
                  >
                    {pairingCode}
                  </div>
                  <p className="text-xs mt-4 text-center" style={{ color: colors.wineTextMuted }}>
                    {t('pages.whatsapp.qrCode.waiting')}
                  </p>
                  <button
                    type="button"
                    onClick={() => setPairingCode(null)}
                    className="mt-3 text-xs underline"
                    style={{ color: colors.wine }}
                  >
                    {t('pages.whatsapp.addChipModal.codeRegenerate', 'Gerar outro código')}
                  </button>
                </div>
              ) : (
                <div className="flex w-full flex-col gap-3">
                  <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                    {t(
                      'pages.whatsapp.addChipModal.codePrompt',
                      'Digite o número do WhatsApp que você quer conectar (com DDI e DDD):'
                    )}
                  </p>
                  <Input
                    type="tel"
                    inputMode="numeric"
                    placeholder="55 31 99999-8888"
                    value={phoneNumber}
                    onChange={(e) => setPhoneNumber(e.target.value)}
                    disabled={requestingCode}
                  />
                  {codeError && (
                    <p className="text-xs" style={{ color: '#dc2626' }}>
                      {codeError}
                    </p>
                  )}
                  <Button
                    onClick={handleRequestCode}
                    disabled={requestingCode}
                    style={{ backgroundColor: colors.wine }}
                  >
                    {requestingCode ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      t('pages.whatsapp.addChipModal.codeGenerate', 'Gerar código')
                    )}
                  </Button>
                </div>
              )}

              <button
                type="button"
                onClick={() => {
                  // Clear any issued code on switch: it belongs to the session
                  // state we're leaving and would be stale on return (a recycle in
                  // QR mode can replace the session under it).
                  setCodeError(null);
                  setPairingCode(null);
                  setAuthMode((m) => (m === 'qr' ? 'code' : 'qr'));
                }}
                className="mt-5 flex items-center gap-1.5 text-xs underline"
                style={{ color: colors.wine }}
              >
                {authMode === 'qr' ? (
                  <>
                    <KeyRound className="h-3.5 w-3.5" />
                    {t(
                      'pages.whatsapp.addChipModal.switchToCode',
                      'Conectar com código em vez do QR'
                    )}
                  </>
                ) : (
                  <>
                    <QrCode className="h-3.5 w-3.5" />
                    {t('pages.whatsapp.addChipModal.switchToQr', 'Usar o QR code')}
                  </>
                )}
              </button>
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
