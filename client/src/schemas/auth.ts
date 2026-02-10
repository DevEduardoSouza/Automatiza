import { z } from 'zod';

/**
 * Schema para formulário de recuperação de senha.
 */
export const forgotPasswordSchema = (t: (key: string) => string) =>
  z.object({
    email: z
      .string()
      .trim()
      .min(1, { message: t('pages.forgotPassword.errors.emailRequired') })
      .email({ message: t('pages.forgotPassword.errors.invalidEmail') })
      .max(255),
  });

export type ForgotPasswordFormData = z.infer<ReturnType<typeof forgotPasswordSchema>>;

/**
 * Schema para formulário de redefinição de senha.
 */
export const resetPasswordSchema = (t: (key: string) => string) =>
  z
    .object({
      password: z
        .string()
        .min(6, { message: t('pages.resetPassword.errors.passwordTooShortMessage') })
        .max(72),
      confirmPassword: z.string().min(1),
    })
    .refine((data) => data.password === data.confirmPassword, {
      message: t('pages.resetPassword.errors.passwordMismatchMessage'),
      path: ['confirmPassword'],
    });

export type ResetPasswordFormData = z.infer<ReturnType<typeof resetPasswordSchema>>;

/**
 * Schema para formulário de login.
 */
export const loginSchema = (t: (key: string) => string) =>
  z.object({
    email: z
      .string()
      .trim()
      .min(1, { message: t('pages.auth.errors.emailRequired') })
      .email({ message: t('pages.auth.errors.invalidEmail') })
      .max(255),
    password: z
      .string()
      .min(1, { message: t('pages.auth.errors.passwordRequired') })
      .max(72),
  });

export type LoginFormData = z.infer<ReturnType<typeof loginSchema>>;

/**
 * Schema para formulário de magic link.
 */
export const magicLinkSchema = (t: (key: string) => string) =>
  z.object({
    email: z
      .string()
      .trim()
      .min(1, { message: t('pages.auth.magicLink.errors.emailRequired') })
      .email({ message: t('pages.auth.magicLink.errors.invalidEmail') })
      .max(255),
  });

export type MagicLinkFormData = z.infer<ReturnType<typeof magicLinkSchema>>;
