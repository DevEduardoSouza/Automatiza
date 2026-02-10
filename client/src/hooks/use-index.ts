import { useTranslation } from 'react-i18next';

interface UseIndexReturn {
  title: string;
  subtitle: string;
}

/**
 * Hook for index/landing page logic.
 * Provides translated content for the welcome screen.
 */
export function useIndex(): UseIndexReturn {
  const { t } = useTranslation();

  return {
    title: t('pages.index.title'),
    subtitle: t('pages.index.subtitle'),
  };
}
