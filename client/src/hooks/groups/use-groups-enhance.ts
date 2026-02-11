import { useAiToolsConnect } from '@/hooks/use-ai-tools-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { UseGroupsEnhanceReturn } from './types';

/**
 * Hook for AI-powered message enhancement
 */
export function useGroupsEnhance(): UseGroupsEnhanceReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { enhanceMessage, isEnhancing: isEnhancingApi } = useAiToolsConnect();

  const [localEnhancing, setLocalEnhancing] = useState(false);
  const isEnhancing = localEnhancing || isEnhancingApi;

  const handleEnhanceMessage = useCallback(
    async (message: string, setMessage: (msg: string) => void) => {
      if (!message.trim()) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.emptyMessage'),
          description: t('pages.groups.errors.typeMessageToEnhance'),
        });
        return;
      }

      setLocalEnhancing(true);
      try {
        const messages = await enhanceMessage({
          productLinks: message,
        });

        if (messages.length > 0) {
          setMessage(messages[0]);
          toast({
            title: t('pages.groups.success.messageEnhanced'),
            description: t('pages.groups.success.messageEnhancedMessage'),
          });
        } else {
          toast({
            variant: 'destructive',
            title: t('pages.groups.errors.unexpectedResponse'),
            description: t('pages.groups.errors.unexpectedResponseMessage'),
          });
        }
      } catch (error: unknown) {
        captureHookError(error, 'useGroupsEnhance', 'handleEnhanceMessage');
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.enhanceMessage'),
          description: t('pages.groups.errors.tryAgain'),
        });
      } finally {
        setLocalEnhancing(false);
      }
    },
    [toast, t, enhanceMessage]
  );

  return {
    isEnhancing,
    handleEnhanceMessage,
  };
}
