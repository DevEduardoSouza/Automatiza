import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import type { Automation } from '@/types/automation';
import { getMonitorRules } from '@/types/automation';
import { Plus, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const parseRulesTextarea = (value: string): string[] =>
  value
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);

interface MonitoringTextProcessingProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

export function MonitoringTextProcessing({ formData, setFormData }: MonitoringTextProcessingProps) {
  const { t } = useTranslation();
  const rules = getMonitorRules(formData.monitoring_text_rules);

  return (
    <div className="space-y-3">
      <MonitorRuleHeader />

      <MonitorRulePrefixSuffix formData={formData} setFormData={setFormData} rules={rules} />

      <MonitorRuleCleanup formData={formData} setFormData={setFormData} rules={rules} />

      <MonitorRuleSubstitutions formData={formData} setFormData={setFormData} rules={rules} />
    </div>
  );
}

function MonitorRuleHeader() {
  const { t } = useTranslation();

  return (
    <div className="space-y-1">
      <Label>{t('pages.offerAutomations.form.textProcessing')}</Label>
      <p className="text-xs text-muted-foreground">
        {t('pages.offerAutomations.form.textProcessingHint')}
      </p>
    </div>
  );
}

interface MonitorRulePrefixSuffixProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  rules: ReturnType<typeof getMonitorRules>;
}

function MonitorRulePrefixSuffix({ formData, setFormData, rules }: MonitorRulePrefixSuffixProps) {
  const { t } = useTranslation();

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 lg:gap-6">
      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.prefix')}</Label>
        <Textarea
          rows={2}
          value={rules.prefixo}
          onChange={(e) =>
            setFormData((prev) => ({
              ...prev,
              monitoring_text_rules: {
                ...getMonitorRules(prev.monitoring_text_rules),
                prefixo: e.target.value,
              },
            }))
          }
          placeholder={t('pages.offerAutomations.form.prefixPlaceholder')}
        />
      </div>
      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.suffix')}</Label>
        <Textarea
          rows={2}
          value={rules.sufixo}
          onChange={(e) =>
            setFormData((prev) => ({
              ...prev,
              monitoring_text_rules: {
                ...getMonitorRules(prev.monitoring_text_rules),
                sufixo: e.target.value,
              },
            }))
          }
          placeholder={t('pages.offerAutomations.form.suffixPlaceholder')}
        />
      </div>
    </div>
  );
}

interface MonitorRuleCleanupProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  rules: ReturnType<typeof getMonitorRules>;
}

function MonitorRuleCleanup({ formData, setFormData, rules }: MonitorRuleCleanupProps) {
  const { t } = useTranslation();

  return (
    <>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 lg:gap-6">
        <div className="flex items-center gap-2">
          <Checkbox
            checked={rules.limpar_arrobas}
            onCheckedChange={(checked) =>
              setFormData((prev) => ({
                ...prev,
                monitoring_text_rules: {
                  ...getMonitorRules(prev.monitoring_text_rules),
                  limpar_arrobas: Boolean(checked),
                },
              }))
            }
          />
          <Label className="cursor-pointer">
            {t('pages.offerAutomations.form.removeMentions')}
          </Label>
        </div>
        <div className="flex items-center gap-2">
          <Checkbox
            checked={rules.limpar_telefones}
            onCheckedChange={(checked) =>
              setFormData((prev) => ({
                ...prev,
                monitoring_text_rules: {
                  ...getMonitorRules(prev.monitoring_text_rules),
                  limpar_telefones: Boolean(checked),
                },
              }))
            }
          />
          <Label className="cursor-pointer">{t('pages.offerAutomations.form.removePhones')}</Label>
        </div>
      </div>

      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.removeTerms')}</Label>
        <Textarea
          rows={4}
          value={rules.remover_termos.join('\n')}
          onChange={(e) =>
            setFormData((prev) => ({
              ...prev,
              monitoring_text_rules: {
                ...getMonitorRules(prev.monitoring_text_rules),
                remover_termos: parseRulesTextarea(e.target.value),
              },
            }))
          }
          placeholder={t('pages.offerAutomations.form.removeTermsPlaceholder')}
        />
        <p className="text-xs text-muted-foreground">
          {t('pages.offerAutomations.form.removeTermsHint')}
        </p>
      </div>
    </>
  );
}

interface MonitorRuleSubstitutionsProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  rules: ReturnType<typeof getMonitorRules>;
}

function MonitorRuleSubstitutions({ formData, setFormData, rules }: MonitorRuleSubstitutionsProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>{t('pages.offerAutomations.form.substitutions')}</Label>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() =>
            setFormData((prev) => {
              const r = getMonitorRules(prev.monitoring_text_rules);
              return {
                ...prev,
                monitoring_text_rules: {
                  ...r,
                  substituicoes: [...r.substituicoes, { de: '', para: '' }],
                },
              };
            })
          }
        >
          <Plus className="h-3 w-3 mr-1" /> {t('common.add')}
        </Button>
      </div>

      {rules.substituicoes.map((s, idx) => (
        <div key={idx} className="grid grid-cols-1 md:grid-cols-5 gap-2 lg:gap-3">
          <Input
            className="md:col-span-2"
            placeholder={t('pages.offerAutomations.form.fromPlaceholder')}
            value={s.de}
            onChange={(e) =>
              setFormData((prev) => {
                const r = getMonitorRules(prev.monitoring_text_rules);
                const next = r.substituicoes.map((x, i) =>
                  i === idx ? { ...x, de: e.target.value } : x
                );
                return {
                  ...prev,
                  monitoring_text_rules: { ...r, substituicoes: next },
                };
              })
            }
          />
          <Input
            className="md:col-span-2"
            placeholder={t('pages.offerAutomations.form.toPlaceholder')}
            value={s.para}
            onChange={(e) =>
              setFormData((prev) => {
                const r = getMonitorRules(prev.monitoring_text_rules);
                const next = r.substituicoes.map((x, i) =>
                  i === idx ? { ...x, para: e.target.value } : x
                );
                return {
                  ...prev,
                  monitoring_text_rules: { ...r, substituicoes: next },
                };
              })
            }
          />
          <div className="flex items-center justify-end">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() =>
                setFormData((prev) => {
                  const r = getMonitorRules(prev.monitoring_text_rules);
                  return {
                    ...prev,
                    monitoring_text_rules: {
                      ...r,
                      substituicoes: r.substituicoes.filter((_, i) => i !== idx),
                    },
                  };
                })
              }
              title={t('common.remove')}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}
