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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Calendar, Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { GroupNameForm } from './group-name-form';
import { GroupSelector } from './group-selector';
import { MessageForm } from './message-form';
import { MlOfferForm } from './ml-offer-form';
import { PollOptionsForm } from './poll-options-form';

interface Group {
  id: string;
  name: string;
  wa_group_id: string;
  is_favorite?: boolean;
}

interface JobFormDialogProps {
  dialogOpen: boolean;
  editingJobId: string | null;
  actionType: string;
  scheduledDate: string;
  scheduledTime: string;
  payload: string;
  mlProductUrl: string;
  mlIncludeImage: boolean;
  mediaFile: File | null;
  isEnhancing: boolean;
  selectedGroups: string[];
  mentionOpen: boolean;
  autoNumberGroups: boolean;
  searchTerm: string;
  pollQuestion: string;
  pollOptions: string[];
  isPending: boolean;
  groups: Group[];
  loadingGroups: boolean;
  onDialogOpenChange: (open: boolean) => void;
  onActionTypeChange: (value: string) => void;
  onScheduledDateChange: (value: string) => void;
  onScheduledTimeChange: (value: string) => void;
  onPayloadChange: (value: string) => void;
  onMlProductUrlChange: (value: string) => void;
  onMlIncludeImageChange: (value: boolean) => void;
  onMediaFileChange: (file: File | null) => void;
  onMentionOpenChange: (open: boolean) => void;
  onAutoNumberGroupsChange: (value: boolean) => void;
  onSearchTermChange: (value: string) => void;
  onPollQuestionChange: (value: string) => void;
  onToggleGroup: (groupId: string) => void;
  onInsertMention: (mention: string) => void;
  onAddPollOption: () => void;
  onRemovePollOption: (index: number) => void;
  onUpdatePollOption: (index: number, value: string) => void;
  onEnhanceMessage: () => void;
  onSubmit: () => void;
}

/**
 * Dialog para criar/editar jobs agendados.
 */
export function JobFormDialog({
  dialogOpen,
  editingJobId,
  actionType,
  scheduledDate,
  scheduledTime,
  payload,
  mlProductUrl,
  mlIncludeImage,
  mediaFile,
  isEnhancing,
  selectedGroups,
  mentionOpen,
  autoNumberGroups,
  searchTerm,
  pollQuestion,
  pollOptions,
  isPending,
  groups,
  loadingGroups,
  onDialogOpenChange,
  onActionTypeChange,
  onScheduledDateChange,
  onScheduledTimeChange,
  onPayloadChange,
  onMlProductUrlChange,
  onMlIncludeImageChange,
  onMediaFileChange,
  onMentionOpenChange,
  onAutoNumberGroupsChange,
  onSearchTermChange,
  onPollQuestionChange,
  onToggleGroup,
  onInsertMention,
  onAddPollOption,
  onRemovePollOption,
  onUpdatePollOption,
  onEnhanceMessage,
  onSubmit,
}: JobFormDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={dialogOpen} onOpenChange={onDialogOpenChange}>
      <DialogTrigger asChild>
        <Button className="flex-1 md:flex-none">
          <Plus className="mr-2 h-4 w-4" />
          <span className="hidden sm:inline">{t('pages.jobs.newJob')}</span>
          <span className="sm:hidden">{t('pages.jobs.newJobShort')}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-lg md:text-xl">
            {editingJobId ? t('pages.jobs.dialog.editTitle') : t('pages.jobs.dialog.createTitle')}
          </DialogTitle>
          <DialogDescription className="text-sm">
            {editingJobId
              ? t('pages.jobs.dialog.editDescription')
              : t('pages.jobs.dialog.createDescription')}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="action-type" className="cursor-pointer">
              {t('pages.jobs.form.actionType')}
            </Label>
            <Select value={actionType} onValueChange={onActionTypeChange}>
              <SelectTrigger id="action-type" className="cursor-pointer">
                <SelectValue placeholder={t('pages.jobs.form.actionTypePlaceholder')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="send_message" className="cursor-pointer">
                  {t('pages.jobs.actionTypes.sendMessage')}
                </SelectItem>
                <SelectItem value="send_poll" className="cursor-pointer">
                  {t('pages.jobs.actionTypes.sendPoll')}
                </SelectItem>
                <SelectItem value="send_ml_offer" className="cursor-pointer">
                  {t('pages.jobs.actionTypes.sendMlOffer')}
                </SelectItem>
                <SelectItem value="update_description" className="cursor-pointer">
                  {t('pages.jobs.actionTypes.updateDescription')}
                </SelectItem>
                <SelectItem value="close_groups" className="cursor-pointer">
                  {t('pages.jobs.actionTypes.closeGroups')}
                </SelectItem>
                <SelectItem value="open_groups" className="cursor-pointer">
                  {t('pages.jobs.actionTypes.openGroups')}
                </SelectItem>
                <SelectItem value="change_group_name" className="cursor-pointer">
                  {t('pages.jobs.actionTypes.changeGroupName')}
                </SelectItem>
                <SelectItem value="change_group_photo" className="cursor-pointer">
                  {t('pages.jobs.actionTypes.changeGroupPhoto')}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <GroupSelector
            groups={groups}
            selectedGroups={selectedGroups}
            searchTerm={searchTerm}
            loading={loadingGroups}
            onSearchChange={onSearchTermChange}
            onToggleGroup={onToggleGroup}
          />

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="date" className="cursor-pointer">
                {t('pages.jobs.form.date')}
              </Label>
              <Input
                id="date"
                type="date"
                value={scheduledDate}
                onChange={(e) => onScheduledDateChange(e.target.value)}
                className="cursor-pointer"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="time" className="cursor-pointer">
                {t('pages.jobs.form.time')}
              </Label>
              <Input
                id="time"
                type="time"
                value={scheduledTime}
                onChange={(e) => onScheduledTimeChange(e.target.value)}
                className="cursor-pointer"
              />
            </div>
          </div>

          {actionType === 'send_poll' && (
            <PollOptionsForm
              pollQuestion={pollQuestion}
              pollOptions={pollOptions}
              onQuestionChange={onPollQuestionChange}
              onAddOption={onAddPollOption}
              onRemoveOption={onRemovePollOption}
              onUpdateOption={onUpdatePollOption}
            />
          )}

          {actionType === 'send_message' && (
            <MessageForm
              payload={payload}
              mediaFile={mediaFile}
              mentionOpen={mentionOpen}
              isEnhancing={isEnhancing}
              onPayloadChange={onPayloadChange}
              onMediaFileChange={onMediaFileChange}
              onMentionOpenChange={onMentionOpenChange}
              onInsertMention={onInsertMention}
              onEnhanceMessage={onEnhanceMessage}
            />
          )}

          {actionType === 'send_ml_offer' && (
            <MlOfferForm
              mlProductUrl={mlProductUrl}
              mlIncludeImage={mlIncludeImage}
              payload={payload}
              onUrlChange={onMlProductUrlChange}
              onIncludeImageChange={onMlIncludeImageChange}
              onPayloadChange={onPayloadChange}
            />
          )}

          {actionType === 'update_description' && (
            <div className="space-y-2">
              <Label htmlFor="payload" className="cursor-pointer">
                {t('pages.jobs.form.newDescription')}
              </Label>
              <Textarea
                id="payload"
                placeholder={t('pages.jobs.form.newDescriptionPlaceholder')}
                value={payload}
                onChange={(e) => onPayloadChange(e.target.value)}
                rows={4}
                className="cursor-text"
              />
            </div>
          )}

          {actionType === 'change_group_name' && (
            <GroupNameForm
              payload={payload}
              autoNumberGroups={autoNumberGroups}
              onPayloadChange={onPayloadChange}
              onAutoNumberChange={onAutoNumberGroupsChange}
            />
          )}

          {actionType === 'change_group_photo' && (
            <div className="space-y-2">
              <Label htmlFor="group-photo" className="cursor-pointer">
                {t('pages.jobs.form.newPhoto')}
              </Label>
              <Input
                id="group-photo"
                type="file"
                accept="image/*"
                onChange={(e) => onMediaFileChange(e.target.files?.[0] || null)}
                className="cursor-pointer"
              />
              {mediaFile && (
                <p className="text-sm text-muted-foreground">
                  {t('pages.jobs.form.fileSelected')} {mediaFile.name}
                </p>
              )}
            </div>
          )}

          <Button onClick={onSubmit} className="w-full" disabled={isPending}>
            <Calendar className="mr-2 h-4 w-4" />
            {editingJobId
              ? isPending
                ? t('pages.jobs.form.saving')
                : t('pages.jobs.form.save')
              : isPending
                ? t('pages.jobs.form.submitting')
                : t('pages.jobs.form.submit')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
