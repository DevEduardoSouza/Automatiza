import type { GroupParticipant } from '@/hooks/use-groups-connect';
import { useToast } from '@/hooks/use-toast';
import { Lock, Settings2, Unlock } from 'lucide-react';
import type { ChangeEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { GroupAudioDialog } from './group-audio-dialog';
import { GroupDescriptionDialog } from './group-description-dialog';
import { GroupMessageDialog } from './group-message-dialog';
import { GroupNameDialog } from './group-name-dialog';
import { GroupPhotoDialog } from './group-photo-dialog';
import { GroupPollDialog } from './group-poll-dialog';

// Wine/burgundy color palette based on #8D2D53
const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface GroupBulkActionsProps {
  selectedGroupsCount: number;
  onCloseGroups: () => void;
  onOpenGroups: () => void;
  changeNameDialogOpen: boolean;
  onChangeNameDialogOpenChange: (open: boolean) => void;
  groupName: string;
  onGroupNameChange: (value: string) => void;
  autoNumberGroups: boolean;
  onAutoNumberGroupsChange: (value: boolean) => void;
  onChangeNameSubmit: () => void;
  changePhotoDialogOpen: boolean;
  onChangePhotoDialogOpenChange: (open: boolean) => void;
  groupPhoto: File | null;
  onPhotoUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onChangePhotoSubmit: () => void;
  changeDescriptionDialogOpen: boolean;
  onChangeDescriptionDialogOpenChange: (open: boolean) => void;
  description: string;
  onDescriptionChange: (value: string) => void;
  onChangeDescriptionSubmit: () => void;
  sendMessageDialogOpen: boolean;
  onSendMessageDialogOpenChange: (open: boolean) => void;
  message: string;
  onMessageChange: (value: string) => void;
  mentionOpen: boolean;
  onMentionOpenChange: (value: boolean) => void;
  onInsertMention: (mention: string) => void;
  mentionParticipants: GroupParticipant[];
  loadingMentionParticipants: boolean;
  isEnhancing: boolean;
  onEnhanceMessage: () => void;
  mediaFile: File | null;
  onMediaUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onRemoveMedia: () => void;
  onSendMessageSubmit: () => void;
  sendAudioDialogOpen: boolean;
  onSendAudioDialogOpenChange: (open: boolean) => void;
  audioFile: File | null;
  onAudioUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onRemoveAudio: () => void;
  audioPtt: boolean;
  onAudioPttChange: (value: boolean) => void;
  audioCaption: string;
  onAudioCaptionChange: (value: string) => void;
  sendingAudio: boolean;
  onSendAudioSubmit: () => void;
  pollDialogOpen: boolean;
  onPollDialogOpenChange: (open: boolean) => void;
  pollQuestion: string;
  onPollQuestionChange: (value: string) => void;
  pollOptions: string[];
  onAddPollOption: () => void;
  onRemovePollOption: (index: number) => void;
  onUpdatePollOption: (index: number, value: string) => void;
  sendingPoll: boolean;
  onSendPollSubmit: () => void;
}

/**
 * Card com botões de ações em massa para grupos.
 */
export function GroupBulkActions({
  selectedGroupsCount,
  onCloseGroups,
  onOpenGroups,
  changeNameDialogOpen,
  onChangeNameDialogOpenChange,
  groupName,
  onGroupNameChange,
  autoNumberGroups,
  onAutoNumberGroupsChange,
  onChangeNameSubmit,
  changePhotoDialogOpen,
  onChangePhotoDialogOpenChange,
  groupPhoto,
  onPhotoUpload,
  onChangePhotoSubmit,
  changeDescriptionDialogOpen,
  onChangeDescriptionDialogOpenChange,
  description,
  onDescriptionChange,
  onChangeDescriptionSubmit,
  sendMessageDialogOpen,
  onSendMessageDialogOpenChange,
  message,
  onMessageChange,
  mentionOpen,
  onMentionOpenChange,
  onInsertMention,
  mentionParticipants,
  loadingMentionParticipants,
  isEnhancing,
  onEnhanceMessage,
  mediaFile,
  onMediaUpload,
  onRemoveMedia,
  onSendMessageSubmit,
  sendAudioDialogOpen,
  onSendAudioDialogOpenChange,
  audioFile,
  onAudioUpload,
  onRemoveAudio,
  audioPtt,
  onAudioPttChange,
  audioCaption,
  onAudioCaptionChange,
  sendingAudio,
  onSendAudioSubmit,
  pollDialogOpen,
  onPollDialogOpenChange,
  pollQuestion,
  onPollQuestionChange,
  pollOptions,
  onAddPollOption,
  onRemovePollOption,
  onUpdatePollOption,
  sendingPoll,
  onSendPollSubmit,
}: GroupBulkActionsProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  const showNoGroupsSelectedToast = () => {
    toast({
      variant: 'destructive',
      title: t('pages.groups.bulkActions.noGroupsSelected'),
      description: t('pages.groups.bulkActions.selectAtLeastOne'),
    });
  };

  const handleCloseGroups = () => {
    if (selectedGroupsCount === 0) {
      showNoGroupsSelectedToast();
      return;
    }
    onCloseGroups();
  };

  const handleOpenGroups = () => {
    if (selectedGroupsCount === 0) {
      showNoGroupsSelectedToast();
      return;
    }
    onOpenGroups();
  };

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8 xl:p-10"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      {/* Header */}
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4 lg:mb-5 xl:mb-6">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
          <Settings2 className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
        </div>
        <div>
          <span
            className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em]"
            style={{ color: colors.wineTextLight }}
          >
            {t('pages.groups.bulkActions.title')}
          </span>
          <p className="text-xs xs:text-sm" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
            {t('pages.groups.bulkActions.selectedCount', {
              count: selectedGroupsCount,
            })}
          </p>
        </div>
      </div>

      {/* Actions Grid */}
      <div className="grid grid-cols-1 xs:grid-cols-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-4 min-[1900px]:grid-cols-8 gap-2 xs:gap-2.5 md:gap-3 xl:gap-4">
        <button
          type="button"
          onClick={handleCloseGroups}
          className="inline-flex w-full min-w-0 cursor-pointer items-center justify-center gap-1.5 rounded-full border border-gray-300 bg-white px-3 py-1.5 text-xs xs:px-4 xs:py-2 xs:text-[13px] md:text-sm font-medium text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
        >
          <Lock className="shrink-0 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">
            {t('pages.groups.bulkActions.closeGroups')}
          </span>
          <span className="sm:hidden truncate">
            {t('pages.groups.bulkActions.closeGroupsShort')}
          </span>
        </button>
        <button
          type="button"
          onClick={handleOpenGroups}
          className="inline-flex w-full min-w-0 cursor-pointer items-center justify-center gap-1.5 rounded-full border border-gray-300 bg-white px-3 py-1.5 text-xs xs:px-4 xs:py-2 xs:text-[13px] md:text-sm font-medium text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
        >
          <Unlock className="shrink-0 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">
            {t('pages.groups.bulkActions.openGroups')}
          </span>
          <span className="sm:hidden truncate">
            {t('pages.groups.bulkActions.openGroupsShort')}
          </span>
        </button>

        <GroupNameDialog
          open={changeNameDialogOpen}
          onOpenChange={onChangeNameDialogOpenChange}
          groupName={groupName}
          onGroupNameChange={onGroupNameChange}
          autoNumber={autoNumberGroups}
          onAutoNumberChange={onAutoNumberGroupsChange}
          onSubmit={onChangeNameSubmit}
          selectedGroupsCount={selectedGroupsCount}
          onNoGroupsSelected={showNoGroupsSelectedToast}
        />

        <GroupPhotoDialog
          open={changePhotoDialogOpen}
          onOpenChange={onChangePhotoDialogOpenChange}
          photo={groupPhoto}
          onPhotoUpload={onPhotoUpload}
          onSubmit={onChangePhotoSubmit}
          selectedGroupsCount={selectedGroupsCount}
          onNoGroupsSelected={showNoGroupsSelectedToast}
        />

        <GroupPollDialog
          open={pollDialogOpen}
          onOpenChange={onPollDialogOpenChange}
          question={pollQuestion}
          onQuestionChange={onPollQuestionChange}
          options={pollOptions}
          onAddOption={onAddPollOption}
          onRemoveOption={onRemovePollOption}
          onUpdateOption={onUpdatePollOption}
          sendingPoll={sendingPoll}
          selectedGroupsCount={selectedGroupsCount}
          onSubmit={onSendPollSubmit}
          onNoGroupsSelected={showNoGroupsSelectedToast}
        />

        <GroupMessageDialog
          open={sendMessageDialogOpen}
          onOpenChange={onSendMessageDialogOpenChange}
          message={message}
          onMessageChange={onMessageChange}
          mentionOpen={mentionOpen}
          onMentionOpenChange={onMentionOpenChange}
          onInsertMention={onInsertMention}
          participants={mentionParticipants}
          loadingParticipants={loadingMentionParticipants}
          isEnhancing={isEnhancing}
          onEnhanceMessage={onEnhanceMessage}
          mediaFile={mediaFile}
          onMediaUpload={onMediaUpload}
          onRemoveMedia={onRemoveMedia}
          selectedGroupsCount={selectedGroupsCount}
          onSubmit={onSendMessageSubmit}
          onNoGroupsSelected={showNoGroupsSelectedToast}
        />

        <GroupAudioDialog
          open={sendAudioDialogOpen}
          onOpenChange={onSendAudioDialogOpenChange}
          audioFile={audioFile}
          onAudioUpload={onAudioUpload}
          onRemoveAudio={onRemoveAudio}
          audioPtt={audioPtt}
          onAudioPttChange={onAudioPttChange}
          audioCaption={audioCaption}
          onAudioCaptionChange={onAudioCaptionChange}
          sendingAudio={sendingAudio}
          selectedGroupsCount={selectedGroupsCount}
          onSubmit={onSendAudioSubmit}
          onNoGroupsSelected={showNoGroupsSelectedToast}
        />

        <GroupDescriptionDialog
          open={changeDescriptionDialogOpen}
          onOpenChange={onChangeDescriptionDialogOpenChange}
          description={description}
          onDescriptionChange={onDescriptionChange}
          onSubmit={onChangeDescriptionSubmit}
          selectedGroupsCount={selectedGroupsCount}
          onNoGroupsSelected={showNoGroupsSelectedToast}
        />
      </div>
    </div>
  );
}
