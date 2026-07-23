import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useInstagramAI } from '@/hooks/use-instagram-ai';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Aba "IA": o treinamento que o cliente escreve e os canais que ela atende.
 *
 * A IA responde SOZINHA, sem conferência humana — por isso a tela deixa
 * explícito o que está ligado, e o botão mestre só liga com persona escrita e
 * pelo menos um canal marcado (o servidor reforça a mesma regra).
 */
export function AIConfigForm() {
  const { t } = useTranslation();
  const { config, aiAvailable, dailyCap, loading, save, saving } = useInstagramAI();

  const [form, setForm] = useState({
    persona: '',
    businessContext: '',
    tone: '',
    blockedTopics: '',
    fallbackMessage: '',
    replyComments: false,
    replyDms: false,
    replyMentions: false,
    isActive: false,
  });

  useEffect(() => {
    if (config) {
      setForm({
        persona: config.persona,
        businessContext: config.businessContext,
        tone: config.tone,
        blockedTopics: config.blockedTopics,
        fallbackMessage: config.fallbackMessage,
        replyComments: config.replyComments,
        replyDms: config.replyDms,
        replyMentions: config.replyMentions,
        isActive: config.isActive,
      });
    }
  }, [config]);

  const anyChannel = form.replyComments || form.replyDms || form.replyMentions;
  const canActivate = form.persona.trim() !== '' && anyChannel;

  if (loading) return <Skeleton className="h-96 w-full max-w-2xl rounded-lg" />;

  return (
    <div className="flex flex-col gap-4 max-w-2xl">
      {!aiAvailable && (
        <Card className="border-amber-500/50">
          <CardHeader>
            <CardTitle className="text-base">{t('pages.instagram.ai.unavailable.title')}</CardTitle>
            <CardDescription>{t('pages.instagram.ai.unavailable.description')}</CardDescription>
          </CardHeader>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {t('pages.instagram.ai.title')}
            {form.isActive ? (
              <Badge variant="default">{t('pages.instagram.ai.on')}</Badge>
            ) : (
              <Badge variant="secondary">{t('pages.instagram.ai.off')}</Badge>
            )}
          </CardTitle>
          <CardDescription>{t('pages.instagram.ai.description')}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ig-persona">{t('pages.instagram.ai.personaLabel')}</Label>
            <Textarea
              id="ig-persona"
              rows={3}
              value={form.persona}
              placeholder={t('pages.instagram.ai.personaPlaceholder')}
              onChange={(e) => setForm({ ...form, persona: e.target.value })}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ig-business">{t('pages.instagram.ai.businessLabel')}</Label>
            <Textarea
              id="ig-business"
              rows={4}
              value={form.businessContext}
              placeholder={t('pages.instagram.ai.businessPlaceholder')}
              onChange={(e) => setForm({ ...form, businessContext: e.target.value })}
            />
            <p className="text-xs text-muted-foreground">{t('pages.instagram.ai.businessHelp')}</p>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ig-tone">{t('pages.instagram.ai.toneLabel')}</Label>
            <Input
              id="ig-tone"
              value={form.tone}
              placeholder={t('pages.instagram.ai.tonePlaceholder')}
              onChange={(e) => setForm({ ...form, tone: e.target.value })}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ig-blocked">{t('pages.instagram.ai.blockedLabel')}</Label>
            <Input
              id="ig-blocked"
              value={form.blockedTopics}
              placeholder={t('pages.instagram.ai.blockedPlaceholder')}
              onChange={(e) => setForm({ ...form, blockedTopics: e.target.value })}
            />
            <p className="text-xs text-muted-foreground">{t('pages.instagram.ai.blockedHelp')}</p>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ig-fallback">{t('pages.instagram.ai.fallbackLabel')}</Label>
            <Input
              id="ig-fallback"
              value={form.fallbackMessage}
              placeholder={t('pages.instagram.ai.fallbackPlaceholder')}
              onChange={(e) => setForm({ ...form, fallbackMessage: e.target.value })}
            />
            <p className="text-xs text-muted-foreground">{t('pages.instagram.ai.fallbackHelp')}</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('pages.instagram.ai.channels.title')}</CardTitle>
          <CardDescription>{t('pages.instagram.ai.channels.description')}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <Label htmlFor="ch-comments">{t('pages.instagram.ai.channels.comments')}</Label>
            <Switch
              id="ch-comments"
              checked={form.replyComments}
              onCheckedChange={(v) => setForm({ ...form, replyComments: v })}
            />
          </div>
          <div className="flex items-center justify-between">
            <div className="flex flex-col">
              <Label htmlFor="ch-dms">{t('pages.instagram.ai.channels.dms')}</Label>
              {/* A regra de 24h é da Meta, não nossa — o cliente precisa saber
                  por que uma mensagem antiga fica sem resposta. */}
              <span className="text-xs text-muted-foreground">
                {t('pages.instagram.ai.channels.dmsHelp')}
              </span>
            </div>
            <Switch
              id="ch-dms"
              checked={form.replyDms}
              onCheckedChange={(v) => setForm({ ...form, replyDms: v })}
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="ch-mentions">{t('pages.instagram.ai.channels.mentions')}</Label>
            <Switch
              id="ch-mentions"
              checked={form.replyMentions}
              onCheckedChange={(v) => setForm({ ...form, replyMentions: v })}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('pages.instagram.ai.master.title')}</CardTitle>
          <CardDescription>
            {t('pages.instagram.ai.master.description', { cap: dailyCap || 50 })}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <Label htmlFor="ai-active">{t('pages.instagram.ai.master.label')}</Label>
            <Switch
              id="ai-active"
              checked={form.isActive}
              disabled={!canActivate}
              onCheckedChange={(v) => setForm({ ...form, isActive: v })}
            />
          </div>
          {!canActivate && (
            <p className="text-xs text-amber-600">{t('pages.instagram.ai.master.requirements')}</p>
          )}
          <Button disabled={saving} onClick={() => save(form)} className="self-start">
            {t('pages.instagram.ai.save')}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
