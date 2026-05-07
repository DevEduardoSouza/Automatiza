import { CouponsTabsHeader } from '@/components/coupons/coupons-tabs';
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
import { useCoupons } from '@/hooks/use-coupons';
import {
  type CreateCouponRuleParams,
  type LocalCouponRule,
  useCouponRules,
} from '@/hooks/use-coupon-rules';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

const PLATFORM_OPTIONS = [
  { value: 'shopee', label: 'Shopee' },
  { value: 'mercado_livre', label: 'Mercado Livre' },
];

interface RuleFormState {
  coupon_id: string;
  platform: string;
  period_start: string; // datetime-local format
  period_end: string;
  min_price: string;
  priority: string;
  is_active: boolean;
}

const EMPTY_FORM: RuleFormState = {
  coupon_id: '',
  platform: 'shopee',
  period_start: '',
  period_end: '',
  min_price: '0',
  priority: '0',
  is_active: true,
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

export default function CouponRulesPage() {
  const { t } = useTranslation();
  const { rules, isLoading, createRule, updateRule, deleteRule, isMutating } = useCouponRules();
  const { coupons } = useCoupons();

  const couponById = useMemo(() => {
    const m = new Map<string, { code: string; title: string }>();
    for (const c of coupons) m.set(c.id, { code: c.code, title: c.title });
    return m;
  }, [coupons]);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<RuleFormState>(EMPTY_FORM);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  function openCreate() {
    setEditingId(null);
    setForm(EMPTY_FORM);
    setDialogOpen(true);
  }

  function openEdit(rule: LocalCouponRule) {
    setEditingId(rule.id);
    setForm({
      coupon_id: rule.coupon_id,
      platform: rule.platform,
      period_start: rfc3339ToLocal(rule.period_start),
      period_end: rfc3339ToLocal(rule.period_end),
      min_price: String(rule.min_price ?? 0),
      priority: String(rule.priority ?? 0),
      is_active: rule.is_active,
    });
    setDialogOpen(true);
  }

  async function handleSave() {
    if (!form.coupon_id || !form.platform) return;

    const payload: CreateCouponRuleParams = {
      coupon_id: form.coupon_id,
      platform: form.platform,
      period_start: localToRFC3339(form.period_start) ?? null,
      period_end: localToRFC3339(form.period_end) ?? null,
      min_price: Number(form.min_price) || 0,
      priority: Number(form.priority) || 0,
      is_active: form.is_active,
    };

    try {
      if (editingId) {
        await updateRule({
          id: editingId,
          coupon_id: payload.coupon_id,
          platform: payload.platform,
          // Empty string explicitly clears the period bound on the server.
          period_start: payload.period_start ?? '',
          period_end: payload.period_end ?? '',
          min_price: payload.min_price,
          priority: payload.priority,
          is_active: payload.is_active,
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
        <Button onClick={openCreate} disabled={coupons.length === 0}>
          <Plus className="h-4 w-4 mr-2" aria-hidden="true" />
          {t('pages.couponRules.addRule', { defaultValue: 'Adicionar Regra' })}
        </Button>
      </div>

      {coupons.length === 0 && (
        <Card>
          <CardContent className="py-6 text-sm text-muted-foreground">
            {t('pages.couponRules.noCouponsHint', {
              defaultValue: 'Crie um cupom em "Cupons" antes de criar regras.',
            })}
          </CardContent>
        </Card>
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
                    <th className="px-4 py-3 font-medium">Cupom</th>
                    <th className="px-4 py-3 font-medium">Plataforma</th>
                    <th className="px-4 py-3 font-medium">Valor mín.</th>
                    <th className="px-4 py-3 font-medium">Período</th>
                    <th className="px-4 py-3 font-medium">Prio.</th>
                    <th className="px-4 py-3 font-medium">Status</th>
                    <th className="px-4 py-3 font-medium text-right">Ações</th>
                  </tr>
                </thead>
                <tbody>
                  {rules.map((rule) => {
                    const cup = couponById.get(rule.coupon_id);
                    return (
                      <tr key={rule.id} className="border-b">
                        <td className="px-4 py-3">
                          <div className="flex flex-col">
                            <span className="font-medium">
                              {cup?.code ?? rule.coupon_id.slice(0, 8)}
                            </span>
                            {cup?.title && (
                              <span className="text-xs text-muted-foreground">{cup.title}</span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3">{platformLabel(rule.platform)}</td>
                        <td className="px-4 py-3">{formatPrice(rule.min_price)}</td>
                        <td className="px-4 py-3 text-xs">{formatPeriod(rule)}</td>
                        <td className="px-4 py-3">{rule.priority}</td>
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
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle>
              {editingId
                ? t('pages.couponRules.editTitle', { defaultValue: 'Editar Regra' })
                : t('pages.couponRules.createTitle', { defaultValue: 'Nova Regra' })}
            </DialogTitle>
            <DialogDescription>
              {t('pages.couponRules.dialogHelp', {
                defaultValue:
                  'A regra dispara quando o produto bate em todos os filtros — plataforma, período e preço mínimo.',
              })}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="rule-coupon">Cupom</Label>
              <Select
                value={form.coupon_id}
                onValueChange={(v) => setForm((f) => ({ ...f, coupon_id: v }))}
              >
                <SelectTrigger id="rule-coupon">
                  <SelectValue placeholder="Selecione um cupom" />
                </SelectTrigger>
                <SelectContent>
                  {coupons.map((c) => (
                    <SelectItem key={c.id} value={c.id}>
                      {c.code} — {c.title}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

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
                  onChange={(e) => setForm((f) => ({ ...f, period_start: e.target.value }))}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="rule-end">Fim (opcional)</Label>
                <Input
                  id="rule-end"
                  type="datetime-local"
                  value={form.period_end}
                  onChange={(e) => setForm((f) => ({ ...f, period_end: e.target.value }))}
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="rule-min-price">Valor mín. (R$)</Label>
                <Input
                  id="rule-min-price"
                  type="number"
                  min="0"
                  step="0.01"
                  value={form.min_price}
                  onChange={(e) => setForm((f) => ({ ...f, min_price: e.target.value }))}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="rule-priority">Prioridade</Label>
                <Input
                  id="rule-priority"
                  type="number"
                  step="1"
                  value={form.priority}
                  onChange={(e) => setForm((f) => ({ ...f, priority: e.target.value }))}
                />
              </div>
            </div>

            <div className="flex items-center justify-between">
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
            <Button onClick={handleSave} disabled={!form.coupon_id || isMutating}>
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
              A regra deixa de ser avaliada nos próximos disparos. Os cupons em si não são
              afetados.
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
