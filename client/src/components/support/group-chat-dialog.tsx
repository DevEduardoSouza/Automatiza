import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
import { GetGroupMessagesRequestSchema } from '@/gen/support/v1/support_pb';
import { getGroupMessages } from '@/gen/support/v1/support-SupportService_connectquery';
import { create } from '@bufbuild/protobuf';
import { useQuery } from '@connectrpc/connect-query';
import { Image as ImageIcon, Paperclip } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  userId: string;
  groupId: string;
  groupName?: string;
  /** Chip usado para ler o chat. O grupo pode ter varios; a UI escolhe um. */
  instanceId?: string;
}

function formatTime(unixSeconds: bigint): string {
  const d = new Date(Number(unixSeconds) * 1000);
  return d.toLocaleString('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Modal de chat: mensagens recentes de um grupo, lidas do WAHA.
 *
 * Somente leitura — o painel de suporte nao envia mensagem. O backend limita
 * a janela e desliga o download de midia, entao aqui a midia aparece como
 * marcador, nunca como conteudo.
 */
export function GroupChatDialog({
  open,
  onOpenChange,
  userId,
  groupId,
  groupName,
  instanceId,
}: Props) {
  const { t } = useTranslation();

  const { data, isLoading, error } = useQuery(
    getGroupMessages,
    create(GetGroupMessagesRequestSchema, {
      userId,
      groupId,
      instanceId: instanceId ?? '',
    }),
    // So busca com o modal aberto: e uma chamada ao WAHA, nao a um cache.
    { enabled: open && !!instanceId, staleTime: 15_000 }
  );

  const messages = data?.messages ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85vh] max-w-[calc(100vw-2rem)] flex-col sm:max-w-lg md:max-w-xl">
        <DialogHeader>
          <DialogTitle className="truncate">
            {groupName || t('pages.support.chat.title')}
          </DialogTitle>
          <DialogDescription>
            {instanceId
              ? t('pages.support.chat.readingVia', { chip: instanceId })
              : t('pages.support.chat.noChip')}
          </DialogDescription>
        </DialogHeader>

        {error && (
          <p className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
            {error.message}
          </p>
        )}

        {isLoading && (
          <div className="space-y-3 py-2">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-12 w-2/3 rounded-lg" />
            ))}
          </div>
        )}

        {!isLoading && !error && messages.length === 0 && (
          <p className="py-8 text-center text-sm text-muted-foreground">
            {t('pages.support.chat.empty')}
          </p>
        )}

        {messages.length > 0 && (
          <ScrollArea className="min-h-0 flex-1 pr-3">
            <div className="space-y-2 py-1">
              {messages.map((m) => (
                <div key={m.id} className={`flex ${m.fromMe ? 'justify-end' : 'justify-start'}`}>
                  <div
                    className={`max-w-[80%] rounded-2xl px-3 py-2 text-sm ${
                      m.fromMe
                        ? 'rounded-br-sm bg-primary/10 text-foreground'
                        : 'rounded-bl-sm bg-muted'
                    }`}
                  >
                    {!m.fromMe && m.author && (
                      <p className="mb-0.5 text-xs font-medium text-muted-foreground">{m.author}</p>
                    )}
                    {m.hasMedia && (
                      <p className="mb-1 inline-flex items-center gap-1 text-xs text-muted-foreground">
                        {m.mediaType === 'image' ? (
                          <ImageIcon className="h-3.5 w-3.5" />
                        ) : (
                          <Paperclip className="h-3.5 w-3.5" />
                        )}
                        {t('pages.support.chat.media', { type: m.mediaType || '' })}
                      </p>
                    )}
                    {m.body && <p className="whitespace-pre-wrap break-words">{m.body}</p>}
                    <p className="mt-1 text-right text-[10px] text-muted-foreground">
                      {formatTime(m.timestamp)}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </ScrollArea>
        )}
      </DialogContent>
    </Dialog>
  );
}
