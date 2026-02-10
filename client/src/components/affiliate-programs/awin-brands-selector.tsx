import { Label } from '@/components/ui/label';
import { useTranslation } from 'react-i18next';

interface AwinBrandsSelectorProps {
  brands: string[];
  selectedBrands: string[];
  onToggleBrand: (brand: string) => void;
}

/**
 * Checkbox grid for selecting Awin advertiser brands.
 * Displays available brands with current selection count.
 */
export function AwinBrandsSelector({
  brands,
  selectedBrands,
  onToggleBrand,
}: AwinBrandsSelectorProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-3 pt-4 border-t">
      <Label className="text-sm font-medium">
        {t('pages.affiliatePrograms.awin.selectBrands')}
      </Label>
      <div className="grid grid-cols-2 gap-2 max-h-48 overflow-y-auto p-2 border rounded-md">
        {brands.map((brand) => (
          <div key={brand} className="flex items-center space-x-2">
            <input
              type="checkbox"
              id={`awin-${brand}`}
              checked={selectedBrands.includes(brand)}
              onChange={() => onToggleBrand(brand)}
              className="rounded border-gray-300"
            />
            <Label htmlFor={`awin-${brand}`} className="text-sm cursor-pointer">
              {brand}
            </Label>
          </div>
        ))}
      </div>
      {selectedBrands.length > 0 && (
        <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          {t('pages.affiliatePrograms.awin.brandsSelected', {
            count: selectedBrands.length,
          })}
        </p>
      )}
    </div>
  );
}
