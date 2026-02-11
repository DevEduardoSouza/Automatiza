import * as Sentry from '@sentry/react';
import { createRoot } from 'react-dom/client';
import App from './app.tsx';
import './index.css';
import './i18n';
import { initSentry } from './lib/sentry';

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
