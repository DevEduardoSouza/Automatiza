// Captura o evento `beforeinstallprompt` no boot (ele pode disparar ANTES do
// React montar) e expõe o prompt diferido + um store para os componentes
// reagirem. Importado uma vez em main.tsx como side-effect.

export interface BeforeInstallPromptEvent extends Event {
  readonly platforms: string[];
  readonly userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>;
  prompt(): Promise<void>;
}

let deferredPrompt: BeforeInstallPromptEvent | null = null;
let installedThisSession = false;
const listeners = new Set<() => void>();
const emit = () => {
  for (const l of listeners) l();
};

if (typeof window !== 'undefined') {
  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault();
    deferredPrompt = e as BeforeInstallPromptEvent;
    emit();
  });
  window.addEventListener('appinstalled', () => {
    deferredPrompt = null;
    installedThisSession = true;
    emit();
  });
}

export const getDeferredPrompt = (): BeforeInstallPromptEvent | null => deferredPrompt;
export const clearDeferredPrompt = (): void => {
  deferredPrompt = null;
  emit();
};
export const wasInstalledThisSession = (): boolean => installedThisSession;
export const subscribeInstallState = (l: () => void): (() => void) => {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
};
