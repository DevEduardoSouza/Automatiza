import { z } from 'zod';

const storeValues = ['shopee', 'amazon', 'magalu', 'ml', 'shein', 'aliexpress', 'awin'] as const;

const discountTypeValues = ['fixed', 'percent'] as const;

/**
 * Schema para criação/edição de cupom.
 */
export const couponSchema = (t: (key: string) => string) =>
  z.object({
    code: z
      .string()
      .trim()
      .min(1, { message: t('pages.coupons.errors.codeRequired') })
      .max(50),
    title: z
      .string()
      .trim()
      .min(1, { message: t('pages.coupons.errors.titleRequired') })
      .max(255),
    store: z.enum(storeValues, {
      errorMap: () => ({ message: t('pages.coupons.errors.storeRequired') }),
    }),
    affiliate_link: z.string().url().nullish().or(z.literal('')),
    expires_at: z.string().nullish(),
    category: z.string().nullish(),
    discount_type: z.enum(discountTypeValues).nullish(),
    discount_value: z.number().positive().nullish(),
    max_discount: z.number().positive().nullish(),
    min_product_price: z.number().positive().nullish(),
    coupon_text: z.string().max(500).nullish(),
  });

export type CouponFormData = z.infer<ReturnType<typeof couponSchema>>;

/**
 * Schema para atualização parcial de cupom.
 */
export const updateCouponSchema = couponSchema;

export type UpdateCouponFormData = Partial<CouponFormData> & { is_active?: boolean };
