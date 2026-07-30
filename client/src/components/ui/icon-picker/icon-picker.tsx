import { Check, ChevronsUpDown } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';
import { ICON_CATEGORIES, ICON_MAP, type IconName } from './icons-map';

interface IconPickerProps {
  value?: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
}

const CATEGORY_LABELS: Record<string, string> = {
  electronics: 'Eletrônicos',
  fashion: 'Moda',
  home: 'Casa',
  beauty: 'Beleza',
  sports: 'Esportes',
  education: 'Educação',
  kids: 'Infantil',
  automotive: 'Automotivo',
  pets: 'Pets',
  general: 'Geral',
};

export function IconPicker({ value, onChange, placeholder, disabled }: IconPickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const { t } = useTranslation();

  const SelectedIcon = value ? ICON_MAP[value] : null;

  const filteredCategories = useMemo(() => {
    if (!search) {
      return ICON_CATEGORIES;
    }

    const searchLower = search.toLowerCase();
    const result: Record<string, string[]> = {};

    for (const [category, icons] of Object.entries(ICON_CATEGORIES)) {
      const filtered = icons.filter((icon) => icon.toLowerCase().includes(searchLower));
      if (filtered.length > 0) {
        result[category] = filtered;
      }
    }

    return result;
  }, [search]);

  const hasResults = Object.keys(filteredCategories).length > 0;

  // `modal`: sem isso o scroll-lock do Dialog em volta bloqueia a rolagem por toque
  // da lista de icones (o portal do popover fica fora do container do Dialog).
  return (
    <Popover modal open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          aria-expanded={open}
          className="w-full justify-between"
          disabled={disabled}
        >
          <span className="flex items-center gap-2">
            {SelectedIcon ? (
              <>
                <SelectedIcon className="h-4 w-4" />
                <span>{value}</span>
              </>
            ) : (
              <span className="text-muted-foreground">
                {placeholder || t('admin.categories.form.selectIcon', 'Selecione um ícone')}
              </span>
            )}
          </span>
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={t('admin.categories.form.searchIcon', 'Buscar ícone...')}
            value={search}
            onValueChange={setSearch}
          />
          <CommandList className="max-h-80">
            {!hasResults && (
              <CommandEmpty>
                {t('admin.categories.form.noIconsFound', 'Nenhum ícone encontrado')}
              </CommandEmpty>
            )}
            {Object.entries(filteredCategories).map(([category, icons]) => (
              <CommandGroup
                key={category}
                heading={CATEGORY_LABELS[category] || category}
                className="px-1"
              >
                <div className="grid grid-cols-6 gap-1 p-1">
                  {icons.map((iconName) => {
                    const IconComponent = ICON_MAP[iconName as IconName];
                    if (!IconComponent) return null;

                    const isSelected = value === iconName;

                    return (
                      <CommandItem
                        key={iconName}
                        value={iconName}
                        onSelect={() => {
                          onChange(iconName);
                          setOpen(false);
                          setSearch('');
                        }}
                        className={cn(
                          'flex items-center justify-center p-2 cursor-pointer rounded-md',
                          isSelected && 'bg-accent'
                        )}
                      >
                        <div className="relative">
                          <IconComponent className="h-5 w-5" />
                          {isSelected && (
                            <Check className="h-3 w-3 absolute -top-1 -right-1 text-primary" />
                          )}
                        </div>
                      </CommandItem>
                    );
                  })}
                </div>
              </CommandGroup>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
