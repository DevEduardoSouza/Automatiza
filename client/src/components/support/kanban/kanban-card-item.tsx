import type { KanbanCard } from '@/gen/support/v1/support_pb';
import { useDraggable, useDroppable } from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import { Paperclip } from 'lucide-react';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { PRIORITY_DOT, type Priority } from './kanban-shared';

/**
 * Um card arrastável do quadro. Distingue clique de arrasto pela distância de
 * ativação do sensor (configurada no board): um clique sem mover abre o modal;
 * arrastar move o card. O `overlay` desenha a versão "fantasma" no DragOverlay
 * (sem listeners, sem virar transparente).
 *
 * É também um droppable (mesmo id do draggable): ao soltar sobre um card, o
 * board insere o arrastado ANTES dele — é isso que dá a reordenação fina dentro
 * da coluna, além do mover-para-o-fim ao soltar no corpo vazio da coluna.
 */
interface Props {
  card: KanbanCard;
  onOpen: (card: KanbanCard) => void;
  overlay?: boolean;
}

export function KanbanCardItem({ card, onOpen, overlay }: Props) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: card.id,
    data: { columnKey: card.columnKey },
  });
  const { setNodeRef: setDropRef } = useDroppable({
    id: card.id,
    data: { columnKey: card.columnKey },
  });

  // Um mesmo nó DOM serve de draggable e droppable — os dois refs precisam
  // apontar pra ele. No overlay não há droppable (é só a cópia fantasma).
  const setRefs = useCallback(
    (node: HTMLElement | null) => {
      setNodeRef(node);
      if (!overlay) setDropRef(node);
    },
    [setNodeRef, setDropRef, overlay]
  );

  const priority = (card.priority as Priority) || 'medium';
  const attachmentCount = card.attachments?.length ?? 0;

  const style = overlay
    ? undefined
    : {
        transform: CSS.Translate.toString(transform),
        // Enquanto arrasta, o card original vira um "buraco" translúcido; o
        // DragOverlay mostra a cópia que segue o cursor.
        opacity: isDragging ? 0.4 : 1,
      };

  return (
    <button
      type="button"
      ref={setRefs}
      style={style}
      onClick={() => !isDragging && onOpen(card)}
      className={`w-full rounded-lg border bg-card p-2.5 text-left shadow-sm transition hover:border-primary/50 ${
        overlay ? 'cursor-grabbing shadow-lg ring-1 ring-primary/40' : 'cursor-grab'
      }`}
      {...listeners}
      {...attributes}
    >
      <div className="flex items-start gap-2">
        <span
          className={`mt-1 h-2 w-2 shrink-0 rounded-full ${PRIORITY_DOT[priority]}`}
          title={t(`pages.support.kanban.priority.${priority}`)}
        />
        <div className="min-w-0 flex-1">
          <p className="break-words text-sm font-medium leading-snug">{card.title}</p>
          {card.description ? (
            <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{card.description}</p>
          ) : null}
          {attachmentCount > 0 ? (
            <span className="mt-1.5 inline-flex items-center gap-1 text-[10px] text-muted-foreground">
              <Paperclip className="h-3 w-3" />
              {attachmentCount}
            </span>
          ) : null}
        </div>
      </div>
    </button>
  );
}
