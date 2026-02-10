import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import type { LocalCategory, LocalSubcategory } from '@/hooks/use-categories';
import { cn } from '@/lib/utils';
import type { TFunction } from 'i18next';
import {
  ChevronDown,
  ChevronRight,
  Edit,
  Lock,
  MoreHorizontal,
  Plus,
  Power,
  Trash2,
} from 'lucide-react';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface CategoryCardProps {
  category: LocalCategory;
  isExpanded: boolean;
  onToggleExpanded: () => void;
  onEditCategory: (category: LocalCategory) => void;
  onDeleteCategory: (id: string, isSystem: boolean) => void;
  onToggleActive: (category: LocalCategory) => void;
  onAddSubcategory: (category: LocalCategory) => void;
  onEditSubcategory: (category: LocalCategory, subcategory: LocalSubcategory) => void;
  onDeleteSubcategory: (id: string, isSystem: boolean) => void;
  onToggleSubcategoryActive: (subcategory: LocalSubcategory) => void;
  t: TFunction;
}

export function CategoryCard({
  category,
  isExpanded,
  onToggleExpanded,
  onEditCategory,
  onDeleteCategory,
  onToggleActive,
  onAddSubcategory,
  onEditSubcategory,
  onDeleteSubcategory,
  onToggleSubcategoryActive,
  t,
}: CategoryCardProps) {
  const hasSubcategories = category.subcategories.length > 0;

  // Group subcategories by groupName
  const groupedSubcategories = category.subcategories.reduce(
    (acc, sub) => {
      const group = sub.groupName || t('admin.categories.noGroup', 'Sem grupo');
      if (!acc[group]) {
        acc[group] = [];
      }
      acc[group].push(sub);
      return acc;
    },
    {} as Record<string, LocalSubcategory[]>
  );

  const sortedGroups = Object.keys(groupedSubcategories).sort();

  return (
    <div
      className={cn('rounded-lg xs:rounded-xl border', !category.isActive && 'opacity-60')}
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="p-3 xs:p-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {category.hasRefinements && (
              <Button
                variant="ghost"
                size="sm"
                className="h-6 w-6 p-0"
                onClick={onToggleExpanded}
                aria-label={isExpanded ? t('common.collapse') : t('common.expand')}
              >
                {isExpanded ? (
                  <ChevronDown className="h-4 w-4" />
                ) : (
                  <ChevronRight className="h-4 w-4" />
                )}
              </Button>
            )}
            <span className="font-medium">{category.name}</span>
            {category.isSystem && (
              <Lock
                className="h-3.5 w-3.5"
                style={{ color: colors.wineTextMuted }}
                aria-label={t('admin.categories.systemCategory')}
              />
            )}
            {!category.isActive && (
              <Badge variant="secondary" className="text-xs">
                {t('common.inactive')}
              </Badge>
            )}
          </div>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                <MoreHorizontal className="h-4 w-4" />
                <span className="sr-only">{t('common.actions')}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => onEditCategory(category)}>
                <Edit className="h-4 w-4 mr-2" />
                {t('common.edit')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => onToggleActive(category)}>
                <Power className="h-4 w-4 mr-2" />
                {category.isActive ? t('common.deactivate') : t('common.activate')}
              </DropdownMenuItem>
              {!category.isSystem && (
                <DropdownMenuItem
                  className="text-destructive focus:text-destructive"
                  onClick={() => onDeleteCategory(category.id, category.isSystem)}
                >
                  <Trash2 className="h-4 w-4 mr-2" />
                  {t('common.delete')}
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Subcategories section */}
      {category.hasRefinements && isExpanded && (
        <div className="pt-0 pb-4 px-4">
          <div className="space-y-2 xs:space-y-3 pl-6 xs:pl-8 border-l ml-2 xs:ml-3">
            {sortedGroups.map((groupName) => (
              <div key={groupName} className="space-y-1">
                <div
                  className="text-xs font-medium uppercase tracking-wider"
                  style={{ color: colors.wineTextMuted }}
                >
                  {groupName}
                </div>
                <div className="space-y-1">
                  {groupedSubcategories[groupName]
                    .sort((a, b) => a.displayOrder - b.displayOrder)
                    .map((subcategory) => (
                      <SubcategoryItem
                        key={subcategory.id}
                        subcategory={subcategory}
                        onEdit={() => onEditSubcategory(category, subcategory)}
                        onDelete={() => onDeleteSubcategory(subcategory.id, subcategory.isSystem)}
                        onToggleActive={() => onToggleSubcategoryActive(subcategory)}
                        t={t}
                      />
                    ))}
                </div>
              </div>
            ))}

            {/* Add subcategory button */}
            <Button
              variant="ghost"
              size="sm"
              className="h-8 text-xs"
              onClick={() => onAddSubcategory(category)}
            >
              <Plus className="h-3.5 w-3.5 mr-1" />
              {t('admin.categories.addSubcategory')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

interface SubcategoryItemProps {
  subcategory: LocalSubcategory;
  onEdit: () => void;
  onDelete: () => void;
  onToggleActive: () => void;
  t: TFunction;
}

function SubcategoryItem({
  subcategory,
  onEdit,
  onDelete,
  onToggleActive,
  t,
}: SubcategoryItemProps) {
  return (
    <div
      className={cn(
        'group flex items-center justify-between py-1.5 px-2 rounded-md transition-colors',
        !subcategory.isActive && 'opacity-60'
      )}
      style={{ ['--hover-bg' as string]: colors.wineBgHover }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLElement).style.backgroundColor = colors.wineBgHover;
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLElement).style.backgroundColor = 'transparent';
      }}
    >
      <div className="flex items-center gap-2">
        <span className="text-sm">{subcategory.name}</span>
        {subcategory.isSystem && (
          <Lock
            className="h-3 w-3"
            style={{ color: colors.wineTextMuted }}
            aria-label={t('admin.categories.systemSubcategory')}
          />
        )}
        {!subcategory.isActive && (
          <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
            {t('common.inactive')}
          </Badge>
        )}
      </div>

      <div className="flex items-center gap-1">
        {/* Inline delete button - visible on hover */}
        {!subcategory.isSystem && (
          <Button
            variant="ghost"
            size="sm"
            className="h-6 w-6 p-0 opacity-0 group-hover:opacity-100 transition-opacity text-destructive hover:text-destructive hover:bg-destructive/10"
            onClick={onDelete}
            aria-label={t('common.delete')}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        )}

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="sm" className="h-6 w-6 p-0">
              <MoreHorizontal className="h-3.5 w-3.5" />
              <span className="sr-only">{t('common.actions')}</span>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={onEdit}>
              <Edit className="h-4 w-4 mr-2" />
              {t('common.edit')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onToggleActive}>
              <Power className="h-4 w-4 mr-2" />
              {subcategory.isActive ? t('common.deactivate') : t('common.activate')}
            </DropdownMenuItem>
            {!subcategory.isSystem && (
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onClick={onDelete}
              >
                <Trash2 className="h-4 w-4 mr-2" />
                {t('common.delete')}
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}
