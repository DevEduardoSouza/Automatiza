import { MetaAppForm } from '@/components/instagram/meta-app-form';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useInstagramConnect } from '@/hooks/use-instagram-connect';
import { useToast } from '@/hooks/use-toast';
import { Instagram } from 'lucide-react';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Página do Instagram, em três abas:
 *
 *  - Aplicativo: o app da Meta do PRÓPRIO cliente (o que dispensa a análise da
 *    Meta e permite usar hoje);
 *  - Conta: o OAuth da conta profissional — só faz sentido depois do app;
 *  - Histórico: os Stories publicados via {story} no grupo monitor.
 */
export default function InstagramPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { status, loading, connect, disconnect, disconnecting } = useInstagramConnect();

  // Feedback do redirect OAuth (?connected=1 | ?error=slug).
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get('connected') === '1') {
      toast({ title: t('pages.instagram.success.connected') });
    } else if (params.get('error')) {
      toast({ title: t('pages.instagram.errors.oauth'), variant: 'destructive' });
    }
    if (params.get('connected') || params.get('error')) {
      window.history.replaceState({}, '', window.location.pathname);
    }
  }, [t, toast]);

  return (
    <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto p-4 gap-4">
      <div className="flex items-center gap-3">
        <Instagram className="h-7 w-7 text-pink-600" />
        <div>
          <h1 className="text-2xl font-semibold">{t('pages.instagram.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('pages.instagram.subtitle')}</p>
        </div>
      </div>

      {loading ? (
        <Skeleton className="h-64 w-full max-w-2xl rounded-lg" />
      ) : !status?.featureEnabled ? (
        <Card className="max-w-2xl">
          <CardHeader>
            <CardTitle>{t('pages.instagram.comingSoon.title')}</CardTitle>
            <CardDescription>{t('pages.instagram.comingSoon.description')}</CardDescription>
          </CardHeader>
        </Card>
      ) : (
        // Sem app cadastrado, a aba Aplicativo é o primeiro passo — abrir na
        // aba Conta deixaria o cliente diante de um botão que não funciona.
        <Tabs defaultValue={status.hasMetaApp ? 'account' : 'app'} className="w-full">
          <TabsList>
            <TabsTrigger value="app">{t('pages.instagram.tabs.app')}</TabsTrigger>
            <TabsTrigger value="account">{t('pages.instagram.tabs.account')}</TabsTrigger>
            <TabsTrigger value="activity">{t('pages.instagram.tabs.activity')}</TabsTrigger>
          </TabsList>

          <TabsContent value="app" className="mt-4">
            <MetaAppForm />
          </TabsContent>

          <TabsContent value="account" className="mt-4">
            <Card className="max-w-2xl">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  {t('pages.instagram.connection.title')}
                  {status.connected ? (
                    <Badge variant="default">{t('pages.instagram.connection.connected')}</Badge>
                  ) : (
                    <Badge variant="secondary">
                      {t('pages.instagram.connection.disconnected')}
                    </Badge>
                  )}
                </CardTitle>
                <CardDescription>
                  {status.connected
                    ? t('pages.instagram.connection.connectedAs', {
                        username: status.username || '—',
                      })
                    : status.hasMetaApp
                      ? t('pages.instagram.connection.notConnected')
                      : // Sem app próprio, conectar não teria como funcionar —
                        // melhor dizer onde ir do que falhar depois do clique.
                        t('pages.instagram.app.requiredFirst')}
                </CardDescription>
              </CardHeader>
              <CardContent className="flex gap-3">
                {status.connected ? (
                  <Button
                    variant="destructive"
                    onClick={() => disconnect()}
                    disabled={disconnecting}
                  >
                    {t('pages.instagram.connection.disconnect')}
                  </Button>
                ) : (
                  <Button onClick={() => connect()} disabled={!status.hasMetaApp}>
                    {t('pages.instagram.connection.connect')}
                  </Button>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="activity" className="mt-4 flex flex-col gap-4">
            <Card className="max-w-2xl">
              <CardHeader>
                <CardTitle>{t('pages.instagram.howItWorks.title')}</CardTitle>
                <CardDescription>{t('pages.instagram.howItWorks.description')}</CardDescription>
              </CardHeader>
            </Card>

            {status.recentStories && status.recentStories.length > 0 && (
              <Card className="max-w-2xl">
                <CardHeader>
                  <CardTitle>{t('pages.instagram.recent.title')}</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-2">
                  {status.recentStories.map((s) => (
                    <div
                      key={s.id}
                      className="flex items-center justify-between text-sm border-b pb-2 last:border-0"
                    >
                      <span className="text-muted-foreground">{s.createdAt}</span>
                      <Badge
                        variant={
                          s.status === 'published'
                            ? 'default'
                            : s.status === 'failed'
                              ? 'destructive'
                              : 'secondary'
                        }
                      >
                        {t(`pages.instagram.recent.status.${s.status}`, { defaultValue: s.status })}
                      </Badge>
                    </div>
                  ))}
                </CardContent>
              </Card>
            )}
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}
