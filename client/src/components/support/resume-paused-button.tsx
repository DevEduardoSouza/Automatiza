import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/ui/toast-utils';
import { ResumePausedAutomationsRequestSchema } from '@/gen/support/v1/support_pb';
import { resumePausedAutomations } from '@/gen/support/v1/support-SupportService_connectquery';
import { create } from '@bufbuild/protobuf';
import { useMutation } from '@connectrpc/connect-query';
import { RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

interface Props {
  /** Chamado após religar, para o dashboard recarregar os números. */
  onDone?: () => void;
}

/**
 * Botão que religa automações pausadas SEM motivo protetor.
 *
 * É a única ação de escrita do painel, e é o espelho do incidente de mass-pause
 * de 16/07 — por isso passa por uma confirmação que primeiro mostra (dry-run)
 * exatamente quantas serão religadas e quantas o backend vai PULAR (cobrança e
 * erro recente), para o atendente nunca disparar às cegas.
 */
export function ResumePausedButton({ onDone }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [preview, setPreview] = useState<{
    resumed: number;
    skippedBilling: number;
    skippedError: number;
  } | null>(null);

  const mutation = useMutation(resumePausedAutomations);

  const openDialog = async () => {
    setPreview(null);
    setOpen(true);
    try {
      // dry-run: conta sem religar, para a confirmação mostrar o impacto real.
      const res = await mutation.mutateAsync(
        create(ResumePausedAutomationsRequestSchema, { dryRun: true })
      );
      setPreview({
        resumed: res.resumed,
        skippedBilling: res.skippedBilling,
        skippedError: res.skippedError,
      });
    } catch (e) {
      toast.error((e as Error).message);
      setOpen(false);
    }
  };

  const confirm = async () => {
    try {
      const res = await mutation.mutateAsync(
        create(ResumePausedAutomationsRequestSchema, { dryRun: false })
      );
      toast.success(t('pages.support.resume.done', { n: res.resumed }));
      setOpen(false);
      onDone?.();
    } catch (e) {
      toast.error((e as Error).message);
    }
  };

  return (
    <>
      <Button variant="outline" size="sm" className="gap-1.5" onClick={openDialog}>
        <RotateCcw className="h-3.5 w-3.5" />
        {t('pages.support.resume.button')}
      </Button>

      <AlertDialog open={open} onOpenChange={setOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('pages.support.resume.title')}</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className="space-y-2 text-sm">
                {preview === null ? (
                  <p>{t('pages.support.resume.loading')}</p>
                ) : (
                  <>
                    <p className="text-foreground">
                      {t('pages.support.resume.willResume', { n: preview.resumed })}
                    </p>
                    {(preview.skippedBilling > 0 || preview.skippedError > 0) && (
                      <div className="rounded-lg bg-muted p-2 text-xs text-muted-foreground">
                        <p className="mb-1 font-medium">{t('pages.support.resume.willSkip')}</p>
                        {preview.skippedBilling > 0 && (
                          <p>
                            {t('pages.support.resume.skipBilling', { n: preview.skippedBilling })}
                          </p>
                        )}
                        {preview.skippedError > 0 && (
                          <p>{t('pages.support.resume.skipError', { n: preview.skippedError })}</p>
                        )}
                      </div>
                    )}
                    {preview.resumed === 0 && (
                      <p className="text-xs text-muted-foreground">
                        {t('pages.support.resume.nothingToDo')}
                      </p>
                    )}
                  </>
                )}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('pages.support.resume.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                confirm();
              }}
              disabled={mutation.isPending || preview === null || preview.resumed === 0}
            >
              {t('pages.support.resume.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
