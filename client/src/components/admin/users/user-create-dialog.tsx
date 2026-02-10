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
import { Loader2, Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { CreateUserFormData } from './types';

interface UserCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  formData: CreateUserFormData;
  onFormDataChange: (data: CreateUserFormData) => void;
  onSubmit: () => void;
  creating: boolean;
}

/**
 * Dialog para criação de novo usuário.
 * Coleta email, senha, telefone e plano.
 */
export function UserCreateDialog({
  open,
  onOpenChange,
  formData,
  onFormDataChange,
  onSubmit,
  creating,
}: UserCreateDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button className="w-full md:w-auto">
          <Plus className="h-4 w-4 mr-2" />
          <span className="hidden sm:inline">{t('admin.users.addUser')}</span>
          <span className="sm:hidden">{t('admin.users.add')}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-md max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('admin.users.dialog.createTitle')}</DialogTitle>
          <DialogDescription>{t('admin.users.dialog.createDescription')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="email">{t('admin.users.dialog.emailRequired')}</Label>
            <Input
              id="email"
              type="email"
              placeholder={t('admin.dashboard.webhook.simulator.emailPlaceholder')}
              value={formData.email}
              onChange={(e) => onFormDataChange({ ...formData, email: e.target.value })}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="password">{t('admin.users.dialog.passwordRequired')}</Label>
            <Input
              id="password"
              type="password"
              placeholder={t('admin.users.dialog.passwordHint')}
              value={formData.password}
              onChange={(e) => onFormDataChange({ ...formData, password: e.target.value })}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="phone">{t('admin.users.dialog.phoneLabel')}</Label>
            <Input
              id="phone"
              type="tel"
              placeholder="+5511999999999"
              value={formData.phone_number}
              onChange={(e) =>
                onFormDataChange({
                  ...formData,
                  phone_number: e.target.value,
                })
              }
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="plan">{t('admin.users.dialog.planRequired')}</Label>
            <Select
              value={formData.plan}
              onValueChange={(value) => onFormDataChange({ ...formData, plan: value })}
            >
              <SelectTrigger id="plan">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="free">{t('common.planNames.free')}</SelectItem>
                <SelectItem value="pro">{t('common.planNames.pro')}</SelectItem>
                <SelectItem value="elite">{t('common.planNames.elite')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Button onClick={onSubmit} disabled={creating} className="w-full">
            {creating ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t('admin.users.dialog.creating')}
              </>
            ) : (
              <>
                <Plus className="h-4 w-4 mr-2" />
                {t('admin.users.dialog.createButton')}
              </>
            )}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
