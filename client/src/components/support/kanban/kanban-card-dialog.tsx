import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { toast } from '@/components/ui/toast-utils';
import type { KanbanAttachment, KanbanCard } from '@/gen/support/v1/support_pb';
import { Download, FileText, Loader2, Paperclip, Trash2 } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  COLUMN_META,
  type ColumnKey,
  PRIORITIES,
  type Priority,
  formatBytes,
} from './kanban-shared';

/**
 * Modal de criação/edição de um card do Kanban. Concentra o que dá pra
 * "escrever no card" (título, descrição livre, prioridade, coluna inicial) e a
 * gestão de anexos (upload/baixar/remover documentos).
 *
 * É burro de propósito: recebe callbacks do board (que detém as mutations e o
 * refetch) e só orquestra o formulário. Serve tanto para criar (card = null)
 * quanto para editar (card preenchido).
 */
interface Props {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  /** null => modo criação. */
  card: KanbanCard | null;
  /** Coluna sugerida ao criar (o "+" de uma coluna específica). */
  defaultColumn?: ColumnKey;
  onCreate: (input: {
    title: string;
    description: string;
    columnKey: ColumnKey;
    priority: Priority;
  }) => Promise<KanbanCard>;
  onUpdate: (input: {
    id: string;
    title: string;
    description: string;
    priority: Priority;
  }) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
  onUpload: (cardId: string, file: File) => Promise<void>;
  onDownload: (att: KanbanAttachment) => Promise<void>;
  onDeleteAttachment: (id: string) => Promise<void>;
}

export function KanbanCardDialog({
  open,
  onOpenChange,
  card,
  defaultColumn = 'corrigir',
  onCreate,
  onUpdate,
  onDelete,
  onUpload,
  onDownload,
  onDeleteAttachment,
}: Props) {
  const { t } = useTranslation();
  const isEdit = card !== null;

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState<Priority>('medium');
  const [columnKey, setColumnKey] = useState<ColumnKey>(defaultColumn);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [busyAtt, setBusyAtt] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  // Sincroniza o formulário ao abrir / trocar de card.
  useEffect(() => {
    if (!open) return;
    setTitle(card?.title ?? '');
    setDescription(card?.description ?? '');
    setPriority(((card?.priority as Priority) || 'medium') as Priority);
    setColumnKey(((card?.columnKey as ColumnKey) || defaultColumn) as ColumnKey);
  }, [open, card, defaultColumn]);

  const submit = async () => {
    if (!title.trim()) {
      toast.error(t('pages.support.kanban.form.errTitle'));
      return;
    }
    setSaving(true);
    try {
      if (isEdit && card) {
        await onUpdate({ id: card.id, title: title.trim(), description, priority });
      } else {
        await onCreate({ title: title.trim(), description, columnKey, priority });
      }
      onOpenChange(false);
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!card) return;
    if (!confirm(t('pages.support.kanban.form.confirmDelete'))) return;
    setSaving(true);
    try {
      await onDelete(card.id);
      onOpenChange(false);
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const pickFile = () => fileRef.current?.click();

  const onFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = ''; // permite re-selecionar o mesmo arquivo
    if (!file || !card) return;
    setUploading(true);
    try {
      await onUpload(card.id, file);
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setUploading(false);
    }
  };

  const download = async (att: KanbanAttachment) => {
    setBusyAtt(att.id);
    try {
      await onDownload(att);
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setBusyAtt(null);
    }
  };

  const removeAttachment = async (att: KanbanAttachment) => {
    setBusyAtt(att.id);
    try {
      await onDeleteAttachment(att.id);
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setBusyAtt(null);
    }
  };

  const attachments = card?.attachments ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {isEdit
              ? t('pages.support.kanban.form.editTitle')
              : t('pages.support.kanban.form.newTitle')}
          </DialogTitle>
          <DialogDescription>{t('pages.support.kanban.form.subtitle')}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-1">
          <div className="space-y-1.5">
            <Label htmlFor="kb-title">{t('pages.support.kanban.form.title')}</Label>
            <Input
              id="kb-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={t('pages.support.kanban.form.titlePh')}
              autoFocus
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="kb-desc">{t('pages.support.kanban.form.description')}</Label>
            <Textarea
              id="kb-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('pages.support.kanban.form.descriptionPh')}
              rows={5}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>{t('pages.support.kanban.form.priority')}</Label>
              <div className="flex gap-1.5">
                {PRIORITIES.map((p) => (
                  <button
                    key={p}
                    type="button"
                    onClick={() => setPriority(p)}
                    className={`flex-1 rounded-md border px-2 py-1.5 text-xs font-medium transition ${
                      priority === p
                        ? `${PRIORITY_ACTIVE[p]} `
                        : 'text-muted-foreground hover:bg-muted'
                    }`}
                  >
                    {t(`pages.support.kanban.priority.${p}`)}
                  </button>
                ))}
              </div>
            </div>

            {/* Coluna inicial só faz sentido na criação; ao editar, move-se por arrasto. */}
            {!isEdit && (
              <div className="space-y-1.5">
                <Label htmlFor="kb-col">{t('pages.support.kanban.form.column')}</Label>
                <select
                  id="kb-col"
                  value={columnKey}
                  onChange={(e) => setColumnKey(e.target.value as ColumnKey)}
                  className="h-9 w-full rounded-md border border-input bg-background px-2 text-sm"
                >
                  {COLUMN_META.map((c) => (
                    <option key={c.key} value={c.key}>
                      {t(c.labelKey)}
                    </option>
                  ))}
                </select>
              </div>
            )}
          </div>

          {/* Anexos — só disponível depois que o card existe (precisa de id). */}
          {isEdit && (
            <div className="space-y-2 rounded-lg border p-3">
              <div className="flex items-center justify-between">
                <span className="flex items-center gap-1.5 text-sm font-medium">
                  <Paperclip className="h-3.5 w-3.5" />
                  {t('pages.support.kanban.form.attachments')}
                </span>
                <Button size="sm" variant="outline" onClick={pickFile} disabled={uploading}>
                  {uploading ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Paperclip className="h-3.5 w-3.5" />
                  )}
                  {t('pages.support.kanban.form.addFile')}
                </Button>
                <input ref={fileRef} type="file" className="hidden" onChange={onFileChange} />
              </div>

              {attachments.length === 0 ? (
                <p className="py-2 text-center text-xs text-muted-foreground">
                  {t('pages.support.kanban.form.noAttachments')}
                </p>
              ) : (
                <ul className="space-y-1">
                  {attachments.map((att) => (
                    <li
                      key={att.id}
                      className="flex items-center gap-2 rounded-md bg-muted/50 px-2 py-1.5 text-sm"
                    >
                      <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
                      <span className="min-w-0 flex-1 truncate" title={att.filename}>
                        {att.filename}
                      </span>
                      <span className="shrink-0 text-[10px] text-muted-foreground">
                        {formatBytes(Number(att.sizeBytes))}
                      </span>
                      <button
                        type="button"
                        onClick={() => download(att)}
                        disabled={busyAtt === att.id}
                        className="text-muted-foreground hover:text-foreground disabled:opacity-50"
                        aria-label={t('common.download')}
                      >
                        {busyAtt === att.id ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        ) : (
                          <Download className="h-3.5 w-3.5" />
                        )}
                      </button>
                      <button
                        type="button"
                        onClick={() => removeAttachment(att)}
                        disabled={busyAtt === att.id}
                        className="text-muted-foreground hover:text-red-600 disabled:opacity-50"
                        aria-label={t('common.delete')}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>

        <DialogFooter className="gap-2 sm:justify-between">
          {isEdit ? (
            <Button variant="ghost" onClick={remove} disabled={saving} className="text-red-600">
              <Trash2 className="h-4 w-4" />
              {t('common.delete')}
            </Button>
          ) : (
            <span />
          )}
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
              {t('common.cancel')}
            </Button>
            <Button onClick={submit} disabled={saving}>
              {saving && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('common.save')}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const PRIORITY_ACTIVE: Record<Priority, string> = {
  low: 'border-slate-400 bg-slate-400/10 text-slate-600',
  medium: 'border-amber-500 bg-amber-500/10 text-amber-600',
  high: 'border-red-500 bg-red-500/10 text-red-600',
};
