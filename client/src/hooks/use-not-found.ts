import { useTranslation } from 'react-i18next';

interface UseNotFoundReturn {
  title: string;
  subtitle: string;
  backHomeLabel: string;
}

/**
 * Hook for NotFound page - encapsulates i18n translations
 * @returns Translated strings for the 404 page
 */
export function useNotFound(): UseNotFoundReturn {
  const { t } = useTranslation();

  return {
    title: t('pages.notFound.title'),
    subtitle: t('pages.notFound.subtitle'),
    backHomeLabel: t('pages.notFound.backHome'),
  };
}
