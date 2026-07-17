import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from '@/components/ui/toast-utils';
import {
  createKanbanCard,
  deleteKanbanAttachment,
  deleteKanbanCard,
  downloadKanbanAttachment,
  listKanbanCards,
  moveKanbanCard,
  updateKanbanCard,
  uploadKanbanAttachment,
} from '@/gen/support/v1/support-SupportService_connectquery';
import {
  CreateKanbanCardRequestSchema,
  DeleteKanbanAttachmentRequestSchema,
  DeleteKanbanCardRequestSchema,
  DownloadKanbanAttachmentRequestSchema,
  type KanbanAttachment,
  type KanbanCard,
  MoveKanbanCardRequestSchema,
  UpdateKanbanCardRequestSchema,
  UploadKanbanAttachmentRequestSchema,
} from '@/gen/support/v1/support_pb';
import { create } from '@bufbuild/protobuf';
import { useMutation, useQuery } from '@connectrpc/connect-query';
import {
  DndContext,
  type DragEndEvent,
  DragOverlay,
  type DragStartEvent,
  PointerSensor,
  closestCorners,
  useDroppable,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { AlertTriangle, Plus } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { KanbanCardDialog } from './kanban-card-dialog';
import { KanbanCardItem } from './kanban-card-item';
import { COLUMN_META, type ColumnKey, POSITION_STEP, type Priority } from './kanban-shared';

/**
 * Quadro Kanban (estilo Trello) da aba de suporte: 5 colunas fixas, cards
 * arrastáveis entre elas, com título/descrição livres, prioridade e anexos.
 *
 * A ordenação usa posições float: ao soltar entre dois cards, a nova posição é
 * o ponto médio das vizinhas — nunca renumera a coluna inteira. O estado local
 * é atualizado de forma otimista no arrasto e reconciliado com o refetch.
 */
export function Kanban() {
  const { t } = useTranslation();
  const { data, isLoading, error, refetch } = useQuery(listKanbanCards, {}, { staleTime: 0 });

  const [cards, setCards] = useState<KanbanCard[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [createColumn, setCreateColumn] = useState<ColumnKey>('corrigir');

  // Espelha a query no estado local (base para o arrasto otimista).
  useEffect(() => {
    if (data?.cards) setCards(data.cards);
  }, [data?.cards]);

  const createMut = useMutation(createKanbanCard);
  const updateMut = useMutation(updateKanbanCard);
  const moveMut = useMutation(moveKanbanCard);
  const deleteMut = useMutation(deleteKanbanCard);
  const uploadMut = useMutation(uploadKanbanAttachment);
  const downloadMut = useMutation(downloadKanbanAttachment);
  const deleteAttMut = useMutation(deleteKanbanAttachment);

  const sensors = useSensors(
    // 5px de tolerância separa clique (abre o card) de arrasto (move o card).
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } })
  );

  const byColumn = useMemo(() => {
    const map: Record<ColumnKey, KanbanCard[]> = {
      corrigir: [],
      aguardando: [],
      em_andamento: [],
      em_revisao: [],
      corrigido: [],
    };
    for (const c of cards) {
      const key = c.columnKey as ColumnKey;
      if (map[key]) map[key].push(c);
    }
    for (const k of Object.keys(map) as ColumnKey[]) {
      map[k].sort((a, b) => a.position - b.position);
    }
    return map;
  }, [cards]);

  const activeCard = activeId ? (cards.find((c) => c.id === activeId) ?? null) : null;
  const selectedCard = selectedId ? (cards.find((c) => c.id === selectedId) ?? null) : null;

  const onDragStart = (e: DragStartEvent) => setActiveId(String(e.active.id));

  const onDragEnd = async (e: DragEndEvent) => {
    const { active, over } = e;
    setActiveId(null);
    if (!over) return;
    const activeCardId = String(active.id);
    const overId = String(over.id);
    if (overId === activeCardId) return;

    const dragged = cards.find((c) => c.id === activeCardId);
    if (!dragged) return;

    // Resolve a coluna e o vizinho de destino.
    let targetColumn: ColumnKey;
    let overCardId: string | null = null;
    if (overId.startsWith('col:')) {
      targetColumn = overId.slice(4) as ColumnKey;
    } else {
      const overCard = cards.find((c) => c.id === overId);
      if (!overCard) return;
      targetColumn = overCard.columnKey as ColumnKey;
      overCardId = overCard.id;
    }

    const colCards = cards
      .filter((c) => c.columnKey === targetColumn && c.id !== activeCardId)
      .sort((a, b) => a.position - b.position);

    let index = colCards.length;
    if (overCardId) {
      const i = colCards.findIndex((c) => c.id === overCardId);
      if (i >= 0) index = i;
    }
    const newPos = midpoint(colCards, index);

    // Nada mudou (mesmo lugar) → não gasta ida ao servidor.
    if (dragged.columnKey === targetColumn && dragged.position === newPos) return;

    // Otimista: aplica localmente já.
    setCards((prev) =>
      prev.map((c) =>
        c.id === activeCardId ? { ...c, columnKey: targetColumn, position: newPos } : c
      )
    );

    try {
      await moveMut.mutateAsync(
        create(MoveKanbanCardRequestSchema, {
          id: activeCardId,
          columnKey: targetColumn,
          position: newPos,
        })
      );
      await refetch();
    } catch (err) {
      toast.error((err as Error).message);
      await refetch(); // reverte para a verdade do servidor
    }
  };

  const openCreate = (column: ColumnKey) => {
    setCreateColumn(column);
    setSelectedId(null);
    setDialogOpen(true);
  };
  const openCard = (card: KanbanCard) => {
    setSelectedId(card.id);
    setDialogOpen(true);
  };

  // --- callbacks do dialog (detêm as mutations) ---
  const onCreate = async (input: {
    title: string;
    description: string;
    columnKey: ColumnKey;
    priority: Priority;
  }) => {
    const res = await createMut.mutateAsync(create(CreateKanbanCardRequestSchema, input));
    await refetch();
    toast.success(t('pages.support.kanban.created'));
    return res.card as KanbanCard;
  };
  const onUpdate = async (input: {
    id: string;
    title: string;
    description: string;
    priority: Priority;
  }) => {
    await updateMut.mutateAsync(create(UpdateKanbanCardRequestSchema, input));
    await refetch();
  };
  const onDelete = async (id: string) => {
    await deleteMut.mutateAsync(create(DeleteKanbanCardRequestSchema, { id }));
    await refetch();
    toast.success(t('pages.support.kanban.deleted'));
  };
  const onUpload = async (cardId: string, file: File) => {
    const buf = new Uint8Array(await file.arrayBuffer());
    await uploadMut.mutateAsync(
      create(UploadKanbanAttachmentRequestSchema, {
        cardId,
        filename: file.name,
        contentType: file.type || 'application/octet-stream',
        data: buf,
      })
    );
    await refetch();
    toast.success(t('pages.support.kanban.form.fileAdded'));
  };
  const onDownload = async (att: KanbanAttachment) => {
    const res = await downloadMut.mutateAsync(
      create(DownloadKanbanAttachmentRequestSchema, { id: att.id })
    );
    const blob = new Blob([res.data as BlobPart], {
      type: res.contentType || 'application/octet-stream',
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = res.filename || att.filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };
  const onDeleteAttachment = async (id: string) => {
    await deleteAttMut.mutateAsync(create(DeleteKanbanAttachmentRequestSchema, { id }));
    await refetch();
  };

  if (error) {
    return (
      <div className="mx-auto max-w-6xl p-4">
        <div className="flex items-center gap-2 rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-700">
          <AlertTriangle className="h-4 w-4" /> {error.message}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">
            {t('pages.support.kanban.title')}
          </h1>
          <p className="text-sm text-muted-foreground">{t('pages.support.kanban.subtitle')}</p>
        </div>
        <Button size="sm" className="gap-1.5" onClick={() => openCreate('corrigir')}>
          <Plus className="h-3.5 w-3.5" />
          {t('pages.support.kanban.newCard')}
        </Button>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-3 xl:grid-cols-5">
          {COLUMN_META.map((c) => (
            <Skeleton key={c.key} className="h-64 rounded-xl" />
          ))}
        </div>
      ) : (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCorners}
          onDragStart={onDragStart}
          onDragEnd={onDragEnd}
        >
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3 xl:grid-cols-5">
            {COLUMN_META.map((meta) => (
              <Column
                key={meta.key}
                columnKey={meta.key}
                label={t(meta.labelKey)}
                accent={meta.accent}
                cards={byColumn[meta.key]}
                onOpenCard={openCard}
                onAdd={() => openCreate(meta.key)}
              />
            ))}
          </div>
          <DragOverlay>
            {activeCard ? <KanbanCardItem card={activeCard} onOpen={() => {}} overlay /> : null}
          </DragOverlay>
        </DndContext>
      )}

      <KanbanCardDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        card={selectedCard}
        defaultColumn={createColumn}
        onCreate={onCreate}
        onUpdate={onUpdate}
        onDelete={onDelete}
        onUpload={onUpload}
        onDownload={onDownload}
        onDeleteAttachment={onDeleteAttachment}
      />
    </div>
  );
}

/** Uma coluna do quadro — área de soltura (droppable) + pilha de cards. */
function Column({
  columnKey,
  label,
  accent,
  cards,
  onOpenCard,
  onAdd,
}: {
  columnKey: ColumnKey;
  label: string;
  accent: string;
  cards: KanbanCard[];
  onOpenCard: (c: KanbanCard) => void;
  onAdd: () => void;
}) {
  const { t } = useTranslation();
  const { setNodeRef, isOver } = useDroppable({ id: `col:${columnKey}` });

  return (
    <div
      ref={setNodeRef}
      className={`flex min-h-64 flex-col rounded-xl border bg-muted/30 transition ${
        isOver ? 'ring-2 ring-primary/40' : ''
      }`}
    >
      <div className="flex items-center justify-between gap-2 rounded-t-xl px-3 py-2">
        <div className="flex items-center gap-2">
          <span className={`h-2.5 w-2.5 rounded-full ${accent}`} />
          <span className="text-sm font-semibold">{label}</span>
          <span className="rounded-full bg-muted px-1.5 text-[10px] text-muted-foreground">
            {cards.length}
          </span>
        </div>
        <button
          type="button"
          onClick={onAdd}
          className="text-muted-foreground hover:text-foreground"
          aria-label={t('pages.support.kanban.newCard')}
        >
          <Plus className="h-4 w-4" />
        </button>
      </div>
      <div className="flex flex-1 flex-col gap-2 p-2">
        {cards.length === 0 ? (
          <p className="py-6 text-center text-xs text-muted-foreground">
            {t('pages.support.kanban.emptyColumn')}
          </p>
        ) : (
          cards.map((c) => <KanbanCardItem key={c.id} card={c} onOpen={onOpenCard} />)
        )}
      </div>
    </div>
  );
}

/**
 * Posição float para inserir na `index` de uma lista ORDENADA (sem o card
 * arrastado): ponto médio das vizinhas, ou um passo além das pontas.
 */
function midpoint(list: KanbanCard[], index: number): number {
  if (list.length === 0) return 0;
  if (index <= 0) return list[0].position - POSITION_STEP;
  if (index >= list.length) return list[list.length - 1].position + POSITION_STEP;
  return (list[index - 1].position + list[index].position) / 2;
}
