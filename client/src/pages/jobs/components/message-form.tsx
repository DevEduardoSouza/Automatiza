import { Button } from '@/components/ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
} from '@/components/ui/command';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import { AtSign, Check, Loader2, Plus, Sparkles } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

interface MessageFormProps {
  payload: string;
  mediaFile: File | null;
  mentionOpen: boolean;
  isEnhancing: boolean;
  onPayloadChange: (value: string) => void;
  onMediaFileChange: (file: File | null) => void;
  onMentionOpenChange: (open: boolean) => void;
  onInsertMention: (mention: string) => void;
  onEnhanceMessage: () => void;
}

/**
 * Formulário de mensagem com menções, IA e upload de mídia.
 */
export function MessageForm({
  payload,
  mediaFile,
  mentionOpen,
  isEnhancing,
  onPayloadChange,
  onMediaFileChange,
  onMentionOpenChange,
  onInsertMention,
  onEnhanceMessage,
}: MessageFormProps) {
  const { t } = useTranslation();
  const [phoneInput, setPhoneInput] = useState('');

  const handleAddPhoneMention = () => {
    const cleaned = phoneInput.replace(/\D/g, '');
    if (cleaned.length >= 8 && cleaned.length <= 15) {
      onInsertMention(`@${cleaned} `);
      setPhoneInput('');
    }
  };

  return (
    <div className="space-y-3 xs:space-y-4">
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label htmlFor="payload" className="cursor-pointer">
            {t('pages.jobs.form.message')}
          </Label>
          <Popover open={mentionOpen} onOpenChange={onMentionOpenChange}>
            <PopoverTrigger asChild>
              <Button type="button" variant="outline" size="sm" className="cursor-pointer text-xs xs:text-sm">
                <AtSign className="mr-1 xs:mr-2 h-3.5 w-3.5 xs:h-4 xs:w-4" />
                {t('pages.jobs.form.mention')}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[260px] p-0">
              <Command>
                <CommandInput placeholder={t('common.search')} />
                <CommandEmpty>{t('common.none')}</CommandEmpty>
                <CommandGroup>
                  <CommandItem
                    onSelect={() => onInsertMention('@todos ')}
                    className="cursor-pointer"
                  >
                    <Check className={cn('mr-2 h-4 w-4', 'opacity-0')} />
                    @todos
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
          id="payload"
          placeholder={t('pages.jobs.form.messagePlaceholder')}
          value={payload}
          onChange={(e) => onPayloadChange(e.target.value)}
          rows={3}
          className="cursor-text min-h-[70px] xs:min-h-[80px] lg:min-h-[120px] xl:min-h-[140px]"
        />
        {payload.includes('@todos') && (
          <div className="flex items-start gap-1.5 xs:gap-2 p-2 xs:p-3 bg-primary/10 border border-primary/20 rounded-lg">
            <AtSign className="h-4 w-4 text-primary mt-0.5 shrink-0" />
            <p className="text-xs xs:text-sm text-primary">
              <strong>{t('pages.jobs.form.mentionDetected')}</strong>{' '}
              {t('pages.jobs.form.mentionHint')}
            </p>
          </div>
        )}
        {!payload.includes('@todos') && /@\d{8,15}/.test(payload) && (
          <div className="flex items-start gap-1.5 xs:gap-2 p-2 xs:p-3 bg-primary/10 border border-primary/20 rounded-lg">
            <AtSign className="h-4 w-4 text-primary mt-0.5 shrink-0" />
            <p className="text-xs xs:text-sm text-primary">
              <strong>{t('pages.jobs.form.individualMentionDetected')}</strong>{' '}
              {t('pages.jobs.form.individualMentionHint')}
            </p>
          </div>
        )}
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onEnhanceMessage}
          disabled={isEnhancing || !payload.trim()}
          className="w-full text-xs xs:text-sm h-8 xs:h-9"
        >
          {isEnhancing ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              {t('pages.jobs.form.enhancing')}
            </>
          ) : (
            <>
              <Sparkles className="mr-2 h-4 w-4" />
              {t('pages.jobs.form.enhanceWithAI')}
            </>
          )}
        </Button>
      </div>

      <div className="space-y-2">
        <Label htmlFor="media" className="cursor-pointer">
          {t('pages.jobs.form.attachMedia')}
        </Label>
        <Input
          id="media"
          type="file"
          accept="image/*,video/*"
          onChange={(e) => onMediaFileChange(e.target.files?.[0] || null)}
          className="cursor-pointer"
        />
        {mediaFile && (
          <p className="text-xs xs:text-sm text-muted-foreground truncate">
            {t('pages.jobs.form.fileSelected')} {mediaFile.name}
          </p>
        )}
      </div>
    </div>
  );
}
