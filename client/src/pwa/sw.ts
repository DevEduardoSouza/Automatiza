/// <reference lib="webworker" />
import {
  cleanupOutdatedCaches,
  createHandlerBoundToURL,
  precacheAndRoute,
} from 'workbox-precaching';
import { NavigationRoute, registerRoute } from 'workbox-routing';

// Contexto de service worker (webworker lib, não DOM). Este arquivo é compilado
// pelo vite-plugin-pwa (injectManifest) e checado por tsconfig.worker.json —
// está EXCLUÍDO do tsconfig.app.json de propósito.
declare let self: ServiceWorkerGlobalScope & {
  __WB_MANIFEST: Array<{ url: string; revision: string | null }>;
};

// Remove caches de versões antigas do Workbox ao ativar o SW novo.
cleanupOutdatedCaches();

// Precache de TODO o app shell (index.html + assets hasheados) com revisão.
// É o que garante que HTML e chunks sejam sempre da MESMA versão no cliente —
// imune ao index.html velho do edge Cloudflare pedindo hash que o deploy apagou.
precacheAndRoute(self.__WB_MANIFEST);

// Navegação (document requests) → app shell precacheado.
// API/RPC/Sentry ficam FORA do SW (sem handler = passthrough nativo do browser),
// então nada autenticado é cacheado e SSE/upload continuam idênticos ao atual.
registerRoute(
  new NavigationRoute(createHandlerBoundToURL('/index.html'), {
    denylist: [/^\/api\//, /^\/connect\//, /^\/sentry-tunnel/],
  })
);

// Update com consentimento: o SW novo só assume quando a página manda SKIP_WAITING
// (disparado pelo botão "Recarregar" do toast). Sem skipWaiting automático.
self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});
