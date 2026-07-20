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
    // Missing keys are a build-time signal, not a production error: reporting
    // them to Sentry floods the monthly error quota (stale cached bundles
    // re-report keys that already exist). Warn in DEV only.
    saveMissing: true,
    missingKeyHandler: (_lngs, _ns, key) => {
      if (import.meta.env.DEV) {
        console.warn(`Missing i18n key: ${key}`);
      }
    },
  });

export default i18n;
