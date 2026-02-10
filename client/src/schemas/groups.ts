import { z } from 'zod';

/**
 * Schema para alteração de nome de grupos em massa.
 */
export const changeGroupNameSchema = (t: (key: string) => string) =>
  z
    .object({
      name: z.string().max(100).optional(),
      autoNumber: z.boolean().default(false),
    })
    .refine((data) => data.autoNumber || (data.name && data.name.trim().length > 0), {
      message: t('pages.groups.errors.emptyNameMessage'),
      path: ['name'],
    });

export type ChangeGroupNameFormData = z.infer<ReturnType<typeof changeGroupNameSchema>>;

/**
 * Schema para alteração de descrição de grupos.
 */
export const changeGroupDescriptionSchema = (t: (key: string) => string) =>
  z.object({
    description: z
      .string()
      .min(1, { message: t('pages.groups.errors.emptyDescriptionMessage') })
      .max(2048),
  });

export type ChangeGroupDescriptionFormData = z.infer<
  ReturnType<typeof changeGroupDescriptionSchema>
>;

/**
 * Schema para envio de mensagem em massa.
 */
export const sendMessageSchema = (t: (key: string) => string) =>
  z.object({
    message: z
      .string()
      .min(1, { message: t('pages.groups.errors.emptyMessageDesc') })
      .max(4096),
  });

export type SendMessageFormData = z.infer<ReturnType<typeof sendMessageSchema>>;

/**
 * Schema para envio de enquete.
 */
export const sendPollSchema = (t: (key: string) => string) =>
  z.object({
    question: z
      .string()
      .min(1, { message: t('pages.groups.errors.questionRequiredMessage') })
      .max(500),
    options: z
      .array(z.string().min(1).max(100))
      .min(2, { message: t('pages.groups.errors.insufficientOptionsMessage') })
      .max(12, { message: t('pages.groups.errors.maxPollOptions') }),
  });

export type SendPollFormData = z.infer<ReturnType<typeof sendPollSchema>>;

/**
 * Schema para código de convite do grupo.
 */
export const inviteCodeSchema = (t: (key: string) => string) =>
  z.object({
    inviteCode: z
      .string()
      .trim()
      .min(1, { message: t('pages.groups.errors.emptyInviteCode') })
      .max(100),
  });

export type InviteCodeFormData = z.infer<ReturnType<typeof inviteCodeSchema>>;
