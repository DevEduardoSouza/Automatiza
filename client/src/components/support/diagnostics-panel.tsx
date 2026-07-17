import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { Diagnosis, Severity } from '@/components/support/diagnostics';
import { Check, CheckCircle2, ClipboardCopy, Info, ShieldAlert, Wrench } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

/** Cores e ícone por severidade — vermelho grita, cinza sussurra. */
const SEVERITY_STYLE: Record<Severity, { border: string; icon: string; Icon: typeof Info }> = {
  high: { border: 'border-l-red-500', icon: 'text-red-600', Icon: ShieldAlert },
  medium: { border: 'border-l-amber-500', icon: 'text-amber-600', Icon: ShieldAlert },
  low: { border: 'border-l-yellow-400', icon: 'text-yellow-600', Icon: Info },
  info: { border: 'border-l-muted-foreground/30', icon: 'text-muted-foreground', Icon: Info },
};

interface Props {
  diagnoses: Diagnosis[];
  /** Enquanto o overview carrega, não afirmamos "está tudo bem". */
  loading?: boolean;
}

/**
 * Painel de diagnóstico: a primeira coisa que o atendente lê.
 *
 * Ordenado por gravidade, cada item diz o problema em linguagem de cliente, a
 * ação que resolve, e — o que mais importa para um suporte não técnico — se é
 * algo que ELE resolve com o cliente ou que precisa ir para o time técnico.
 */
export function DiagnosticsPanel({ diagnoses, loading }: Props) {
  const { t } = useTranslation();
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const copy = async (d: Diagnosis) => {
    const text = `Problema: ${d.title}\n${d.detail}\nO que fazer: ${d.action}`;
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(d.id);
      setTimeout(() => setCopiedId((cur) => (cur === d.id ? null : cur)), 2000);
    } catch {
      // Clipboard bloqueado (contexto inseguro): silencioso, não é crítico.
    }
  };

  if (loading) return null;

  if (diagnoses.length === 0) {
    return (
      <div className="flex items-center gap-3 rounded-xl border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900/50 dark:bg-emerald-950/30">
        <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-600" />
        <div>
          <p className="text-sm font-medium text-emerald-900 dark:text-emerald-200">
            {t('pages.support.diag.healthyTitle')}
          </p>
          <p className="text-xs text-emerald-700 dark:text-emerald-400">
            {t('pages.support.diag.healthySubtitle')}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">{t('pages.support.diag.title')}</h2>
        <span className="text-xs text-muted-foreground">
          {t('pages.support.diag.count', { count: diagnoses.length })}
        </span>
      </div>

      {diagnoses.map((d) => {
        const s = SEVERITY_STYLE[d.severity];
        return (
          <div key={d.id} className={`rounded-xl border border-l-4 bg-background p-4 ${s.border}`}>
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="flex min-w-0 items-start gap-2">
                <s.Icon className={`mt-0.5 h-4 w-4 shrink-0 ${s.icon}`} />
                <p className="font-medium leading-tight">{d.title}</p>
              </div>
              {d.resolution === 'support' ? (
                <Badge className="shrink-0 gap-1 border-transparent bg-emerald-100 text-emerald-800 hover:bg-emerald-100">
                  <Wrench className="h-3 w-3" />
                  {t('pages.support.diag.resolvable')}
                </Badge>
              ) : (
                <Badge className="shrink-0 gap-1 border-transparent bg-amber-100 text-amber-800 hover:bg-amber-100">
                  <ShieldAlert className="h-3 w-3" />
                  {t('pages.support.diag.escalate')}
                </Badge>
              )}
            </div>

            <p className="mt-2 pl-6 text-sm text-muted-foreground">{d.detail}</p>

            <div className="mt-2 flex flex-wrap items-start justify-between gap-2 pl-6">
              <p className="min-w-0 text-sm">
                <span className="font-medium">{t('pages.support.diag.whatToDo')}: </span>
                {d.action}
              </p>
              <Button
                variant="ghost"
                size="sm"
                className="h-7 shrink-0 gap-1 text-xs"
                onClick={() => copy(d)}
              >
                {copiedId === d.id ? (
                  <>
                    <Check className="h-3.5 w-3.5" />
                    {t('pages.support.diag.copied')}
                  </>
                ) : (
                  <>
                    <ClipboardCopy className="h-3.5 w-3.5" />
                    {t('pages.support.diag.copy')}
                  </>
                )}
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
