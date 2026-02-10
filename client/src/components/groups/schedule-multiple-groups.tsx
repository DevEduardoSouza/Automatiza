import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
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
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import {
  type CreateJobParams,
  type LocalJobActionType,
  useJobsConnect,
} from '@/hooks/use-jobs-connect';
import { useToast } from '@/hooks/use-toast';
import { AlertCircle, Plus, Search, Star, Upload, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Componente para agendar acoes em multiplos grupos.
 * Permite configurar nome, descricao, status, foto e mensagem para data/hora futura.
 */
export function ScheduleMultipleGroups() {
  const { t } = useTranslation();
  const { groups: availableGroups, loading: loadingGroups } = useGroupsConnect();
  const { createBulkJobs, isCreatingJob } = useJobsConnect();
  const { toast } = useToast();

  const [scheduledDate, setScheduledDate] = useState('');
  const [scheduledTime, setScheduledTime] = useState('');
  const [groupName, setGroupName] = useState('');
  const [description, setDescription] = useState('');
  const [message, setMessage] = useState('');
  const [status, setStatus] = useState<'open' | 'closed' | 'none'>('none');
  const [groupPhoto, setGroupPhoto] = useState<File | null>(null);
  const [selectedGroups, setSelectedGroups] = useState<string[]>([]);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');

  // Use all available groups (backend query already handles admin fallback)
  const adminGroups = availableGroups;

  const toggleGroup = (groupId: string) => {
    setSelectedGroups((prev) =>
      prev.includes(groupId) ? prev.filter((id) => id !== groupId) : [...prev, groupId]
    );
  };

  const handlePhotoUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setGroupPhoto(file);
      toast({
        title: t('pages.groups.schedule.photoSelected'),
        description: file.name,
      });
    }
  };

  const handleSchedule = async () => {
    if (!scheduledDate || !scheduledTime) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.schedule.errors.requiredFields'),
        description: t('pages.groups.schedule.errors.fillDateAndTime'),
      });
      return;
    }

    if (selectedGroups.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.schedule.errors.noGroupSelected'),
        description: t('pages.groups.schedule.errors.selectAtLeastOne'),
      });
      return;
    }

    if (
      !groupName.trim() &&
      !description.trim() &&
      !message.trim() &&
      status === 'none' &&
      !groupPhoto
    ) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.schedule.errors.noActionConfigured'),
        description: t('pages.groups.schedule.errors.configureAtLeastOneAction'),
      });
      return;
    }

    try {
      let photoBase64: string | null = null;
      if (groupPhoto) {
        const reader = new FileReader();
        const dataUrl = await new Promise<string>((resolve, reject) => {
          reader.onload = () => resolve(reader.result as string);
          reader.onerror = reject;
          reader.readAsDataURL(groupPhoto);
        });
        photoBase64 = dataUrl.split(',')[1] || dataUrl;
      }

      const localDateTime = new Date(`${scheduledDate}T${scheduledTime}:00`);

      const jobsToCreate: CreateJobParams[] = [];
      const selectedGroupsData = adminGroups.filter((g) => selectedGroups.includes(g.id));

      for (let i = 0; i < selectedGroupsData.length; i++) {
        const group = selectedGroupsData[i];
        const baseTime = localDateTime.getTime() + i * 8000;
        let actionDelay = 0;

        if (groupName.trim()) {
          const numberedGroupName = `#${i + 1} ${groupName}`;
          jobsToCreate.push({
            action_type: 'change_group_name' as LocalJobActionType,
            scheduled_for: new Date(baseTime + actionDelay),
            payload: {
              groups: [group.id],
              name: numberedGroupName,
              autoNumber: false,
            },
          });
          actionDelay += 2000;
        }

        if (photoBase64) {
          jobsToCreate.push({
            action_type: 'change_group_photo' as LocalJobActionType,
            scheduled_for: new Date(baseTime + actionDelay),
            payload: {
              groups: [group.id],
              image: photoBase64,
            },
          });
          actionDelay += 2000;
        }

        if (description.trim()) {
          jobsToCreate.push({
            action_type: 'update_description' as LocalJobActionType,
            scheduled_for: new Date(baseTime + actionDelay),
            payload: {
              groups: [group.id],
              description: description,
            },
          });
          actionDelay += 2000;
        }

        if (status !== 'none') {
          jobsToCreate.push({
            action_type: (status === 'closed'
              ? 'close_groups'
              : 'open_groups') as LocalJobActionType,
            scheduled_for: new Date(baseTime + actionDelay),
            payload: {
              groups: [group.id],
            },
          });
          actionDelay += 2000;
        }

        if (message.trim()) {
          jobsToCreate.push({
            action_type: 'send_message' as LocalJobActionType,
            scheduled_for: new Date(baseTime + actionDelay),
            payload: {
              groups: [group.id],
              message: message,
            },
          });
        }
      }

      await createBulkJobs(jobsToCreate);

      toast({
        title: t('pages.groups.schedule.success.scheduled'),
        description: t('pages.groups.schedule.success.scheduledDescription', {
          actions: jobsToCreate.length,
          groups: selectedGroups.length,
        }),
      });

      setSelectedGroups([]);
      setGroupName('');
      setDescription('');
      setMessage('');
      setStatus('none');
      setGroupPhoto(null);
      setScheduledDate('');
      setScheduledTime('');
      setSearchTerm('');
      setDialogOpen(false);
    } catch (error) {
      const err = error as { message?: string };
      toast({
        variant: 'destructive',
        title: t('pages.groups.schedule.errors.scheduleError'),
        description: err.message || t('common.tryAgain'),
      });
    }
  };

  return (
    <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
      <DialogTrigger asChild>
        <Button className="flex-1 md:flex-none bg-[#8D2D53] text-white hover:bg-[#7a2747]">
          <Plus className="mr-2 h-4 w-4" />
          <span className="hidden sm:inline">{t('pages.groups.schedule.scheduleButton')}</span>
          <span className="sm:hidden">{t('pages.groups.schedule.scheduleButtonShort')}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-lg md:text-xl">
            {t('pages.groups.schedule.title')}
          </DialogTitle>
          <DialogDescription className="text-sm">
            {t('pages.groups.schedule.description')}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <Alert>
            <AlertCircle className="h-4 w-4 shrink-0" />
            <AlertDescription className="text-xs md:text-sm">
              <strong>{t('pages.groups.schedule.howItWorks.title')}</strong>{' '}
              {t('pages.groups.schedule.howItWorks.description')}
            </AlertDescription>
          </Alert>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="date" className="cursor-pointer">
                {t('pages.groups.schedule.date')}
              </Label>
              <Input
                id="date"
                type="date"
                value={scheduledDate}
                onChange={(e) => setScheduledDate(e.target.value)}
                className="cursor-pointer"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="time" className="cursor-pointer">
                {t('pages.groups.schedule.time')}
              </Label>
              <Input
                id="time"
                type="time"
                value={scheduledTime}
                onChange={(e) => setScheduledTime(e.target.value)}
                className="cursor-pointer"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label className="cursor-default">{t('pages.groups.schedule.selectGroups')}</Label>
            <div className="relative mb-2">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t('pages.groups.schedule.searchPlaceholder')}
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-9 cursor-text"
              />
            </div>
            <div className="border rounded-lg p-4 space-y-2 max-h-48 overflow-y-auto">
              {loadingGroups ? (
                <p className="text-sm text-muted-foreground text-center py-4">
                  {t('common.loading')}
                </p>
              ) : adminGroups.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-4">
                  {t('pages.groups.schedule.noGroupsAvailable')}
                </p>
              ) : (
                adminGroups
                  .filter((group) => group.name.toLowerCase().includes(searchTerm.toLowerCase()))
                  .map((group) => (
                    <div
                      key={group.id}
                      className="flex items-center space-x-2 cursor-pointer"
                      onClick={() => toggleGroup(group.id)}
                    >
                      <Checkbox
                        id={`group-${group.id}`}
                        checked={selectedGroups.includes(group.id)}
                        onCheckedChange={() => toggleGroup(group.id)}
                        className="cursor-pointer"
                      />
                      <Label
                        htmlFor={`group-${group.id}`}
                        className="text-sm font-normal cursor-pointer flex-1 flex items-center gap-1.5"
                      >
                        {group.is_favorite && (
                          <Star className="h-3.5 w-3.5 fill-[#8D2D53] text-[#8D2D53]" />
                        )}
                        {group.name}
                      </Label>
                    </div>
                  ))
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {t('pages.groups.schedule.selectedCount', { count: selectedGroups.length })}
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="groupName" className="cursor-pointer">
              {t('pages.groups.schedule.baseNameOptional')}
            </Label>
            <Input
              id="groupName"
              placeholder={t('pages.groups.schedule.baseNamePlaceholder')}
              value={groupName}
              onChange={(e) => setGroupName(e.target.value)}
              className="cursor-text"
            />
            {groupName.trim() && (
              <p className="text-xs text-muted-foreground">
                {t('pages.groups.schedule.renameHint', { name: groupName })}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="description" className="cursor-pointer">
              {t('pages.groups.schedule.descriptionOptional')}
            </Label>
            <Textarea
              id="description"
              placeholder={t('pages.groups.schedule.descriptionPlaceholder')}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              className="cursor-text"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="status" className="cursor-pointer">
              {t('pages.groups.schedule.statusOptional')}
            </Label>
            <Select
              value={status}
              onValueChange={(value: 'open' | 'closed' | 'none') => setStatus(value)}
            >
              <SelectTrigger className="cursor-pointer">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none" className="cursor-pointer">
                  {t('pages.groups.schedule.statusNoChange')}
                </SelectItem>
                <SelectItem value="open" className="cursor-pointer">
                  {t('pages.groups.schedule.statusOpen')}
                </SelectItem>
                <SelectItem value="closed" className="cursor-pointer">
                  {t('pages.groups.schedule.statusClosed')}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="groupPhoto" className="cursor-pointer">
              {t('pages.groups.schedule.photoOptional')}
            </Label>
            <div className="flex gap-2">
              <Input
                id="groupPhoto"
                type="file"
                accept="image/*"
                onChange={handlePhotoUpload}
                className="flex-1 cursor-pointer"
              />
              {groupPhoto && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="cursor-pointer"
                  onClick={() => setGroupPhoto(null)}
                >
                  <X className="h-4 w-4" />
                </Button>
              )}
            </div>
            {groupPhoto && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Upload className="h-4 w-4" />
                <span>{groupPhoto.name}</span>
              </div>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="message" className="cursor-pointer">
              {t('pages.groups.schedule.messageOptional')}
            </Label>
            <Textarea
              id="message"
              placeholder={t('pages.groups.schedule.messagePlaceholder')}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              rows={4}
              className="cursor-text"
            />
            {message.includes('@todos') && (
              <Alert className="bg-blue-50 border-blue-200">
                <AlertCircle className="h-4 w-4 text-blue-600" />
                <AlertDescription className="text-blue-800">
                  <strong>{t('pages.groups.schedule.mentionDetected.title')}</strong>{' '}
                  {t('pages.groups.schedule.mentionDetected.description')}
                </AlertDescription>
              </Alert>
            )}
          </div>

          <Button
            onClick={handleSchedule}
            className="w-full"
            disabled={selectedGroups.length === 0 || isCreatingJob}
          >
            {isCreatingJob
              ? t('common.loading')
              : t('pages.groups.schedule.scheduleCount', { count: selectedGroups.length || 0 })}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
