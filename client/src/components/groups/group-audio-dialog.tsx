import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
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
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { AlertCircle, Mic, Upload } from 'lucide-react';
import type { ChangeEvent } from 'react';
import { useTranslation } from 'react-i18next';

interface GroupAudioDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  audioFile: File | null;
  onAudioUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onRemoveAudio: () => void;
  audioPtt: boolean;
  onAudioPttChange: (value: boolean) => void;
  audioCaption: string;
  onAudioCaptionChange: (value: string) => void;
  sendingAudio: boolean;
  selectedGroupsCount: number;
  onSubmit: () => void;
  onNoGroupsSelected: () => void;
}

/**
 * Dialog para enviar áudio (PTT) em massa para grupos.
 */
export function GroupAudioDialog({
  open,
  onOpenChange,
  audioFile,
  onAudioUpload,
  onRemoveAudio,
  audioPtt,
  onAudioPttChange,
  audioCaption,
  onAudioCaptionChange,
  sendingAudio,
  selectedGroupsCount,
  onSubmit,
  onNoGroupsSelected,
}: GroupAudioDialogProps) {
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
        <Button variant="outline" className="w-full min-w-0 text-xs md:text-sm">
          <Mic className="shrink-0 mr-1 md:mr-2 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">{t('pages.groups.bulkActions.sendAudio')}</span>
          <span className="sm:hidden truncate">{t('pages.groups.bulkActions.sendAudioShort')}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('pages.groups.dialogs.sendAudio.title')}</DialogTitle>
          <DialogDescription>{t('pages.groups.dialogs.sendAudio.description')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {selectedGroupsCount > 10 && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" aria-hidden="true" />
              <AlertTitle>{t('pages.groups.dialogs.sendAudio.limitTitle')}</AlertTitle>
              <AlertDescription>
                {t('pages.groups.dialogs.sendAudio.limitDescription')}
              </AlertDescription>
            </Alert>
          )}

          <div className="space-y-2">
            <Label htmlFor="audio">{t('pages.groups.dialogs.sendAudio.selectAudio')}</Label>
            <div className="flex items-center gap-3">
              <Input
                id="audio"
                type="file"
                accept="audio/*,.ogg"
                onChange={onAudioUpload}
                className="flex-1"
              />
              <Upload className="h-5 w-5 text-muted-foreground" aria-hidden="true" />
            </div>
            {audioFile && (
              <div className="flex items-center justify-between text-sm bg-muted p-2 rounded">
                <span className="truncate flex-1">{audioFile.name}</span>
                <Button variant="ghost" size="sm" onClick={onRemoveAudio}>
                  {t('common.remove')}
                </Button>
              </div>
            )}
            <p className="text-xs text-muted-foreground">
              {t('pages.groups.dialogs.sendAudio.hint')}
            </p>
          </div>

          <div className="flex items-center justify-between space-x-2 rounded-lg border p-4">
            <div className="space-y-0.5">
              <Label className="font-medium">{t('pages.groups.dialogs.sendAudio.pttLabel')}</Label>
              <p className="text-sm text-muted-foreground">
                {t('pages.groups.dialogs.sendAudio.pttDescription')}
              </p>
            </div>
            <Switch checked={audioPtt} onCheckedChange={onAudioPttChange} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="audioCaption">{t('pages.groups.dialogs.sendAudio.captionLabel')}</Label>
            <Textarea
              id="audioCaption"
              placeholder={t('pages.groups.dialogs.sendAudio.captionPlaceholder')}
              value={audioCaption}
              onChange={(e) => onAudioCaptionChange(e.target.value)}
              rows={3}
            />
          </div>

          <Button
            className="w-full"
            onClick={onSubmit}
            disabled={
              sendingAudio || selectedGroupsCount === 0 || selectedGroupsCount > 10 || !audioFile
            }
          >
            {sendingAudio
              ? t('pages.groups.dialogs.sendAudio.sending')
              : t('pages.groups.dialogs.sendAudio.submit')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
