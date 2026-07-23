import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import type { FailoverEvent } from '@/gen/failover/v1/failover_pb';
import { useFailover } from '@/hooks/use-failover';
import { ChevronDown } from 'lucide-react';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface FailoverEventListProps {
  policyId: bigint;
}

function formatWhen(iso: string): string {
  const ts = Date.parse(iso);
  if (!Number.isFinite(ts)) return iso;
  return new Date(ts).toLocaleString('pt-BR');
}

/**
 * Histórico da política (ledger append-only).
 *
 * Carregado sob demanda: é auditoria, não algo que o usuário precise ver a cada
 * render — e buscar sempre custaria uma query por política em cada poll de 2s
 * durante uma troca.
 */
export function FailoverEventList({ policyId }: FailoverEventListProps) {
  const { t } = useTranslation();
  const { listEvents } = useFailover();
  const [events, setEvents] = useState<FailoverEvent[]>([]);
  const [open, setOpen] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const handleOpen = useCallback(
    async (next: boolean) => {
      setOpen(next);
      if (next && !loaded) {
        try {
          setEvents(await listEvents(policyId, 20));
        } catch {
          // Histórico é auxiliar: falhar aqui não pode derrubar o card.
          setEvents([]);
        }
        setLoaded(true);
      }
    },
    [listEvents, loaded, policyId]
  );

  return (
    <Collapsible open={open} onOpenChange={handleOpen}>
      <CollapsibleTrigger className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
        <ChevronDown
          className={`h-3 w-3 transition-transform ${open ? 'rotate-180' : ''}`}
          aria-hidden="true"
        />
        {t('pages.whatsapp.failover.history')}
      </CollapsibleTrigger>

      <CollapsibleContent>
        <ul className="mt-2 space-y-1">
          {events.map((ev) => (
            <li key={String(ev.id)} className="text-xs text-muted-foreground">
              <span className="font-mono">{formatWhen(ev.createdAt)}</span>{' '}
              {t(`pages.whatsapp.failover.event.${ev.eventType}`, { defaultValue: ev.eventType })}
            </li>
          ))}
          {loaded && events.length === 0 && (
            <li className="text-xs text-muted-foreground">
              {t('pages.whatsapp.failover.noHistory')}
            </li>
          )}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  );
}
