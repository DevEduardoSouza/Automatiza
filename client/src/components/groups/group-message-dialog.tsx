import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
} from '@/components/ui/command';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Textarea } from '@/components/ui/textarea';
import type { GroupParticipant } from '@/hooks/use-groups-connect';
import { AtSign, Loader2, MessageSquare, Sparkles, Upload } from 'lucide-react';
import type { ChangeEvent } from 'react';
import { useTranslation } from 'react-i18next';

interface GroupMessageDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  message: string;
  onMessageChange: (value: string) => void;
  mentionOpen: boolean;
  onMentionOpenChange: (value: boolean) => void;
  onInsertMention: (mention: string) => void;
  participants: GroupParticipant[];
  loadingParticipants: boolean;
  isEnhancing: boolean;
  onEnhanceMessage: () => void;
  mediaFile: File | null;
  onMediaUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onRemoveMedia: () => void;
  /** WF20: Disables submit button while message is being sent */
  isSending?: boolean;
  selectedGroupsCount: number;
  onSubmit: () => void;
  onNoGroupsSelected: () => void;
}

/**
 * Dialog para enviar mensagens em massa para grupos.
 */
export function GroupMessageDialog({
  open,
  onOpenChange,
  message,
  onMessageChange,
  mentionOpen,
  onMentionOpenChange,
  onInsertMention,
  participants,
  loadingParticipants,
  isEnhancing,
  onEnhanceMessage,
  mediaFile,
  onMediaUpload,
  onRemoveMedia,
  isSending = false,
  selectedGroupsCount,
  onSubmit,
  onNoGroupsSelected,
}: GroupMessageDialogProps) {
  const { t } = useTranslation();

  const handleOpenChange = (newOpen: boolean) => {
    if (newOpen && selectedGroupsCount === 0) {
      onNoGroupsSelected();
      return;
    }
    onOpenChange(newOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <button
          type="button"
          className="inline-flex w-full min-w-0 cursor-pointer items-center justify-center gap-1.5 rounded-full border border-gray-300 bg-white px-3 py-1.5 text-xs xs:px-4 xs:py-2 xs:text-[13px] md:text-sm font-medium text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
        >
          <MessageSquare className="shrink-0 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">
            {t('pages.groups.bulkActions.sendMessage')}
          </span>
          <span className="sm:hidden truncate">
            {t('pages.groups.bulkActions.sendMessageShort')}
          </span>
        </button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('pages.groups.dialogs.sendMessage.title')}</DialogTitle>
          <DialogDescription>{t('pages.groups.dialogs.sendMessage.description')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="message">{t('pages.groups.form.message')}</Label>
              <Popover open={mentionOpen} onOpenChange={onMentionOpenChange}>
                <PopoverTrigger asChild>
                  <Button type="button" variant="outline" size="sm" className="rounded-full">
                    <AtSign className="mr-2 h-4 w-4" aria-hidden="true" />
                    {t('pages.groups.actions.mention')}
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-[280px] p-0" align="end">
                  <Command>
                    <CommandInput placeholder={t('pages.groups.form.searchMention')} />
                    <CommandEmpty>{t('pages.groups.form.noOptionFound')}</CommandEmpty>
                    <CommandGroup heading={t('pages.groups.form.mentionOptions')}>
                      <CommandItem
                        onSelect={() => onInsertMention('@todos ')}
                        className="cursor-pointer"
                      >
                        <AtSign className="mr-2 h-4 w-4" aria-hidden="true" />
                        {t('pages.groups.actions.mentionAll')}
                      </CommandItem>
                    </CommandGroup>
                    <CommandGroup heading={t('pages.groups.form.participants')}>
                      {loadingParticipants ? (
                        <div className="flex items-center justify-center gap-2 py-4 text-sm text-muted-foreground">
                          <Loader2 className="h-4 w-4 animate-spin" />
                          {t('pages.groups.form.loadingParticipants')}
                        </div>
                      ) : participants.length === 0 ? (
                        <div className="py-4 text-center text-sm text-muted-foreground">
                          {t('pages.groups.form.noParticipants')}
                        </div>
                      ) : (
                        participants.map((p) => (
                          <CommandItem
                            key={p.phone_number}
                            onSelect={() => onInsertMention(`@${p.phone_number} `)}
                            className="cursor-pointer"
                          >
                            <AtSign className="mr-2 h-4 w-4 shrink-0" aria-hidden="true" />
                            <span className="truncate">{p.phone_number}</span>
                            {(p.is_admin || p.is_super_admin) && (
                              <Badge
                                variant="secondary"
                                className="ml-auto text-[10px] px-1.5 py-0"
                              >
                                {t('pages.groups.form.admin')}
                              </Badge>
                            )}
                          </CommandItem>
                        ))
                      )}
                    </CommandGroup>
                  </Command>
                </PopoverContent>
              </Popover>
            </div>
            <Textarea
              id="message"
              placeholder={t('pages.groups.form.messagePlaceholder')}
              value={message}
              onChange={(e) => onMessageChange(e.target.value)}
              rows={4}
            />

            {message.includes('@todos') && (
              <Alert className="bg-blue-50 border-blue-200">
                <AtSign className="h-4 w-4 text-blue-600" aria-hidden="true" />
                <AlertDescription className="text-blue-800">
                  <strong>{t('pages.groups.dialogs.sendMessage.mentionDetected')}</strong>{' '}
                  {t('pages.groups.dialogs.sendMessage.mentionDescription')}
                </AlertDescription>
              </Alert>
            )}

            {!message.includes('@todos') && /@\d{8,15}/.test(message) && (
              <Alert className="bg-blue-50 border-blue-200">
                <AtSign className="h-4 w-4 text-blue-600" aria-hidden="true" />
                <AlertDescription className="text-blue-800">
                  <strong>{t('pages.groups.dialogs.sendMessage.individualMentionDetected')}</strong>{' '}
                  {t('pages.groups.dialogs.sendMessage.individualMentionDescription')}
                </AlertDescription>
              </Alert>
            )}

            <Button
              variant="outline"
              size="sm"
              onClick={onEnhanceMessage}
              disabled={isEnhancing || !message.trim()}
              className="w-full rounded-full"
            >
              <Sparkles className="mr-2 h-4 w-4" aria-hidden="true" />
              {isEnhancing
                ? t('pages.groups.dialogs.sendMessage.enhancing')
                : t('pages.groups.dialogs.sendMessage.enhanceWithAI')}
            </Button>
          </div>

          <div className="space-y-2">
            <Label htmlFor="media">{t('pages.groups.dialogs.sendMessage.attachMedia')}</Label>
            <div className="flex items-center gap-3">
              <Input
                id="media"
                type="file"
                accept="image/*,video/*"
                onChange={onMediaUpload}
                className="flex-1"
              />
              <Upload className="h-5 w-5 text-muted-foreground" aria-hidden="true" />
            </div>
            {mediaFile && (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-sm bg-muted p-2 rounded">
                  <span className="truncate flex-1">{mediaFile.name}</span>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="rounded-full"
                    onClick={onRemoveMedia}
                  >
                    {t('common.remove')}
                  </Button>
                </div>
              </div>
            )}
          </div>

          <button
            type="button"
            onClick={onSubmit}
            disabled={isSending || !message.trim() || selectedGroupsCount === 0}
            className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              {isSending ? t('common.loading') : t('pages.groups.dialogs.sendMessage.submit')}
            </span>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
