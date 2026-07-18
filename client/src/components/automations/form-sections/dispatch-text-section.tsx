import { Button } from '@/components/ui/button';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useDispatchTextsConnect } from '@/hooks/use-dispatch-texts-connect';
import type { Automation, DispatchText } from '@/types/automation';
import { ChevronDown, Pencil } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

interface DispatchTextSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  dispatchTexts: DispatchText[];
}

/** copies é a fonte da verdade; body é legado (copies[0]). */
function contentOf(dt: DispatchText | undefined): string[] {
  if (!dt) return [];
  if (dt.copies?.length) return dt.copies;
  return dt.body ? [dt.body] : [];
}

export function DispatchTextSelector({
  formData,
  setFormData,
  dispatchTexts,
}: DispatchTextSelectorProps) {
  const { t } = useTranslation();
  const { updateDispatchText, isUpdatingDispatchText } = useDispatchTextsConnect();

  const [previewOpen, setPreviewOpen] = useState(true);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState('');

  const selected = formData.dispatch_text_id
    ? dispatchTexts.find((dt) => dt.id === formData.dispatch_text_id)
    : undefined;
  const copies = contentOf(selected);
  // Edição inline só faz sentido com 1 variação — um único textarea não
  // representa múltiplas cópias sem perdê-las no save. Multi-cópia edita no
  // editor completo (/dispatch-texts).
  const canEditInline = copies.length <= 1;

  // Trocar de template (ou desmarcar) cancela uma edição em andamento para o
  // rascunho não vazar de um template para outro.
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the selected template changes
  useEffect(() => {
    setEditing(false);
  }, [selected?.id]);

  const startEdit = () => {
    setDraft(copies[0] ?? '');
    setEditing(true);
    setPreviewOpen(true);
  };

  const saveEdit = async () => {
    if (!selected) return;
    // Preserva name/is_active — o update sobrescreve o registro inteiro.
    await updateDispatchText({
      id: selected.id,
      name: selected.name,
      copies: [draft],
      is_active: selected.is_active,
    });
    setEditing(false);
  };

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <Label className="cursor-pointer">{t('pages.offerAutomations.form.dispatchText')}</Label>
      <Select
        value={formData.dispatch_text_id || '__default__'}
        onValueChange={(v) =>
          setFormData((prev) => ({
            ...prev,
            dispatch_text_id: v === '__default__' ? null : v,
          }))
        }
      >
        <SelectTrigger className="cursor-pointer">
          <SelectValue placeholder={t('pages.offerAutomations.form.useDefault')} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__default__" className="cursor-pointer">
            {t('pages.offerAutomations.form.systemDefault')}
          </SelectItem>
          {dispatchTexts.map((dt) => (
            <SelectItem key={dt.id} value={dt.id} className="cursor-pointer">
              {dt.name}
              {dt.copies?.length > 1 &&
                ` (${dt.copies.length} ${t('pages.dispatchTexts.copies.variations')})`}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Prévia do template escolhido — recolhível, com edição na hora. */}
      {selected && (
        <Collapsible
          open={previewOpen}
          onOpenChange={setPreviewOpen}
          className="rounded-lg border bg-muted/30"
        >
          <div className="flex items-center justify-between gap-2 px-3 py-2">
            <CollapsibleTrigger className="flex items-center gap-1.5 text-xs font-medium cursor-pointer">
              <ChevronDown
                className={`h-3.5 w-3.5 transition-transform ${previewOpen ? '' : '-rotate-90'}`}
              />
              {t('pages.offerAutomations.form.dispatchTextPreview', 'Prévia do texto')}
              {copies.length > 1 &&
                ` (${copies.length} ${t('pages.dispatchTexts.copies.variations')})`}
            </CollapsibleTrigger>
            {!editing &&
              (canEditInline ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-6 gap-1 px-2 text-xs"
                  onClick={startEdit}
                >
                  <Pencil className="h-3 w-3" />
                  {t('common.edit', 'Editar')}
                </Button>
              ) : (
                <Link
                  to="/dispatch-texts"
                  className="flex items-center gap-1 text-xs underline text-muted-foreground"
                >
                  <Pencil className="h-3 w-3" />
                  {t('common.edit', 'Editar')}
                </Link>
              ))}
          </div>
          <CollapsibleContent className="px-3 pb-3">
            {editing ? (
              <div className="space-y-2">
                <Textarea
                  rows={7}
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  className="text-xs font-mono"
                />
                <p className="text-[11px] text-muted-foreground">
                  {t(
                    'pages.offerAutomations.form.dispatchTextEditWarning',
                    'Isso altera o texto em todas as automações que usam este template.'
                  )}
                </p>
                <div className="flex gap-2">
                  <Button
                    type="button"
                    size="sm"
                    className="h-7 text-xs"
                    onClick={saveEdit}
                    disabled={isUpdatingDispatchText}
                  >
                    {isUpdatingDispatchText
                      ? t('common.saving', 'Salvando...')
                      : t('common.save', 'Salvar')}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    className="h-7 text-xs"
                    onClick={() => setEditing(false)}
                    disabled={isUpdatingDispatchText}
                  >
                    {t('common.cancel', 'Cancelar')}
                  </Button>
                </div>
              </div>
            ) : (
              <p className="whitespace-pre-wrap break-words text-xs text-muted-foreground">
                {copies.join('\n\n— ou —\n\n')}
              </p>
            )}
          </CollapsibleContent>
        </Collapsible>
      )}

      <p className="text-xs text-muted-foreground">
        {t('pages.offerAutomations.form.dispatchTextHint')}
      </p>
      {!dispatchTexts.length && (
        <p className="text-xs text-muted-foreground">
          {t('pages.offerAutomations.form.noDispatchTexts')}{' '}
          <Link to="/dispatch-texts" className="underline">
            {t('pages.offerAutomations.form.createNow')}
          </Link>
        </p>
      )}
    </div>
  );
}
