import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Controla os sinais vindos da captura do beforeinstallprompt.
const prompt = { current: null as unknown };
const installed = { current: false };
vi.mock('../install-prompt', () => ({
  getDeferredPrompt: () => prompt.current,
  wasInstalledThisSession: () => installed.current,
  subscribeInstallState: () => () => {},
  clearDeferredPrompt: () => {},
}));

import { getState } from '../use-pwa-install';

function setEnv(o: {
  matchStandalone?: boolean;
  navStandalone?: boolean;
  ios?: boolean;
  hasBIP?: boolean;
}) {
  window.matchMedia = ((_q: string) => ({
    matches: !!o.matchStandalone,
  })) as typeof window.matchMedia;
  Object.defineProperty(window.navigator, 'userAgent', {
    value: o.ios ? 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_4)' : 'Mozilla/5.0 (Windows) Chrome/120',
    configurable: true,
  });
  Object.defineProperty(window.navigator, 'platform', { value: 'Win32', configurable: true });
  Object.defineProperty(window.navigator, 'maxTouchPoints', { value: 0, configurable: true });
  (window.navigator as unknown as { standalone?: boolean }).standalone = !!o.navStandalone;
  if (o.hasBIP)
    (window as unknown as { onbeforeinstallprompt?: unknown }).onbeforeinstallprompt = null;
  else delete (window as unknown as { onbeforeinstallprompt?: unknown }).onbeforeinstallprompt;
}

describe('getState (derivação de instalabilidade)', () => {
  beforeEach(() => {
    prompt.current = null;
    installed.current = false;
  });
  afterEach(() => vi.restoreAllMocks());

  it('installed quando display-mode standalone', () => {
    setEnv({ matchStandalone: true });
    expect(getState()).toBe('installed');
  });

  it('installed quando appinstalled disparou nesta sessão', () => {
    setEnv({});
    installed.current = true;
    expect(getState()).toBe('installed');
  });

  it('native-prompt quando há beforeinstallprompt capturado', () => {
    setEnv({ hasBIP: true });
    prompt.current = { prompt: () => {} };
    expect(getState()).toBe('native-prompt');
  });

  it('ios-manual em iOS sem prompt nativo', () => {
    setEnv({ ios: true });
    expect(getState()).toBe('ios-manual');
  });

  it('chromium-manual quando suporta install mas ainda sem prompt', () => {
    setEnv({ hasBIP: true });
    expect(getState()).toBe('chromium-manual');
  });

  it('unsupported quando nada se aplica (ex.: Firefox desktop)', () => {
    setEnv({});
    expect(getState()).toBe('unsupported');
  });
});
