import { Alert, AlertDescription } from '@/components/ui/alert';
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
import { AtSign, MessageSquare, Plus, Sparkles, Upload } from 'lucide-react';
import { type ChangeEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface GroupMessageDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  message: string;
  onMessageChange: (value: string) => void;
  mentionOpen: boolean;
  onMentionOpenChange: (value: boolean) => void;
  onInsertMention: (mention: string) => void;
  isEnhancing: boolean;
  onEnhanceMessage: () => void;
  mediaFile: File | null;
  onMediaUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onRemoveMedia: () => void;
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
  isEnhancing,
  onEnhanceMessage,
  mediaFile,
  onMediaUpload,
  onRemoveMedia,
  selectedGroupsCount,
  onSubmit,
  onNoGroupsSelected,
}: GroupMessageDialogProps) {
  const { t } = useTranslation();
  const [phoneInput, setPhoneInput] = useState('');

  const handleAddPhoneMention = () => {
    const cleaned = phoneInput.replace(/\D/g, '');
    if (cleaned.length >= 8 && cleaned.length <= 15) {
      onInsertMention(`@${cleaned} `);
      setPhoneInput('');
    }
  };

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
        <Button variant="outline" className="w-full min-w-0 text-xs md:text-sm">
          <MessageSquare className="shrink-0 mr-1 md:mr-2 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">{t('pages.groups.bulkActions.sendMessage')}</span>
          <span className="sm:hidden truncate">{t('pages.groups.bulkActions.sendMessageShort')}</span>
        </Button>
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
                  <Button type="button" variant="outline" size="sm">
                    <AtSign className="mr-2 h-4 w-4" aria-hidden="true" />
                    {t('pages.groups.actions.mention')}
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-[260px] p-0" align="end">
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
                  </Command>
                  <div className="border-t p-3 space-y-2">
                    <p className="text-xs font-medium text-muted-foreground">
                      {t('pages.groups.form.mentionByNumber')}
                    </p>
                    <div className="flex gap-2">
                      <Input
                        value={phoneInput}
                        onChange={(e) => setPhoneInput(e.target.value)}
                        placeholder={t('pages.groups.form.phonePlaceholder')}
                        className="h-8 text-sm"
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') {
                            e.preventDefault();
                            handleAddPhoneMention();
                          }
                        }}
                      />
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 px-2 shrink-0"
                        onClick={handleAddPhoneMention}
                        disabled={phoneInput.replace(/\D/g, '').length < 8}
                      >
                        <Plus className="h-4 w-4" aria-hidden="true" />
                      </Button>
                    </div>
                  </div>
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
              className="w-full"
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
                  <Button variant="ghost" size="sm" onClick={onRemoveMedia}>
                    {t('common.remove')}
                  </Button>
                </div>
              </div>
            )}
          </div>

          <Button
            className="w-full"
            onClick={onSubmit}
            disabled={!message.trim() || selectedGroupsCount === 0}
          >
            {t('pages.groups.dialogs.sendMessage.submit')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
