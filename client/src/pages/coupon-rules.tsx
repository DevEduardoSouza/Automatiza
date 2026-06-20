import { CouponsTabsHeader } from '@/components/coupons/coupons-tabs';
import { WhatsAppPreview } from '@/components/coupons/whatsapp-preview';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  type CreateCouponRuleParams,
  type LocalCouponRule,
  useCouponRules,
} from '@/hooks/use-coupon-rules';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

const PLATFORM_OPTIONS = [
  { value: 'shopee', label: 'Shopee' },
  { value: 'mercado_livre', label: 'Mercado Livre' },
];

/**
 * Ordem da lista de regras de cupom.
 * - recent: data de criação, mais recentes primeiro (default)
 * - oldest: data de criação, mais antigas primeiro
 * - period: data de início do período, as que começam antes primeiro
 *   (regras "Sempre ativo", sem período, vão pro fim)
 */
type RuleSortOrder = 'recent' | 'oldest' | 'period';

interface RuleFormState {
  platform: string;
  period_start: string; // datetime-local format
  period_end: string;
  min_price: string;
  is_active: boolean;
  official_only: boolean;
  message: string;
  discount_type: '' | 'fixed' | 'percent';
  discount_value: string;
  max_discount: string;
}

const EMPTY_FORM: RuleFormState = {
  platform: 'shopee',
  period_start: '',
  period_end: '',
  min_price: '0',
  is_active: true,
  official_only: false,
  message: '',
  discount_type: '',
  discount_value: '',
  max_discount: '',
};

// datetime-local widgets emit "YYYY-MM-DDTHH:mm"; the proto / repo
// expects RFC3339. Convert when sending and when prefilling the form.
function localToRFC3339(local: string): string | undefined {
  if (!local) return undefined;
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString();
}
function rfc3339ToLocal(rfc?: string | null): string {
  if (!rfc) return '';
  const d = new Date(rfc);
  if (Number.isNaN(d.getTime())) return '';
  // Strip seconds + Z so the input control accepts it.
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function platformLabel(p: string) {
  const opt = PLATFORM_OPTIONS.find((o) => o.value === p);
  return opt?.label ?? p;
}

function formatPeriod(rule: LocalCouponRule): string {
  if (!rule.period_start && !rule.period_end) return 'Sempre ativo';
  const fmt = (iso: string) => new Date(iso).toLocaleString('pt-BR');
  const start = rule.period_start ? fmt(rule.period_start) : '—';
  const end = rule.period_end ? fmt(rule.period_end) : '—';
  return `${start} até ${end}`;
}

function formatPrice(v: number): string {
  if (v <= 0) return 'Qualquer';
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
}

function formatDiscount(rule: LocalCouponRule): string {
  if (!rule.discount_type || rule.discount_value == null) return '—';
  if (rule.discount_type === 'fixed') {
    return `R$ ${rule.discount_value.toFixed(2)} OFF`;
  }
  let line = `${rule.discount_value.toFixed(0)}% OFF`;
  if (rule.max_discount && rule.max_discount > 0) {
    line += ` (max R$ ${rule.max_discount.toFixed(2)})`;
  }
  return line;
}

// Build the WhatsApp preview body from the form state. Mirrors the
// formatter the worker uses (formatCouponText em coupon_handler.go): o
// bloco do cupom no disparo é literalmente o que o operador digitou em
// "Mensagem do cupom". Código, desconto e link continuam no form porque
// alimentam preço/cálculo/analytics, mas não aparecem mais no texto
// auto-gerado — quem quiser inclui na própria mensagem.
function buildPreview(form: RuleFormState): string {
  return form.message.trim();
}

export default function CouponRulesPage() {
  const { t } = useTranslation();
  const { rules, isLoading, createRule, updateRule, deleteRule, isMutating } = useCouponRules();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<RuleFormState>(EMPTY_FORM);
  const [periodError, setPeriodError] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [sortOrder, setSortOrder] = useState<RuleSortOrder>('recent');

  // Ordena por data pra facilitar localizar uma regra. Datas são ISO 8601,
  // então localeCompare ordena cronologicamente. Copia o array antes (rules
  // vem do cache do React Query — não pode mutar).
  const sortedRules = useMemo(() => {
    const copy = [...rules];
    switch (sortOrder) {
      case 'oldest':
        return copy.sort((a, b) => a.created_at.localeCompare(b.created_at));
      case 'period':
        // Começa antes no topo; regras sem período ("Sempre ativo") vão pro fim.
        return copy.sort((a, b) => {
          if (!a.period_start && !b.period_start) return 0;
          if (!a.period_start) return 1;
          if (!b.period_start) return -1;
          return a.period_start.localeCompare(b.period_start);
        });
      default: // 'recent' — mais recentes primeiro (default)
        return copy.sort((a, b) => b.created_at.localeCompare(a.created_at));
    }
  }, [rules, sortOrder]);

  function openCreate() {
    setEditingId(null);
    setForm(EMPTY_FORM);
    setPeriodError(null);
    setDialogOpen(true);
  }

  function openEdit(rule: LocalCouponRule) {
    setEditingId(rule.id);
    setPeriodError(null);
    setForm({
      platform: rule.platform,
      period_start: rfc3339ToLocal(rule.period_start),
      period_end: rfc3339ToLocal(rule.period_end),
      min_price: String(rule.min_price ?? 0),
      is_active: rule.is_active,
      official_only: rule.official_only,
      message: rule.message ?? '',
      discount_type: rule.discount_type ?? '',
      discount_value: rule.discount_value != null ? String(rule.discount_value) : '',
      max_discount: rule.max_discount != null ? String(rule.max_discount) : '',
    });
    setDialogOpen(true);
  }

  async function handleSave() {
    if (!form.platform) return;
    if (!form.message.trim()) return; // mensagem é o que aparece no disparo

    const startRFC = localToRFC3339(form.period_start) ?? null;
    const endRFC = localToRFC3339(form.period_end) ?? null;
    // Janela inválida (fim <= início) salva uma regra que nunca dispara: ela
    // aparece ativa mas não casa nenhum produto. Bloqueia antes de enviar.
    if (startRFC && endRFC && new Date(endRFC) <= new Date(startRFC)) {
      setPeriodError('O fim do período deve ser depois do início.');
      return;
    }
    setPeriodError(null);

    const discountValueNum = form.discount_value ? Number(form.discount_value) : null;
    const maxDiscountNum = form.max_discount ? Number(form.max_discount) : null;

    const payload: CreateCouponRuleParams = {
      platform: form.platform,
      period_start: startRFC,
      period_end: endRFC,
      min_price: Number(form.min_price) || 0,
      // Priority foi removido do form em PR #128-ish — todas as regras
      // criadas/editadas pela UI gravam 0. Backend ainda tem a coluna e
      // ordena por (min_price DESC, priority DESC, created_at DESC), então
      // regras antigas com priority>0 continuam vencendo o tie-break. Pra
      // ajustar uma regra legada agora, mexer direto no banco.
      priority: 0,
      is_active: form.is_active,
      // official_only só faz efeito no Shopee (onde o flag IsMall é
      // populado); na ML o filtro acaba não casando nada. Forçamos
      // false quando a plataforma não é Shopee pra não confundir o
      // operador no banco.
      official_only: form.platform === 'shopee' && form.official_only,
      // code removido do form (PR atual): operador inclui o código na
      // própria mensagem se quiser exibir. Backend continua aceitando o
      // campo, então regras criadas via API ainda podem mandar — daqui
      // sempre fica null.
      code: null,
      message: form.message.trim() || null,
      discount_type: form.discount_type || null,
      discount_value: discountValueNum,
      max_discount: maxDiscountNum,
    };

    try {
      if (editingId) {
        // Code intencionalmente omitido: regras legadas com code preenchido
        // preservam o valor (handler do backend só altera campos enviados).
        // Pra limpar code de uma regra antiga é necessário mexer direto no
        // banco — não é exposto na UI hoje.
        await updateRule({
          id: editingId,
          platform: payload.platform,
          // Empty string explicitly clears the period bound on the server.
          period_start: payload.period_start ?? '',
          period_end: payload.period_end ?? '',
          min_price: payload.min_price,
          priority: payload.priority,
          is_active: payload.is_active,
          official_only: payload.official_only,
          message: payload.message ?? '',
          discount_type: payload.discount_type ?? null,
          discount_value: discountValueNum ?? 0,
          max_discount: maxDiscountNum ?? 0,
        });
      } else {
        await createRule(payload);
      }
      setDialogOpen(false);
    } catch {
      // toast already shown by hook
    }
  }

  async function handleToggle(rule: LocalCouponRule) {
    try {
      await updateRule({ id: rule.id, is_active: !rule.is_active });
    } catch {
      // toast already shown by hook
    }
  }

  async function handleDelete() {
    if (!confirmDeleteId) return;
    try {
      await deleteRule(confirmDeleteId);
      setConfirmDeleteId(null);
    } catch {
      // toast already shown by hook
    }
  }

  return (
    <div className="flex flex-col gap-4 p-4 lg:p-8 xl:max-w-[1400px] mx-auto w-full">
      <CouponsTabsHeader />
      <div className="flex items-center justify-between gap-2">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t('pages.couponRules.title', { defaultValue: 'Cupons Avançados' })}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t('pages.couponRules.subtitle', {
              defaultValue:
                'Regras que aplicam um cupom automaticamente quando uma oferta encaixa nos filtros.',
            })}
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="h-4 w-4 mr-2" aria-hidden="true" />
          {t('pages.couponRules.addRule', { defaultValue: 'Adicionar Regra' })}
        </Button>
      </div>

      {rules.length > 0 && (
        <div className="flex items-center justify-end gap-2">
          <Label htmlFor="rule-sort" className="text-sm text-muted-foreground">
            Ordenar por
          </Label>
          <Select value={sortOrder} onValueChange={(v) => setSortOrder(v as RuleSortOrder)}>
            <SelectTrigger id="rule-sort" className="w-[200px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="recent">Mais recentes</SelectItem>
              <SelectItem value="oldest">Mais antigas</SelectItem>
              <SelectItem value="period">Período (começa antes)</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="p-6 text-sm text-muted-foreground">Carregando...</div>
          ) : rules.length === 0 ? (
            <div className="p-6 text-sm text-muted-foreground">
              {t('pages.couponRules.empty', {
                defaultValue: 'Nenhuma regra criada ainda.',
              })}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-muted-foreground">
                    <th className="px-4 py-3 font-medium">Mensagem</th>
                    <th className="px-4 py-3 font-medium">Desconto</th>
                    <th className="px-4 py-3 font-medium">Plataforma</th>
                    <th className="px-4 py-3 font-medium">Valor mín.</th>
                    <th className="px-4 py-3 font-medium">Período</th>
                    <th className="px-4 py-3 font-medium">Status</th>
                    <th className="px-4 py-3 font-medium text-right">Ações</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedRules.map((rule) => (
                    <tr key={rule.id} className="border-b">
                      <td className="px-4 py-3 max-w-md">
                        <span className="text-sm line-clamp-2">
                          {rule.message ?? '—'}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-xs">{formatDiscount(rule)}</td>
                      <td className="px-4 py-3">
                        <div className="flex flex-col gap-0.5">
                          <span>{platformLabel(rule.platform)}</span>
                          {rule.official_only && (
                            <span className="inline-flex w-fit items-center rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-medium text-amber-800">
                              Loja Oficial
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3">{formatPrice(rule.min_price)}</td>
                      <td className="px-4 py-3 text-xs">{formatPeriod(rule)}</td>
                      <td className="px-4 py-3">
                        <Switch
                          checked={rule.is_active}
                          onCheckedChange={() => handleToggle(rule)}
                          disabled={isMutating}
                          aria-label="Ativar/desativar"
                        />
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => openEdit(rule)}
                            aria-label="Editar"
                          >
                            <Pencil className="h-4 w-4" aria-hidden="true" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => setConfirmDeleteId(rule.id)}
                            aria-label="Remover"
                          >
                            <Trash2 className="h-4 w-4 text-red-500" aria-hidden="true" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-[680px] max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingId
                ? t('pages.couponRules.editTitle', { defaultValue: 'Editar Regra' })
                : t('pages.couponRules.createTitle', { defaultValue: 'Nova Regra' })}
            </DialogTitle>
            <DialogDescription>
              {t('pages.couponRules.dialogHelp', {
                defaultValue:
                  'A regra dispara quando o produto bate nos filtros (plataforma + valor mínimo + período) e injeta a mensagem na oferta.',
              })}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 py-2">
            {/* === Filtros === */}
            <section className="space-y-3">
              <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
                Filtros
              </h3>

              <div className="space-y-1.5">
                <Label htmlFor="rule-platform">Plataforma</Label>
                <Select
                  value={form.platform}
                  onValueChange={(v) => setForm((f) => ({ ...f, platform: v }))}
                >
                  <SelectTrigger id="rule-platform">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PLATFORM_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="rule-start">Início (opcional)</Label>
                  <Input
                    id="rule-start"
                    type="datetime-local"
                    value={form.period_start}
                    onChange={(e) => {
                      setPeriodError(null);
                      setForm((f) => ({ ...f, period_start: e.target.value }));
                    }}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="rule-end">Fim (opcional)</Label>
                  <Input
                    id="rule-end"
                    type="datetime-local"
                    value={form.period_end}
                    aria-invalid={periodError ? true : undefined}
                    onChange={(e) => {
                      setPeriodError(null);
                      setForm((f) => ({ ...f, period_end: e.target.value }));
                    }}
                  />
                </div>
              </div>
              {periodError && (
                <p className="text-xs text-red-500" role="alert">
                  {periodError}
                </p>
              )}

              <div className="space-y-1.5">
                <Label htmlFor="rule-min-price">Valor mín. produto (R$)</Label>
                <Input
                  id="rule-min-price"
                  type="number"
                  min="0"
                  step="0.01"
                  value={form.min_price}
                  onChange={(e) => setForm((f) => ({ ...f, min_price: e.target.value }))}
                />
              </div>

              {form.platform === 'shopee' && (
                <div className="flex items-start justify-between gap-4 rounded-lg border p-3">
                  <div className="space-y-0.5">
                    <Label htmlFor="rule-official" className="cursor-pointer">
                      Apenas Loja Oficial Shopee
                    </Label>
                    <p className="text-xs text-muted-foreground">
                      Quando ligado, a regra só dispara em produtos do Shopee Mall
                      (selo de loja oficial).
                    </p>
                  </div>
                  <Switch
                    id="rule-official"
                    checked={form.official_only}
                    onCheckedChange={(v) => setForm((f) => ({ ...f, official_only: v }))}
                  />
                </div>
              )}
            </section>

            {/* === Cupom a Aplicar === */}
            <section className="space-y-3">
              <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
                Cupom a aplicar
              </h3>

              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="rule-discount-type">Tipo</Label>
                  <Select
                    value={form.discount_type || 'none'}
                    onValueChange={(v) =>
                      setForm((f) => ({
                        ...f,
                        discount_type: v === 'none' ? '' : (v as 'fixed' | 'percent'),
                      }))
                    }
                  >
                    <SelectTrigger id="rule-discount-type">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">—</SelectItem>
                      <SelectItem value="fixed">Fixo (R$)</SelectItem>
                      <SelectItem value="percent">Percentual (%)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="rule-discount-value">Valor</Label>
                  <Input
                    id="rule-discount-value"
                    type="number"
                    min="0"
                    step="0.01"
                    placeholder="0"
                    value={form.discount_value}
                    onChange={(e) => setForm((f) => ({ ...f, discount_value: e.target.value }))}
                    disabled={!form.discount_type}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="rule-max-discount">Desconto máx. (R$)</Label>
                  <Input
                    id="rule-max-discount"
                    type="number"
                    min="0"
                    step="0.01"
                    placeholder="0"
                    value={form.max_discount}
                    onChange={(e) => setForm((f) => ({ ...f, max_discount: e.target.value }))}
                    disabled={form.discount_type !== 'percent'}
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="rule-message">Mensagem do cupom</Label>
                <Textarea
                  id="rule-message"
                  rows={4}
                  placeholder="Ex.: Use o cupom e ganhe R$ 100 OFF | resgate aqui: https://..."
                  value={form.message}
                  onChange={(e) => setForm((f) => ({ ...f, message: e.target.value }))}
                />
                <p className="text-xs text-muted-foreground">
                  Suporta formatação do WhatsApp:{' '}
                  <code className="text-[10px]">*negrito*</code>{' '}
                  <code className="text-[10px]">_itálico_</code>{' '}
                  <code className="text-[10px]">~tachado~</code>{' '}
                  <code className="text-[10px]">`mono`</code>
                </p>
              </div>

              <WhatsAppPreview
                text={buildPreview(form)}
                title="Preview no WhatsApp"
                emptyHint="Preencha a mensagem do cupom para ver o preview..."
              />
            </section>

            <div className="flex items-center justify-between rounded-lg border p-3">
              <Label htmlFor="rule-active" className="cursor-pointer">
                Regra ativa
              </Label>
              <Switch
                id="rule-active"
                checked={form.is_active}
                onCheckedChange={(v) => setForm((f) => ({ ...f, is_active: v }))}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Cancelar
            </Button>
            <Button
              onClick={handleSave}
              disabled={isMutating || !form.message.trim() || !!periodError}
            >
              {editingId ? 'Salvar' : 'Criar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <Dialog
        open={confirmDeleteId !== null}
        onOpenChange={(open) => !open && setConfirmDeleteId(null)}
      >
        <DialogContent className="sm:max-w-[400px]">
          <DialogHeader>
            <DialogTitle>Remover regra</DialogTitle>
            <DialogDescription>
              A regra deixa de ser avaliada nos próximos disparos.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmDeleteId(null)}>
              Cancelar
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={isMutating}>
              Remover
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
