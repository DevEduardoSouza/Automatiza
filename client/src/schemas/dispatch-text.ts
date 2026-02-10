import { z } from 'zod';

/**
 * Schema para criação/edição de texto de disparo.
 */
export const dispatchTextSchema = (t: (key: string) => string) =>
  z.object({
    name: z
      .string()
      .trim()
      .min(1, { message: t('pages.dispatchTexts.errors.nameRequired') })
      .max(100),
    body: z
      .string()
      .min(1, { message: t('pages.dispatchTexts.errors.bodyRequired') })
      .max(4000),
    is_active: z.boolean().default(true),
  });

export type DispatchTextFormData = z.infer<ReturnType<typeof dispatchTextSchema>>;
