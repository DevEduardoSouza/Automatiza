import { Button } from '@/components/ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
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
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';
import type { LocalCategory, LocalSubcategory } from '@/hooks/use-categories';
import { cn } from '@/lib/utils';
import type { TFunction } from 'i18next';
import { Check, ChevronsUpDown, Languages } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { SubcategoryFormData, SubcategoryTranslations } from '../types/categories.types';

interface SubcategoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  parentCategory: LocalCategory | null;
  editingSubcategory: LocalSubcategory | null;
  formData: SubcategoryFormData;
  setFormData: React.Dispatch<React.SetStateAction<SubcategoryFormData>>;
  onSave: () => Promise<void>;
  saving: boolean;
  t: TFunction;
  allCategories?: LocalCategory[];
  translations?: SubcategoryTranslations;
  onTranslationsChange?: (translations: SubcategoryTranslations) => void;
}

export function SubcategoryDialog({
  open,
  onOpenChange,
  parentCategory,
  editingSubcategory,
  formData,
  setFormData,
  onSave,
  saving,
  t,
  allCategories = [],
  translations = {},
  onTranslationsChange,
}: SubcategoryDialogProps) {
  const [groupPopoverOpen, setGroupPopoverOpen] = useState(false);
  const isEditing = !!editingSubcategory;

  // Collect existing group names from all categories for suggestions
  const existingGroups = useMemo(() => {
    const groups = new Set<string>();
    for (const category of allCategories) {
      for (const sub of category.subcategories) {
        if (sub.groupName) {
          groups.add(sub.groupName);
        }
      }
    }
    return Array.from(groups).sort();
  }, [allCategories]);

  // Generate displayKey based on parentCategory, groupName, and name
  const generatedDisplayKey = useMemo(() => {
    if (!parentCategory?.name || !formData.name) return '';
    if (formData.groupName) {
      return `${parentCategory.name} - ${formData.groupName} (${formData.name})`;
    }
    return `${parentCategory.name} - ${formData.name}`;
  }, [parentCategory?.name, formData.groupName, formData.name]);

  // Auto-generate displayKey when name or groupName changes
  const handleNameChange = (name: string) => {
    setFormData((prev) => {
      const newDisplayKey = generateDisplayKey(parentCategory?.name || '', prev.groupName, name);
      return { ...prev, name, displayKey: newDisplayKey };
    });
  };

  const handleGroupNameChange = (groupName: string) => {
    setFormData((prev) => {
      const newDisplayKey = generateDisplayKey(parentCategory?.name || '', groupName, prev.name);
      return { ...prev, groupName, displayKey: newDisplayKey };
    });
  };

  const handleTranslationChange = (lang: 'en' | 'es', value: string) => {
    onTranslationsChange?.({
      ...translations,
      [lang]: value,
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>
            {isEditing
              ? t('admin.categories.dialog.editSubcategory')
              : t('admin.categories.dialog.createSubcategory')}
          </DialogTitle>
          <DialogDescription>
            {parentCategory && (
              <span>
                {t('admin.categories.dialog.parentCategory')}:{' '}
                <strong>{parentCategory.name}</strong>
              </span>
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          {/* Name field */}
          <div className="space-y-2">
            <Label htmlFor="subcategory-name">
              {t('admin.categories.form.name')} <span className="text-destructive">*</span>
            </Label>
            <Input
              id="subcategory-name"
              value={formData.name}
              onChange={(e) => handleNameChange(e.target.value)}
              placeholder={t('admin.categories.form.subcategoryNamePlaceholder')}
            />
          </div>

          {/* Group name with combobox/autocomplete */}
          <div className="space-y-2">
            <Label htmlFor="subcategory-group">{t('admin.categories.form.groupName')}</Label>
            <Popover open={groupPopoverOpen} onOpenChange={setGroupPopoverOpen}>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  aria-expanded={groupPopoverOpen}
                  className="w-full justify-between font-normal"
                >
                  {formData.groupName || (
                    <span className="text-muted-foreground">
                      {t('admin.categories.form.groupNamePlaceholder')}
                    </span>
                  )}
                  <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
                <Command>
                  <CommandInput
                    placeholder={t(
                      'admin.categories.form.searchOrCreateGroup',
                      'Buscar ou criar grupo...'
                    )}
                    value={formData.groupName}
                    onValueChange={handleGroupNameChange}
                  />
                  <CommandList>
                    <CommandEmpty>
                      <button
                        type="button"
                        className="w-full text-left px-2 py-1.5 text-sm hover:bg-accent rounded cursor-pointer"
                        onClick={() => {
                          setGroupPopoverOpen(false);
                        }}
                      >
                        {t('admin.categories.form.useGroupName', 'Usar')}: "{formData.groupName}"
                      </button>
                    </CommandEmpty>
                    {existingGroups.length > 0 && (
                      <CommandGroup
                        heading={t('admin.categories.form.existingGroups', 'Grupos existentes')}
                      >
                        {existingGroups.map((group) => (
                          <CommandItem
                            key={group}
                            value={group}
                            onSelect={() => {
                              handleGroupNameChange(group);
                              setGroupPopoverOpen(false);
                            }}
                          >
                            <Check
                              className={cn(
                                'mr-2 h-4 w-4',
                                formData.groupName === group ? 'opacity-100' : 'opacity-0'
                              )}
                            />
                            {group}
                          </CommandItem>
                        ))}
                      </CommandGroup>
                    )}
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
            <p className="text-xs text-muted-foreground">
              {t('admin.categories.form.groupNameHint')}
            </p>
          </div>

          {/* Display order */}
          <div className="space-y-2">
            <Label htmlFor="subcategory-order">{t('admin.categories.form.displayOrder')}</Label>
            <Input
              id="subcategory-order"
              type="number"
              value={formData.displayOrder}
              onChange={(e) => setFormData((prev) => ({ ...prev, displayOrder: e.target.value }))}
              placeholder="0"
            />
          </div>

          {/* Translations section */}
          {onTranslationsChange && (
            <>
              <Separator />
              <div className="space-y-3">
                <Label className="flex items-center gap-2">
                  <Languages className="h-4 w-4" />
                  {t('admin.categories.form.translations', 'Traduções')}
                </Label>
                <p className="text-xs text-muted-foreground">
                  {t(
                    'admin.categories.form.translationsHint',
                    'Adicione traduções para outros idiomas (opcional)'
                  )}
                </p>

                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <span className="w-14 text-sm font-medium text-muted-foreground">PT-BR</span>
                    <Input value={formData.name} disabled className="bg-muted" />
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="w-14 text-sm font-medium text-muted-foreground">EN</span>
                    <Input
                      value={translations.en || ''}
                      onChange={(e) => handleTranslationChange('en', e.target.value)}
                      placeholder="English translation"
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="w-14 text-sm font-medium text-muted-foreground">ES</span>
                    <Input
                      value={translations.es || ''}
                      onChange={(e) => handleTranslationChange('es', e.target.value)}
                      placeholder="Traducción al español"
                    />
                  </div>
                </div>
              </div>
            </>
          )}

          {/* Preview */}
          {generatedDisplayKey && (
            <div className="bg-muted/50 p-3 rounded-md">
              <Label className="text-xs text-muted-foreground">
                {t('admin.categories.form.preview', 'Preview')}
              </Label>
              <p className="font-medium text-sm mt-1">{generatedDisplayKey}</p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            {t('common.cancel')}
          </Button>
          <Button onClick={onSave} disabled={saving || !formData.name}>
            {saving ? t('common.saving') : isEditing ? t('common.save') : t('common.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * Generates display key in the format: "Category - Group (Name)" or "Category - Name"
 */
function generateDisplayKey(categoryName: string, groupName: string, name: string): string {
  if (!categoryName || !name) return '';

  if (groupName) {
    return `${categoryName} - ${groupName} (${name})`;
  }
  return `${categoryName} - ${name}`;
}
