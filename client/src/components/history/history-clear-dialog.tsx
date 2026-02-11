import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Loader2, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface HistoryClearDialogProps {
  onConfirm: () => Promise<void>;
  loading: boolean;
  disabled?: boolean;
}

export function HistoryClearDialog({ onConfirm, loading, disabled }: HistoryClearDialogProps) {
  const { t } = useTranslation();

  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <button
          type="button"
          disabled={disabled || loading}
          className="inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Trash2 className="h-3.5 w-3.5" />
          {t('pages.history.clearButton')}
        </button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('pages.history.clearDialog.title')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('pages.history.clearDialog.description')}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm} disabled={loading}>
            {loading && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
            {t('common.confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
