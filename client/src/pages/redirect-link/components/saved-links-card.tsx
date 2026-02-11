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
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Bookmark, Copy, Edit, ExternalLink, Eye, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { SavedRedirectLink } from '../types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface SavedLinksCardProps {
  savedLinks: SavedRedirectLink[];
  onCopy: (slug: string) => void;
  onEdit: (link: SavedRedirectLink) => void;
  onDelete: (id: string) => void;
}

export function SavedLinksCard({ savedLinks, onCopy, onEdit, onDelete }: SavedLinksCardProps) {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
          <Bookmark
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wine }}
            aria-hidden="true"
          />
        </div>
        <div>
          <span
            className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
            style={{ color: colors.wineTextLight }}
          >
            {t('pages.redirectLink.savedLinks.title')}
          </span>
          <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
            {t('pages.redirectLink.savedLinks.description')}
          </p>
        </div>
      </div>

      <div className="space-y-3">
        {savedLinks.length > 0 ? (
          savedLinks.map((link) => (
            <div
              key={link.id}
              className="p-3 xs:p-4 rounded-lg xs:rounded-xl border"
              style={{ backgroundColor: '#ffffff', borderColor: colors.wineBorder }}
            >
              <div className="flex items-start justify-between mb-2">
                <div className="flex-1">
                  <h3 className="font-medium text-sm mb-1">
                    {link.name || t('common.notAvailable')}
                  </h3>
                  <div className="flex items-center gap-2 mb-1">
                    <code className="text-xs" style={{ color: colors.wineTextMuted }}>
                      /r/{link.slug}
                    </code>
                    {link.is_active && (
                      <Badge variant="outline" className="text-xs">
                        {t('pages.redirectLink.savedLinks.active')}
                      </Badge>
                    )}
                  </div>
                  <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                    {link.total_clicks} {t('pages.redirectLink.savedLinks.clicks')} •{' '}
                    {new Date(link.created_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Button
                  size="sm"
                  variant="outline"
                  className="w-full sm:flex-1"
                  onClick={() => onCopy(link.slug)}
                >
                  <Copy className="h-3 w-3 mr-1" />
                  {t('pages.redirectLink.savedLinks.copy')}
                </Button>
                <div className="flex gap-1 justify-end">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button size="sm" variant="ghost" onClick={() => onEdit(link)}>
                        <Edit className="h-3 w-3" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{t('pages.redirectLink.buttons.editTooltip')}</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => window.open(`/r/${link.slug}`, '_blank')}
                      >
                        <ExternalLink className="h-3 w-3" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{t('pages.redirectLink.buttons.openTooltip')}</TooltipContent>
                  </Tooltip>
                  <AlertDialog>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <AlertDialogTrigger asChild>
                          <Button size="sm" variant="ghost">
                            <Trash2 className="h-3 w-3" />
                          </Button>
                        </AlertDialogTrigger>
                      </TooltipTrigger>
                      <TooltipContent>{t('pages.redirectLink.buttons.deleteTooltip')}</TooltipContent>
                    </Tooltip>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>
                          {t('pages.redirectLink.deleteDialog.title')}
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                          {t('pages.redirectLink.deleteDialog.description')}
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>{t('pages.redirectLink.buttons.cancel')}</AlertDialogCancel>
                        <AlertDialogAction onClick={() => onDelete(link.id)}>
                          {t('pages.redirectLink.buttons.deleteTooltip')}
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              </div>
            </div>
          ))
        ) : (
          <div className="text-center py-8">
            <Eye className="h-12 w-12 mx-auto mb-2 opacity-20" style={{ color: colors.wine }} />
            <p className="text-sm md:text-base" style={{ color: colors.wineTextMuted }}>
              {t('pages.redirectLink.savedLinks.empty')}
            </p>
            <p className="text-xs mt-1" style={{ color: colors.wineTextMuted }}>
              <span className="hidden lg:inline">
                {t('pages.redirectLink.savedLinks.emptyHintDesktop')}
              </span>
              <span className="lg:hidden">
                {t('pages.redirectLink.savedLinks.emptyHintMobile')}
              </span>
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
