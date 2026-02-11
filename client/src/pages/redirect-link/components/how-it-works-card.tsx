import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { ChevronDown, ChevronUp, HelpCircle } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

export function HowItWorksCard() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <CollapsibleTrigger asChild>
          <button
            type="button"
            className="flex items-center justify-between w-full gap-2 xs:gap-3 cursor-pointer"
          >
            <div className="flex items-center gap-2 xs:gap-3">
              <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                <HelpCircle
                  className="w-4 h-4 xs:w-5 xs:h-5"
                  style={{ color: colors.wine }}
                  aria-hidden="true"
                />
              </div>
              <span
                className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
                style={{ color: colors.wineTextLight }}
              >
                {t('pages.redirectLink.howItWorks.title')}
              </span>
            </div>
            {open ? (
              <ChevronUp className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
            ) : (
              <ChevronDown className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
            )}
          </button>
        </CollapsibleTrigger>

        <CollapsibleContent>
          <div className="space-y-4 mt-3 xs:mt-4">
            <p className="text-sm md:text-base" style={{ color: '#71717a' }}>
              {t('pages.redirectLink.howItWorks.description')}
            </p>
            <ol
              className="list-decimal list-inside space-y-2 text-sm md:text-base"
              style={{ color: '#71717a' }}
            >
              <li>{t('pages.redirectLink.howItWorks.step1')}</li>
              <li>{t('pages.redirectLink.howItWorks.step2')}</li>
              <li>{t('pages.redirectLink.howItWorks.step3')}</li>
              <li>{t('pages.redirectLink.howItWorks.step4')}</li>
              <li>{t('pages.redirectLink.howItWorks.step5')}</li>
            </ol>
            <div
              className="p-3 md:p-4 rounded-lg border"
              style={{
                backgroundColor: colors.iconBg,
                borderColor: colors.wineBorder,
              }}
            >
              <p className="text-xs md:text-sm">
                <strong>{t('common.tip')}:</strong> {t('pages.redirectLink.howItWorks.tip')}
              </p>
            </div>
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}
