import * as Sentry from '@sentry/react';
import { createRoot } from 'react-dom/client';
import App from './app.tsx';
import './index.css';
import './i18n';
import { installDomSafetyNet } from './lib/dom-safety';
import { initSentry } from './lib/sentry';

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
