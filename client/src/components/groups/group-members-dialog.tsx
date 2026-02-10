import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import type { GroupParticipant } from '@/hooks/use-groups-connect';
import { ChevronDown, ChevronUp, Loader2, LogOut, Search, Trash2, UserPlus } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Formata número de telefone com máscara.
 * Suporta números brasileiros e internacionais.
 */
function formatPhoneNumber(phone: string): string {
  // Remove tudo que não é dígito
  const digits = phone.replace(/\D/g, '');

  // Número brasileiro com código do país (55)
  if (digits.startsWith('55') && digits.length >= 12) {
    const countryCode = digits.slice(0, 2);
    const areaCode = digits.slice(2, 4);
    const remaining = digits.slice(4);

    if (remaining.length === 9) {
      // Celular: +55 (11) 99999-9999
      return `+${countryCode} (${areaCode}) ${remaining.slice(0, 5)}-${remaining.slice(5)}`;
    }
    if (remaining.length === 8) {
      // Fixo: +55 (11) 9999-9999
      return `+${countryCode} (${areaCode}) ${remaining.slice(0, 4)}-${remaining.slice(4)}`;
    }
    // Fallback para números com tamanho diferente
    return `+${countryCode} (${areaCode}) ${remaining}`;
  }

  // Número brasileiro sem código do país
  if (digits.length === 11) {
    // Celular: (11) 99999-9999
    return `(${digits.slice(0, 2)}) ${digits.slice(2, 7)}-${digits.slice(7)}`;
  }
  if (digits.length === 10) {
    // Fixo: (11) 9999-9999
    return `(${digits.slice(0, 2)}) ${digits.slice(2, 6)}-${digits.slice(6)}`;
  }

  // Número internacional genérico
  if (digits.length > 10) {
    // Tenta formatar como +XX (XX) XXXXX-XXXX
    const countryCode = digits.slice(0, 2);
    const rest = digits.slice(2);
    if (rest.length >= 10) {
      const areaCode = rest.slice(0, 2);
      const number = rest.slice(2);
      if (number.length === 9) {
        return `+${countryCode} (${areaCode}) ${number.slice(0, 5)}-${number.slice(5)}`;
      }
      if (number.length === 8) {
        return `+${countryCode} (${areaCode}) ${number.slice(0, 4)}-${number.slice(4)}`;
      }
      return `+${countryCode} (${areaCode}) ${number}`;
    }
    return `+${countryCode} ${rest}`;
  }

  // Retorna o número original se não conseguir formatar
  return phone;
}

interface GroupMembersDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  groupName: string;
  participants: GroupParticipant[];
  filteredParticipants: GroupParticipant[];
  loading: boolean;
  phoneNumberInput: string;
  searchQuery: string;
  onPhoneNumberChange: (value: string) => void;
  onSearchChange: (value: string) => void;
  onAddMember: () => Promise<void>;
  onRemoveMember: (participantId: string) => Promise<void>;
  onLeaveGroup: () => Promise<void>;
  onPromoteMember: (participantId: string) => Promise<void>;
  onDemoteMember: (participantId: string) => Promise<void>;
  isAddingMember: boolean;
  isRemovingMember: boolean;
  isLeavingGroup: boolean;
  isPromotingMember: boolean;
  isDemotingMember: boolean;
}

export function GroupMembersDialog({
  open,
  onOpenChange,
  groupName,
  participants,
  filteredParticipants,
  loading,
  phoneNumberInput,
  searchQuery,
  onPhoneNumberChange,
  onSearchChange,
  onAddMember,
  onRemoveMember,
  onLeaveGroup,
  onPromoteMember,
  onDemoteMember,
  isAddingMember,
  isRemovingMember,
  isLeavingGroup,
  isPromotingMember,
  isDemotingMember,
}: GroupMembersDialogProps) {
  const { t } = useTranslation();
  const [confirmAction, setConfirmAction] = useState<{
    type: 'remove' | 'leave' | 'promote' | 'demote';
    participantId?: string;
    participantName?: string;
  } | null>(null);

  const isProcessing = isRemovingMember || isPromotingMember || isDemotingMember;

  function getRoleBadge(participant: GroupParticipant) {
    if (participant.is_super_admin) {
      return <Badge variant="default">{t('pages.groups.members.superAdmin')}</Badge>;
    }
    if (participant.is_admin) {
      return (
        <Badge className="bg-[#8D2D53] text-white hover:bg-[#7a2747]">
          {t('pages.groups.members.admin')}
        </Badge>
      );
    }
    return <Badge variant="outline">{t('pages.groups.members.member')}</Badge>;
  }

  function getDisplayName(participant: GroupParticipant): string {
    const rawNumber = participant.phone_number || participant.id.replace(/@.*$/, '');
    return formatPhoneNumber(rawNumber);
  }

  async function handleConfirmedAction() {
    if (!confirmAction) return;
    switch (confirmAction.type) {
      case 'remove':
        if (confirmAction.participantId) {
          await onRemoveMember(confirmAction.participantId);
        }
        break;
      case 'leave':
        await onLeaveGroup();
        break;
      case 'promote':
        if (confirmAction.participantId) {
          await onPromoteMember(confirmAction.participantId);
        }
        break;
      case 'demote':
        if (confirmAction.participantId) {
          await onDemoteMember(confirmAction.participantId);
        }
        break;
    }
    setConfirmAction(null);
  }

  function getConfirmMessage(): string {
    if (!confirmAction) return '';
    const name = confirmAction.participantName ?? '';
    switch (confirmAction.type) {
      case 'remove':
        return t('pages.groups.members.removeMemberConfirm', { name });
      case 'leave':
        return t('pages.groups.members.leaveGroupConfirm', { name: groupName });
      case 'promote':
        return t('pages.groups.members.promoteConfirm', { name });
      case 'demote':
        return t('pages.groups.members.demoteConfirm', { name });
      default:
        return '';
    }
  }

  function getConfirmTitle(): string {
    if (!confirmAction) return '';
    switch (confirmAction.type) {
      case 'remove':
        return t('pages.groups.members.removeMember');
      case 'leave':
        return t('pages.groups.members.leaveGroup');
      case 'promote':
        return t('pages.groups.members.promoteToAdmin');
      case 'demote':
        return t('pages.groups.members.demoteFromAdmin');
      default:
        return '';
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-w-[calc(100vw-2rem)] xs:max-w-md sm:max-w-lg md:max-w-xl max-h-[85vh] sm:max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>{t('pages.groups.members.title')}</DialogTitle>
            <DialogDescription>
              {t('pages.groups.members.description', { name: groupName })}
            </DialogDescription>
          </DialogHeader>

          {/* Add member form */}
          <div className="flex gap-2 xs:gap-3">
            <Input
              placeholder={t('pages.groups.members.phonePlaceholder')}
              value={phoneNumberInput}
              onChange={(e) => onPhoneNumberChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && phoneNumberInput.trim()) {
                  onAddMember();
                }
              }}
              className="flex-1"
            />
            <Button
              onClick={onAddMember}
              disabled={!phoneNumberInput.trim() || isAddingMember}
              size="sm"
            >
              {isAddingMember ? (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              ) : (
                <UserPlus className="h-4 w-4" aria-hidden="true" />
              )}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">{t('pages.groups.members.phoneHint')}</p>

          <Separator />

          {/* Search participants */}
          <div className="relative">
            <Search
              className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              placeholder={t('pages.groups.members.searchParticipants')}
              value={searchQuery}
              onChange={(e) => onSearchChange(e.target.value)}
              className="pl-8"
            />
          </div>

          {/* Participant count */}
          <p className="text-sm text-muted-foreground">
            {t('pages.groups.members.participantCount', { count: participants.length })}
          </p>

          {/* Participants list */}
          <ScrollArea className="flex-1 min-h-0 max-h-[40vh] sm:max-h-[45vh] md:max-h-[50vh]">
            {loading ? (
              <div className="space-y-2 p-1">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={`skeleton-${i}`} className="h-12 w-full" />
                ))}
              </div>
            ) : filteredParticipants.length === 0 ? (
              <p className="text-center text-sm text-muted-foreground py-8">
                {t('pages.groups.members.noParticipants')}
              </p>
            ) : (
              <div className="space-y-1 p-1">
                {filteredParticipants.map((participant) => (
                  <div
                    key={participant.id}
                    className="flex items-center justify-between py-2 px-2 gap-2 xs:gap-3 xs:px-3 rounded-md hover:bg-muted/50"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-sm font-mono truncate">
                        {getDisplayName(participant)}
                      </span>
                      {getRoleBadge(participant)}
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      {!participant.is_super_admin && (
                        <>
                          {participant.is_admin ? (
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-7 w-7 sm:h-8 sm:w-8"
                              disabled={isProcessing}
                              onClick={() =>
                                setConfirmAction({
                                  type: 'demote',
                                  participantId: participant.id,
                                  participantName: getDisplayName(participant),
                                })
                              }
                              title={t('pages.groups.members.demoteFromAdmin')}
                            >
                              <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
                            </Button>
                          ) : (
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-7 w-7 sm:h-8 sm:w-8"
                              disabled={isProcessing}
                              onClick={() =>
                                setConfirmAction({
                                  type: 'promote',
                                  participantId: participant.id,
                                  participantName: getDisplayName(participant),
                                })
                              }
                              title={t('pages.groups.members.promoteToAdmin')}
                            >
                              <ChevronUp className="h-3.5 w-3.5" aria-hidden="true" />
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7 sm:h-8 sm:w-8 text-destructive hover:text-destructive"
                            disabled={isProcessing}
                            onClick={() =>
                              setConfirmAction({
                                type: 'remove',
                                participantId: participant.id,
                                participantName: getDisplayName(participant),
                              })
                            }
                            title={t('pages.groups.members.removeMember')}
                          >
                            <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </ScrollArea>

          <Separator />

          {/* Leave group button */}
          <Button
            variant="destructive"
            className="w-full"
            disabled={isLeavingGroup}
            onClick={() => setConfirmAction({ type: 'leave' })}
          >
            {isLeavingGroup ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
            ) : (
              <LogOut className="mr-2 h-4 w-4" aria-hidden="true" />
            )}
            {t('pages.groups.members.leaveGroup')}
          </Button>
        </DialogContent>
      </Dialog>

      {/* Confirmation dialog */}
      <AlertDialog open={!!confirmAction} onOpenChange={(open) => !open && setConfirmAction(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{getConfirmTitle()}</AlertDialogTitle>
            <AlertDialogDescription>{getConfirmMessage()}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirmedAction}>
              {t('common.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
