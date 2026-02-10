import { z } from 'zod';

const distributionStrategyValues = ['member_limit', 'click_limit'] as const;

/**
 * Schema para criação/edição de link de redirecionamento.
 */
export const redirectLinkSchema = (t: (key: string) => string) =>
  z.object({
    name: z
      .string()
      .trim()
      .min(1, { message: t('pages.redirectLink.errors.nameRequiredMessage') })
      .max(100),
    slug: z
      .string()
      .trim()
      .min(1, { message: t('pages.redirectLink.errors.slugRequiredMessage') })
      .max(50)
      .regex(/^[a-z0-9-]+$/, {
        message: t('pages.redirectLink.errors.invalidSlugMessage'),
      }),
    distribution_strategy: z.enum(distributionStrategyValues).default('member_limit'),
    group_ids: z
      .array(z.string())
      .min(1, { message: t('pages.redirectLink.errors.groupsRequiredMessage') }),
  });

export type RedirectLinkFormData = z.infer<ReturnType<typeof redirectLinkSchema>>;

/**
 * Schema para prioridade de grupo no link.
 */
export const groupPrioritySchema = z.object({
  id: z.string(),
  priority: z.number().int().positive(),
  click_limit: z.number().int().positive().optional(),
});

export type GroupPriorityData = z.infer<typeof groupPrioritySchema>;
