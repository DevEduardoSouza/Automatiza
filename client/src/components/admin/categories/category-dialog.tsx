import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { IconPicker } from '@/components/ui/icon-picker';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import type { LocalCategory } from '@/hooks/use-categories';
import type { TFunction } from 'i18next';
import type { CategoryFormData } from '../types/categories.types';

interface CategoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingCategory: LocalCategory | null;
  formData: CategoryFormData;
  setFormData: React.Dispatch<React.SetStateAction<CategoryFormData>>;
  onSave: () => Promise<void>;
  saving: boolean;
  t: TFunction;
}

export function CategoryDialog({
  open,
  onOpenChange,
  editingCategory,
  formData,
  setFormData,
  onSave,
  saving,
  t,
}: CategoryDialogProps) {
  const isEditing = !!editingCategory;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>
            {isEditing
              ? t('admin.categories.dialog.editCategory')
              : t('admin.categories.dialog.createCategory')}
          </DialogTitle>
          <DialogDescription>
            {isEditing
              ? t('admin.categories.dialog.editCategoryDescription')
              : t('admin.categories.dialog.createCategoryDescription')}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="category-name">{t('admin.categories.form.name')}</Label>
            <Input
              id="category-name"
              value={formData.name}
              onChange={(e) => setFormData((prev) => ({ ...prev, name: e.target.value }))}
              placeholder={t('admin.categories.form.namePlaceholder')}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="category-icon">{t('admin.categories.form.icon')}</Label>
            <IconPicker
              value={formData.icon}
              onChange={(icon) => setFormData((prev) => ({ ...prev, icon }))}
              placeholder={t('admin.categories.form.selectIcon')}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="category-order">{t('admin.categories.form.displayOrder')}</Label>
            <Input
              id="category-order"
              type="number"
              value={formData.displayOrder}
              onChange={(e) => setFormData((prev) => ({ ...prev, displayOrder: e.target.value }))}
              placeholder="0"
            />
          </div>

          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="category-refinements">
                {t('admin.categories.form.hasRefinements')}
              </Label>
              <p className="text-xs text-muted-foreground">
                {t('admin.categories.form.hasRefinementsHint')}
              </p>
            </div>
            <Switch
              id="category-refinements"
              checked={formData.hasRefinements}
              onCheckedChange={(checked) =>
                setFormData((prev) => ({ ...prev, hasRefinements: checked }))
              }
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            {t('common.cancel')}
          </Button>
          <Button onClick={onSave} disabled={saving}>
            {saving ? t('common.saving') : isEditing ? t('common.save') : t('common.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
