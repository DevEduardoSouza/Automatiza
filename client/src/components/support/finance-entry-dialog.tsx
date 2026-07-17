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
import { Switch } from '@/components/ui/switch';
import { toast } from '@/components/ui/toast-utils';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

export interface NewFinanceEntry {
  kind: 'expense' | 'revenue';
  description: string;
  amount: number;
  category?: string;
  entryDate: string; // YYYY-MM-DD
  recurring: boolean;
  recurrenceMonths?: number; // ausente + recurring => infinito
}

interface Props {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  onSubmit: (e: NewFinanceEntry) => Promise<void>;
}

const today = () => new Date().toISOString().slice(0, 10);

/**
 * Modal de lançamento de despesa/lucro, com recorrência opcional (N meses ou
 * infinito). É a escrita da aba Financeiro; valida no cliente antes de enviar,
 * mas o backend revalida (kind, amount>=0, data, recurrence>0).
 */
export function FinanceEntryDialog({ open, onOpenChange, onSubmit }: Props) {
  const { t } = useTranslation();
  const [kind, setKind] = useState<'expense' | 'revenue'>('expense');
  const [description, setDescription] = useState('');
  const [amount, setAmount] = useState('');
  const [category, setCategory] = useState('');
  const [entryDate, setEntryDate] = useState(today());
  const [recurring, setRecurring] = useState(false);
  const [infinite, setInfinite] = useState(true);
  const [months, setMonths] = useState('12');
  const [saving, setSaving] = useState(false);

  const reset = () => {
    setKind('expense');
    setDescription('');
    setAmount('');
    setCategory('');
    setEntryDate(today());
    setRecurring(false);
    setInfinite(true);
    setMonths('12');
  };

  const close = (v: boolean) => {
    if (!v) reset();
    onOpenChange(v);
  };

  const submit = async () => {
    const value = Number(amount.replace(',', '.'));
    if (!description.trim()) {
      toast.error(t('pages.support.financeiro.form.errDescription'));
      return;
    }
    if (!Number.isFinite(value) || value < 0) {
      toast.error(t('pages.support.financeiro.form.errAmount'));
      return;
    }
    const n = Number(months);
    if (recurring && !infinite && (!Number.isInteger(n) || n <= 0)) {
      toast.error(t('pages.support.financeiro.form.errMonths'));
      return;
    }
    const payload: NewFinanceEntry = {
      kind,
      description: description.trim(),
      amount: value,
      entryDate,
      recurring,
    };
    if (category.trim()) payload.category = category.trim();
    if (recurring && !infinite) payload.recurrenceMonths = n;

    setSaving(true);
    try {
      await onSubmit(payload);
      toast.success(t('pages.support.financeiro.form.saved'));
      close(false);
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('pages.support.financeiro.form.title')}</DialogTitle>
          <DialogDescription>{t('pages.support.financeiro.form.subtitle')}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Tipo: despesa | lucro */}
          <div className="grid grid-cols-2 gap-2">
            <button
              type="button"
              onClick={() => setKind('expense')}
              className={`rounded-lg border p-2 text-sm font-medium transition ${
                kind === 'expense'
                  ? 'border-red-500 bg-red-500/10 text-red-600'
                  : 'text-muted-foreground'
              }`}
            >
              {t('pages.support.financeiro.form.expense')}
            </button>
            <button
              type="button"
              onClick={() => setKind('revenue')}
              className={`rounded-lg border p-2 text-sm font-medium transition ${
                kind === 'revenue'
                  ? 'border-emerald-500 bg-emerald-500/10 text-emerald-600'
                  : 'text-muted-foreground'
              }`}
            >
              {t('pages.support.financeiro.form.revenue')}
            </button>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="fin-desc">{t('pages.support.financeiro.form.description')}</Label>
            <Input
              id="fin-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('pages.support.financeiro.form.descriptionPh')}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="fin-amount">{t('pages.support.financeiro.form.amount')}</Label>
              <Input
                id="fin-amount"
                inputMode="decimal"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="0,00"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="fin-date">{t('pages.support.financeiro.form.date')}</Label>
              <Input
                id="fin-date"
                type="date"
                value={entryDate}
                onChange={(e) => setEntryDate(e.target.value)}
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="fin-cat">{t('pages.support.financeiro.form.category')}</Label>
            <Input
              id="fin-cat"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              placeholder={t('pages.support.financeiro.form.categoryPh')}
            />
          </div>

          {/* Recorrência */}
          <div className="rounded-lg border p-3 space-y-3">
            <div className="flex items-center justify-between">
              <Label htmlFor="fin-rec" className="cursor-pointer">
                {t('pages.support.financeiro.form.recurring')}
              </Label>
              <Switch id="fin-rec" checked={recurring} onCheckedChange={setRecurring} />
            </div>
            {recurring && (
              <div className="space-y-2 text-sm">
                <label className="flex items-center gap-2">
                  <input type="radio" checked={infinite} onChange={() => setInfinite(true)} />
                  {t('pages.support.financeiro.form.infinite')}
                </label>
                <label className="flex items-center gap-2">
                  <input type="radio" checked={!infinite} onChange={() => setInfinite(false)} />
                  {t('pages.support.financeiro.form.forMonths')}
                  <Input
                    className="h-8 w-20"
                    inputMode="numeric"
                    value={months}
                    disabled={infinite}
                    onChange={(e) => setMonths(e.target.value)}
                  />
                  {t('pages.support.financeiro.form.months')}
                </label>
              </div>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => close(false)} disabled={saving}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving}>
            {t('pages.support.financeiro.form.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
