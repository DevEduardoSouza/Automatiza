import * as Sentry from '@sentry/react';
import { createRoot } from 'react-dom/client';
import { toast } from 'sonner';
import App from './app.tsx';
import i18n from './i18n';
import './index.css';
import { installDomSafetyNet } from './lib/dom-safety';
import { initSentry } from './lib/sentry';
import './pwa/install-prompt';
import { registerPwa } from './pwa/register';

// Blinda removeChild/insertBefore contra DOM alterado por tradutores e
// extensões ANTES de qualquer render do React (facebook/react#11538).
installDomSafetyNet();

// FG1: Initialize Sentry BEFORE React root creation
initSentry();

// FSQ1: Root null check with Sentry capture
const rootElement = document.getElementById('root');
if (!rootElement) {
  const error = new Error('Root element #root not found in DOM');
  Sentry.captureException(error);
  throw error;
}

createRoot(rootElement).render(<App />);

// PWA: registra o service worker (no-op silencioso em navegador sem suporte).
// Quando um SW novo fica pronto, mostra um toast persistente com "Recarregar".
registerPwa((reload) => {
  toast(i18n.t('pwa.updateAvailable'), {
    duration: Number.POSITIVE_INFINITY,
    action: { label: i18n.t('pwa.reload'), onClick: reload },
  });
});
