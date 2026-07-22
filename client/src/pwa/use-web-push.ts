import {
  GetConfigRequestSchema,
  SendTestRequestSchema,
  SubscribeRequestSchema,
  UnsubscribeRequestSchema,
  WebPushService,
} from '@/gen/webpush/v1/webpush_pb';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';

export type PushState =
  | 'unsupported' // navegador sem push
  | 'needs-install' // iOS não instalado (push só funciona no PWA instalado)
  | 'disabled-server' // servidor sem VAPID (feature off)
  | 'denied' // usuário negou a permissão (sticky)
  | 'subscribed' // já inscrito neste dispositivo
  | 'available'; // pode inscrever

const pushSupported =
  typeof navigator !== 'undefined' &&
  'serviceWorker' in navigator &&
  typeof window !== 'undefined' &&
  'PushManager' in window &&
  'Notification' in window;

function isStandalone(): boolean {
  return (
    window.matchMedia?.('(display-mode: standalone)').matches ||
    (window.navigator as unknown as { standalone?: boolean }).standalone === true
  );
}

function isIOS(): boolean {
  const nav = window.navigator;
  return (
    /iPad|iPhone|iPod/.test(nav.userAgent) ||
    (nav.platform === 'MacIntel' && nav.maxTouchPoints > 1)
  );
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  const out = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

function keyToB64(sub: PushSubscription, name: 'p256dh' | 'auth'): string {
  const k = sub.getKey(name);
  if (!k) return '';
  return btoa(String.fromCharCode(...new Uint8Array(k)));
}

/**
 * Web Push (PWA). Feature detection embutida; nunca pede permissão no load —
 * subscribe() SÓ deve ser chamado a partir de um gesto do usuário.
 */
export function useWebPush() {
  const transport = useTransport();
  const [subscribed, setSubscribed] = useState(false);
  const [busy, setBusy] = useState(false);

  const configQuery = useQuery({
    queryKey: ['webpush-config'],
    enabled: pushSupported,
    staleTime: 5 * 60_000,
    queryFn: async () => {
      const client = createClient(WebPushService, transport);
      return client.getConfig(create(GetConfigRequestSchema, {}));
    },
  });

  // Reflete se este dispositivo já tem uma subscription ativa.
  useEffect(() => {
    if (!pushSupported) return;
    let alive = true;
    navigator.serviceWorker.ready
      .then((reg) => reg.pushManager.getSubscription())
      .then((sub) => {
        if (alive) setSubscribed(!!sub);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  const state: PushState = !pushSupported
    ? 'unsupported'
    : isIOS() && !isStandalone()
      ? 'needs-install'
      : configQuery.data && !configQuery.data.enabled
        ? 'disabled-server'
        : Notification.permission === 'denied'
          ? 'denied'
          : subscribed
            ? 'subscribed'
            : 'available';

  const subscribe = useCallback(async (): Promise<boolean> => {
    if (busy) return false;
    setBusy(true);
    try {
      const client = createClient(WebPushService, transport);
      const cfg = await client.getConfig(create(GetConfigRequestSchema, {}));
      if (!cfg.enabled || !cfg.vapidPublicKey) return false;

      const permission = await Notification.requestPermission();
      if (permission !== 'granted') return false;

      const reg = await navigator.serviceWorker.ready;
      const sub =
        (await reg.pushManager.getSubscription()) ||
        (await reg.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(cfg.vapidPublicKey),
        }));

      await client.subscribe(
        create(SubscribeRequestSchema, {
          endpoint: sub.endpoint,
          p256dh: keyToB64(sub, 'p256dh'),
          auth: keyToB64(sub, 'auth'),
          userAgent: navigator.userAgent,
        })
      );
      setSubscribed(true);
      return true;
    } catch {
      return false;
    } finally {
      setBusy(false);
    }
  }, [busy, transport]);

  const unsubscribe = useCallback(async (): Promise<void> => {
    if (busy) return;
    setBusy(true);
    try {
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.getSubscription();
      if (sub) {
        const client = createClient(WebPushService, transport);
        await client
          .unsubscribe(create(UnsubscribeRequestSchema, { endpoint: sub.endpoint }))
          .catch(() => {});
        await sub.unsubscribe().catch(() => {});
      }
      setSubscribed(false);
    } finally {
      setBusy(false);
    }
  }, [busy, transport]);

  const sendTest = useCallback(async (): Promise<number> => {
    const client = createClient(WebPushService, transport);
    const res = await client.sendTest(create(SendTestRequestSchema, {}));
    return res.delivered;
  }, [transport]);

  return { state, subscribe, unsubscribe, sendTest, busy };
}
