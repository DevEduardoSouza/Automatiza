// Registro do service worker — progressive enhancement puro.
// Qualquer falha aqui é silenciosa: sem SW, o site segue 100% idêntico.
//
// URL versionada (?v=BUILD_ID) + updateViaCache:'none': cada deploy muda a URL
// registrada, então o browser trata como script novo e o edge Cloudflare dá MISS
// (query única) — neutraliza o cache de 1h do CF sem acesso ao painel.

export type OnNeedRefresh = (reload: () => void) => void;

export function registerPwa(onNeedRefresh: OnNeedRefresh): void {
  if (!('serviceWorker' in navigator)) return;
  if (import.meta.env.DEV) return;

  window.addEventListener('load', () => {
    void navigator.serviceWorker
      .register(`/sw.js?v=${__BUILD_ID__}`, {
        scope: '/',
        updateViaCache: 'none',
      })
      .then((reg) => {
        // Best-effort: pede storage persistente (isento de eviction de 7 dias).
        void navigator.storage?.persist?.().catch(() => {});

        const promptIfWaiting = (worker: ServiceWorker | null) => {
          if (!worker) return;
          onNeedRefresh(() => worker.postMessage({ type: 'SKIP_WAITING' }));
        };

        // SW novo já aguardando (aba reaberta após um deploy anterior).
        if (reg.waiting && navigator.serviceWorker.controller) {
          promptIfWaiting(reg.waiting);
        }

        // SW novo detectado enquanto a aba está aberta.
        reg.addEventListener('updatefound', () => {
          const installing = reg.installing;
          installing?.addEventListener('statechange', () => {
            if (installing.state === 'installed' && navigator.serviceWorker.controller) {
              promptIfWaiting(installing);
            }
          });
        });

        // Quando o SW novo assume, recarrega uma única vez.
        let reloading = false;
        navigator.serviceWorker.addEventListener('controllerchange', () => {
          if (reloading) return;
          reloading = true;
          window.location.reload();
        });

        // Checagem de update: periódica (mitiga edge CF, ≤1h) e ao voltar o foco.
        setInterval(() => void reg.update().catch(() => {}), 60 * 60 * 1000);
        document.addEventListener('visibilitychange', () => {
          if (document.visibilityState === 'visible') void reg.update().catch(() => {});
        });
      })
      .catch(() => {
        // Falha ao registrar = sem SW. Site 100% funcional mesmo assim.
      });
  });
}
