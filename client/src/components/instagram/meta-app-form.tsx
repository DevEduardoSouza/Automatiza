import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { useInstagramMetaApp } from '@/hooks/use-instagram-meta-app';
import { useToast } from '@/hooks/use-toast';
import { Check, Copy, ExternalLink } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Campo somente-leitura com botão de copiar.
 *
 * A URL do webhook e o token de verificação precisam ser colados EXATAMENTE no
 * painel da Meta — um caractere a mais e o canal não recebe nada, sem nenhum
 * erro visível para o cliente. Por isso copiar é um botão, não um "selecione e
 * copie".
 */
function CopyField({ label, value }: { label: string; value: string }) {
  const { toast } = useToast();
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard bloqueado (http, permissão): o valor segue visível e
      // selecionável no input, então o cliente não fica sem saída.
      toast({ title: t('pages.instagram.app.copyFailed'), variant: 'destructive' });
    }
  };

  return (
    <div className="flex flex-col gap-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <div className="flex gap-2">
        <Input
          readOnly
          value={value}
          className="font-mono text-xs"
          onFocus={(e) => e.target.select()}
        />
        <Button type="button" variant="outline" size="icon" onClick={copy} aria-label={label}>
          {copied ? <Check className="h-4 w-4 text-green-600" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
    </div>
  );
}

/**
 * Aba "Aplicativo": onde o cliente cadastra o app da Meta DELE.
 *
 * Este é o passo mais difícil do produto — criar um app no painel de
 * desenvolvedores da Meta é tarefa técnica para um afiliado. Em troca, é o que
 * dispensa a análise da Meta: app do próprio negócio usa Standard Access e não
 * passa por App Review. Daí o passo a passo numerado e os campos prontos para
 * copiar.
 */
export function MetaAppForm() {
  const { t } = useTranslation();
  const { app, loading, save, saving, remove, removing } = useInstagramMetaApp();

  const [appId, setAppId] = useState('');
  const [appSecret, setAppSecret] = useState('');
  const [touched, setTouched] = useState(false);

  // Enquanto o cliente não editar, os campos espelham o que está salvo.
  const currentAppId = touched ? appId : (app?.appId ?? '');
  const hasStoredSecret = Boolean(app?.hasSecret);
  // Segredo só é obrigatório no primeiro cadastro: depois, vazio = manter.
  const canSave = currentAppId.trim() !== '' && (hasStoredSecret || appSecret.trim() !== '');

  if (loading) {
    return <Skeleton className="h-64 w-full max-w-2xl rounded-lg" />;
  }

  return (
    <div className="flex flex-col gap-4 max-w-2xl">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {t('pages.instagram.app.title')}
            {app ? (
              <Badge variant="default">{t('pages.instagram.app.registered')}</Badge>
            ) : (
              <Badge variant="secondary">{t('pages.instagram.app.notRegistered')}</Badge>
            )}
          </CardTitle>
          <CardDescription>{t('pages.instagram.app.description')}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ig-app-id">{t('pages.instagram.app.appIdLabel')}</Label>
            <Input
              id="ig-app-id"
              value={currentAppId}
              placeholder={t('pages.instagram.app.appIdPlaceholder')}
              onChange={(e) => {
                setTouched(true);
                setAppId(e.target.value);
              }}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ig-app-secret">{t('pages.instagram.app.appSecretLabel')}</Label>
            <Input
              id="ig-app-secret"
              type="password"
              autoComplete="off"
              value={appSecret}
              placeholder={
                hasStoredSecret
                  ? t('pages.instagram.app.appSecretStored')
                  : t('pages.instagram.app.appSecretPlaceholder')
              }
              onChange={(e) => setAppSecret(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              {t('pages.instagram.app.appSecretHelp')}
            </p>
          </div>

          <div className="flex gap-3">
            <Button
              disabled={!canSave || saving}
              onClick={() => {
                save({ appId: currentAppId.trim(), appSecret: appSecret.trim() });
                setAppSecret('');
              }}
            >
              {t('pages.instagram.app.save')}
            </Button>
            {app && (
              <Button variant="destructive" disabled={removing} onClick={() => remove()}>
                {t('pages.instagram.app.remove')}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Os valores para colar de volta no painel da Meta só existem depois que
          o app foi salvo — antes disso não há token de webhook gerado. */}
      {app && (
        <Card>
          <CardHeader>
            <CardTitle>{t('pages.instagram.app.pasteBack.title')}</CardTitle>
            <CardDescription>{t('pages.instagram.app.pasteBack.description')}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <CopyField label={t('pages.instagram.app.redirectUriLabel')} value={app.redirectUri} />
            <CopyField label={t('pages.instagram.app.webhookUrlLabel')} value={app.webhookUrl} />
            <CopyField label={t('pages.instagram.app.verifyTokenLabel')} value={app.verifyToken} />
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>{t('pages.instagram.app.steps.title')}</CardTitle>
          <CardDescription>{t('pages.instagram.app.steps.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <ol className="flex flex-col gap-2 text-sm list-decimal pl-5">
            <li>{t('pages.instagram.app.steps.s1')}</li>
            <li>{t('pages.instagram.app.steps.s2')}</li>
            <li>{t('pages.instagram.app.steps.s3')}</li>
            <li>{t('pages.instagram.app.steps.s4')}</li>
            <li>{t('pages.instagram.app.steps.s5')}</li>
          </ol>
          <Button variant="link" className="px-0 mt-2" asChild>
            <a
              href="https://developers.facebook.com/apps/"
              target="_blank"
              rel="noopener noreferrer"
            >
              {t('pages.instagram.app.steps.openMeta')}
              <ExternalLink className="h-3.5 w-3.5 ml-1" />
            </a>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
