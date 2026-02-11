import * as Sentry from '@sentry/react';
import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';

import ptBR from './locales/pt-br.json';

const resources = {
  'pt-BR': {
    translation: ptBR,
  },
};

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'pt-BR',
    lng: 'pt-BR',
    interpolation: {
      escapeValue: false,
    },
    // FR11: Report missing i18n keys to Sentry in production
    saveMissing: true,
    missingKeyHandler: (_lngs, _ns, key) => {
      if (import.meta.env.PROD) {
        Sentry.captureMessage(`Missing i18n key: ${key}`, 'warning');
      }
    },
  });

export default i18n;
