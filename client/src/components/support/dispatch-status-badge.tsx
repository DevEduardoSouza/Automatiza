import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useTranslation } from 'react-i18next';

/**
 * Sinais que o backend usa para decidir se uma automacao esta disparando.
 * Cada um responde a uma pergunta diferente — ver dispatchState() no
 * support_handler.go.
 */
export type DispatchSignal = 'run_active' | 'recent_dispatch' | 'event_driven_idle' | 'idle' | '';

interface Props {
  isDispatching: boolean;
  signal: string;
}

/**
 * Badge de status de disparo.
 *
 * O ponto sutil: um espelho/monitor "parado" NAO significa quebrado — essas
 * automacoes sao event-driven e so disparam quando o grupo-fonte fala. Por
 * isso o estado ocioso event-driven tem rotulo e tooltip proprios, em vez de
 * um "parado" vermelho que mandaria o atendente cacar um defeito inexistente.
 */
export function DispatchStatusBadge({ isDispatching, signal }: Props) {
  const { t } = useTranslation();

  if (isDispatching) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge className="gap-1.5 border-transparent bg-emerald-100 text-emerald-800 hover:bg-emerald-100">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-500 opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-600" />
            </span>
            {t('pages.support.dispatch.dispatching')}
          </Badge>
        </TooltipTrigger>
        <TooltipContent>
          {signal === 'run_active'
            ? t('pages.support.dispatch.tip.runActive')
            : t('pages.support.dispatch.tip.recentDispatch')}
        </TooltipContent>
      </Tooltip>
    );
  }

  if (signal === 'event_driven_idle') {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge variant="secondary" className="gap-1.5">
            <span className="h-2 w-2 rounded-full bg-muted-foreground/50" />
            {t('pages.support.dispatch.waitingSource')}
          </Badge>
        </TooltipTrigger>
        <TooltipContent className="max-w-xs">
          {t('pages.support.dispatch.tip.eventDriven')}
        </TooltipContent>
      </Tooltip>
    );
  }

  return (
    <Badge variant="outline" className="gap-1.5">
      <span className="h-2 w-2 rounded-full bg-muted-foreground/40" />
      {t('pages.support.dispatch.idle')}
    </Badge>
  );
}
