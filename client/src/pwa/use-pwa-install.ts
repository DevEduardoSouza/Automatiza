import { useCallback, useSyncExternalStore } from 'react';
import {
  clearDeferredPrompt,
  getDeferredPrompt,
  subscribeInstallState,
  wasInstalledThisSession,
} from './install-prompt';

export type InstallState =
  | 'installed' // display-mode standalone || navigator.standalone (iOS)
  | 'native-prompt' // Chromium com beforeinstallprompt capturado
  | 'ios-manual' // iOS não instalado (sem beforeinstallprompt)
  | 'chromium-manual' // Chromium sem prompt ainda (engajamento) → menu do navegador
  | 'unsupported'; // Firefox desktop etc.

function isStandalone(): boolean {
  return (
    window.matchMedia?.('(display-mode: standalone)').matches ||
    // navigator.standalone é específico do iOS (não está no lib.dom)
    (window.navigator as unknown as { standalone?: boolean }).standalone === true
  );
}

// UA sniffing SÓ onde não há feature detection possível: iOS não emite
// beforeinstallprompt, então precisamos identificá-lo por UA/plataforma.
function isIOS(): boolean {
  const nav = window.navigator;
  return (
    /iPad|iPhone|iPod/.test(nav.userAgent) ||
    (nav.platform === 'MacIntel' && nav.maxTouchPoints > 1)
  );
}

export function getState(): InstallState {
  if (isStandalone() || wasInstalledThisSession()) return 'installed';
  if (getDeferredPrompt()) return 'native-prompt';
  if (isIOS()) return 'ios-manual';
  if ('onbeforeinstallprompt' in window) return 'chromium-manual';
  return 'unsupported';
}

/** Estado de instalabilidade + disparo do prompt nativo (Chromium). */
export function usePwaInstall(): {
  state: InstallState;
  promptInstall: () => Promise<'accepted' | 'dismissed' | 'unavailable'>;
} {
  const state = useSyncExternalStore(
    subscribeInstallState,
    getState,
    () => 'unsupported' as InstallState
  );

  const promptInstall = useCallback(async () => {
    const dp = getDeferredPrompt();
    if (!dp) return 'unavailable' as const;
    await dp.prompt();
    const choice = await dp.userChoice;
    clearDeferredPrompt();
    return choice.outcome;
  }, []);

  return { state, promptInstall };
}
