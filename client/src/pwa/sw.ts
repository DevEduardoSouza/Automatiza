/// <reference lib="webworker" />
import {
	cleanupOutdatedCaches,
	createHandlerBoundToURL,
	precacheAndRoute,
} from "workbox-precaching";
import { NavigationRoute, registerRoute } from "workbox-routing";

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
// /r/<slug> é resolvido pelo BACKEND (302 direto pro WhatsApp, sem bundle).
// Sem esta denylist o SW serviria o app shell e o usuário controlado pelo SW
// nunca chegaria no Gin. /join continua no app shell — é uma página React.
// Atenção: um SW já instalado só passa a respeitar isto quando for substituído;
// até lá esses clientes seguem no fluxo SPA legado.
registerRoute(
	new NavigationRoute(createHandlerBoundToURL("/index.html"), {
		denylist: [/^\/api\//, /^\/connect\//, /^\/sentry-tunnel/, /^\/r(?:\/|$)/],
	}),
);

// Update com consentimento: o SW novo só assume quando a página manda SKIP_WAITING
// (disparado pelo botão "Recarregar" do toast). Sem skipWaiting automático.
self.addEventListener("message", (event) => {
	if (event.data?.type === "SKIP_WAITING") {
		self.skipWaiting();
	}
});

// ─── Web Push ──────────────────────────────────────────────────────────────
// push → showNotification (OBRIGATÓRIO: sem isso o browser mostra aviso genérico
// e pode revogar a subscription).
self.addEventListener("push", (event) => {
	if (!event.data) return;
	let payload: { title?: string; body?: string; url?: string } = {};
	try {
		payload = event.data.json();
	} catch {
		payload = { title: event.data.text() };
	}
	const title = payload.title || "Automatisa";
	event.waitUntil(
		self.registration.showNotification(title, {
			body: payload.body,
			icon: "/pwa-192.png",
			badge: "/pwa-192.png",
			data: { url: payload.url || "/" },
		}),
	);
});

// Clique na notificação → foca uma janela do app existente ou abre uma nova.
self.addEventListener("notificationclick", (event) => {
	event.notification.close();
	const url = (event.notification.data as { url?: string } | null)?.url || "/";
	event.waitUntil(
		(async () => {
			const wins = await self.clients.matchAll({
				type: "window",
				includeUncontrolled: true,
			});
			const target = new URL(url, self.location.origin).href;
			for (const w of wins) {
				if (new URL(w.url).origin === self.location.origin) {
					await w.focus();
					if ("navigate" in w)
						await (w as WindowClient).navigate(target).catch(() => {});
					return;
				}
			}
			await self.clients.openWindow(target);
		})(),
	);
});

// Chave da subscription rotacionou → re-inscreve com a MESMA applicationServerKey
// e reenvia ao backend (best-effort; falha silenciosa).
self.addEventListener("pushsubscriptionchange", (event) => {
	const e = event as ExtendableEvent & { oldSubscription?: PushSubscription };
	event.waitUntil(
		(async () => {
			try {
				const appServerKey =
					e.oldSubscription?.options?.applicationServerKey ?? undefined;
				const sub = await self.registration.pushManager.subscribe({
					userVisibleOnly: true,
					applicationServerKey: appServerKey ?? undefined,
				});
				const key = sub.getKey.bind(sub);
				const b64 = (name: "p256dh" | "auth") => {
					const k = key(name);
					return k ? btoa(String.fromCharCode(...new Uint8Array(k))) : "";
				};
				await fetch("/connect/webpush.v1.WebPushService/Subscribe", {
					method: "POST",
					headers: { "Content-Type": "application/json" },
					credentials: "include",
					body: JSON.stringify({
						endpoint: sub.endpoint,
						p256dh: b64("p256dh"),
						auth: b64("auth"),
						userAgent: self.navigator.userAgent,
					}),
				});
			} catch {
				/* best-effort */
			}
		})(),
	);
});
