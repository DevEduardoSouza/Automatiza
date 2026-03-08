import { useAuth } from '@/contexts';
import {
  type LocalAffiliateCredential,
  type LocalAffiliateStore,
  useAffiliateCredentialsConnect,
} from '@/hooks/use-affiliate-credentials-connect';
import { useMercadoLivreConnect } from '@/hooks/use-mercadolivre-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

type StoreKey = LocalAffiliateStore;

interface AffiliateCredential {
  id?: string;
  store: StoreKey;
  credentials: Record<string, string>;
  auto_generate: boolean;
  is_active: boolean;
  selected_brands?: string[];
}

interface StoreField {
  key: string;
  label: string;
  type: string;
  placeholder: string;
}

interface StoreConfig {
  key: StoreKey;
  name: string;
  fields: StoreField[];
}

interface MlLastTest {
  originUrl: string;
  shortUrl?: string | null;
  longUrl?: string | null;
  title?: string | null;
  price?: number | null;
  currency?: string | null;
  thumbnail?: string | null;
  condition?: string | null;
  soldQuantity?: number | null;
  ratingAverage?: number | null;
  reviewsTotal?: number | null;
  originalPrice?: number | null;
  discountPercent?: number | null;
  installmentsText?: string | null;
}

type MlConnectionToken = { tag: string; cookie: string; csrfToken?: string } | { error: string };

const AWIN_BRANDS = [
  'Beleza na Web',
  'C&A',
  'Dafiti',
  'Eudora',
  'Natura',
  'O Boticario',
  'Amend',
  'Amaro',
  'Netshoes',
  'Zattini',
  'Centauro',
  'Shop2gether',
  'Tricae',
];

const STORE_CONFIGS: StoreConfig[] = [
  {
    key: 'shopee',
    name: 'Shopee',
    fields: [
      { key: 'appId', label: 'App ID', type: 'text', placeholder: '12345' },
      {
        key: 'password',
        label: 'Senha',
        type: 'password',
        placeholder: '********',
      },
    ],
  },
  {
    key: 'amazon',
    name: 'Amazon Associates',
    fields: [
      { key: 'tag', label: 'Tag/ID', type: 'text', placeholder: 'seusite-20' },
      {
        key: 'accessKey',
        label: 'Access Key',
        type: 'text',
        placeholder: 'AKIAIOSFODNN7EXAMPLE',
      },
      {
        key: 'secretKey',
        label: 'Secret Key',
        type: 'password',
        placeholder: 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY',
      },
    ],
  },
  {
    key: 'magalu',
    name: 'Magazine Luiza',
    fields: [
      {
        key: 'partnerStore',
        label: 'Loja Parceiro',
        type: 'text',
        placeholder: 'magazinevoce.com.br/sualoja',
      },
    ],
  },
  {
    key: 'ml',
    name: 'Mercado Livre',
    fields: [
      {
        key: 'tag',
        label: 'Tag (etiqueta)',
        type: 'text',
        placeholder: 'soed324928',
      },
    ],
  },
  {
    key: 'shein',
    name: 'Shein',
    fields: [
      {
        key: 'affiliateLink',
        label: 'Link de Afiliado',
        type: 'text',
        placeholder: 'https://onelink.shein.com/...',
      },
    ],
  },
  {
    key: 'aliexpress',
    name: 'AliExpress',
    fields: [
      {
        key: 'trackingId',
        label: 'Tracking ID',
        type: 'text',
        placeholder: 'default',
      },
      { key: 'appKey', label: 'App Key', type: 'text', placeholder: '12345' },
    ],
  },
  {
    key: 'awin',
    name: 'Awin',
    fields: [
      {
        key: 'awinId',
        label: 'Awin ID',
        type: 'text',
        placeholder: 'Exemplo: 123457',
      },
      {
        key: 'oauth2Token',
        label: 'Awin OAuth2 Token',
        type: 'password',
        placeholder: 'Exemplo: a12b3c4-d5e6-f890-123a-b4c567890f12',
      },
    ],
  },
];

const ENABLED_STORES: StoreKey[] = ['shopee', 'ml'];

function base64UrlDecode(input: string): string {
  const s = input.replace(/-/g, '+').replace(/_/g, '/');
  const pad = s.length % 4 === 0 ? '' : '='.repeat(4 - (s.length % 4));
  try {
    return atob(s + pad);
  } catch (error: unknown) {
    captureHookError(error, 'useAffiliatePrograms', 'base64UrlDecode');
    throw error;
  }
}

function decodeMlConnectionToken(rawToken: string): MlConnectionToken {
  const token = String(rawToken || '').trim();
  if (!token) return { error: 'Token vazio.' };

  const prefix = 'MLCRED_V1:';
  const payload = token.startsWith(prefix) ? token.slice(prefix.length) : token;

  try {
    const json = base64UrlDecode(payload);
    const obj = JSON.parse(json || '{}');
    const tag = String(obj?.tag || '').trim();
    const cookie = String(obj?.cookie || '').trim();
    const csrfToken = obj?.csrfToken ? String(obj.csrfToken).trim() : undefined;
    if (!tag || !cookie) {
      return { error: 'Token invalido (faltando tag/cookie).' };
    }
    return { tag, cookie, csrfToken };
  } catch (error: unknown) {
    captureHookError(error, 'useAffiliatePrograms', 'decodeMlConnectionToken');
    return { error: 'Token invalido (nao consegui decodificar).' };
  }
}

interface UseAffiliateProgramsReturn {
  credentials: Record<StoreKey, AffiliateCredential>;
  loading: boolean;
  saving: StoreKey | null;
  testing: StoreKey | null;
  mlLastTest: MlLastTest | null;
  storeConfigs: StoreConfig[];
  enabledStores: StoreKey[];
  awinBrands: string[];
  updateCredentialField: (storeKey: StoreKey, field: string, value: string) => void;
  toggleAutoGenerate: (storeKey: StoreKey) => void;
  toggleBrand: (storeKey: StoreKey, brand: string) => void;
  handleSave: (storeKey: StoreKey) => Promise<void>;
  handleTest: (storeKey: StoreKey) => Promise<void>;
  decodeMlToken: (rawToken: string) => MlConnectionToken;
  formatSoldQuantity: (q: number | null | undefined) => string | null;
  getStoreDescription: (storeKey: StoreKey) => string;
}

export type {
  StoreKey,
  StoreConfig,
  StoreField,
  AffiliateCredential,
  MlLastTest,
  MlConnectionToken,
};

/**
 * Hook for affiliate programs page logic.
 * Manages affiliate credentials for multiple stores.
 * Uses ConnectRPC useAffiliateCredentialsConnect for all operations.
 */
export function useAffiliatePrograms(): UseAffiliateProgramsReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const {
    connect: mlConnect,
    createLink: mlCreateLink,
    preview: mlPreview,
  } = useMercadoLivreConnect();

  // Use ConnectRPC hook for credentials
  const {
    credentials: connectCredentials,
    loading: connectLoading,
    upsertCredential,
    refetch: refetchCredentials,
  } = useAffiliateCredentialsConnect();

  const [credentials, setCredentials] = useState<Record<StoreKey, AffiliateCredential>>(
    {} as Record<StoreKey, AffiliateCredential>
  );
  const [saving, setSaving] = useState<StoreKey | null>(null);
  const [testing, setTesting] = useState<StoreKey | null>(null);
  const [mlLastTest, setMlLastTest] = useState<MlLastTest | null>(null);

  const formatSoldQuantity = useCallback(
    (q: number | null | undefined): string | null => {
      if (q == null || !Number.isFinite(q)) return null;
      if (q >= 1000) {
        const mil = Math.round((q / 1000) * 10) / 10;
        const s = String(mil).replace('.', ',');
        return t('pages.affiliatePrograms.sales.thousandsSold', { count: s });
      }
      return t('pages.affiliatePrograms.sales.unitsSold', { count: q });
    },
    [t]
  );

  const getStoreDescription = useCallback(
    (storeKey: StoreKey): string => {
      return t(`pages.affiliatePrograms.descriptions.${storeKey}`);
    },
    [t]
  );

  // Convert ConnectRPC credentials to local format
  const convertToLocalCredentials = useCallback(
    (apiCredentials: LocalAffiliateCredential[]): Record<StoreKey, AffiliateCredential> => {
      const credsMap = {} as Record<StoreKey, AffiliateCredential>;

      // Initialize with defaults for all stores
      for (const config of STORE_CONFIGS) {
        credsMap[config.key] = {
          store: config.key,
          credentials: {},
          auto_generate: true,
          is_active: false,
        };
      }

      // Map API credentials
      for (const cred of apiCredentials) {
        let cleanCredentials = { ...cred.credentials };
        let selectedBrands: string[] | undefined;

        if (cred.store === 'awin' && cred.credentials.selected_brands) {
          try {
            selectedBrands = JSON.parse(cred.credentials.selected_brands);
          } catch (error: unknown) {
            captureHookError(error, 'useAffiliatePrograms', 'parseAwinBrands');
            selectedBrands = undefined;
          }
          const { selected_brands, ...rest } = cred.credentials;
          cleanCredentials = rest;
        }

        if (cred.store === 'ml') {
          cleanCredentials = {
            tag: String(cred.credentials?.tag || '').trim(),
          };
        }

        credsMap[cred.store] = {
          id: cred.id,
          store: cred.store,
          credentials: cleanCredentials,
          auto_generate: cred.auto_generate,
          is_active: cred.is_active,
          selected_brands: selectedBrands,
        };
      }

      return credsMap;
    },
    []
  );

  // Sync credentials from ConnectRPC hook
  useEffect(() => {
    if (connectCredentials.length > 0) {
      setCredentials(convertToLocalCredentials(connectCredentials));
    } else if (!connectLoading) {
      // Initialize with defaults if no credentials exist
      const defaults = {} as Record<StoreKey, AffiliateCredential>;
      for (const config of STORE_CONFIGS) {
        defaults[config.key] = {
          store: config.key,
          credentials: {},
          auto_generate: true,
          is_active: false,
        };
      }
      setCredentials(defaults);
    }
  }, [connectCredentials, connectLoading, convertToLocalCredentials]);

  const updateCredentialField = useCallback((storeKey: StoreKey, field: string, value: string) => {
    setCredentials((prev) => ({
      ...prev,
      [storeKey]: {
        ...prev[storeKey],
        credentials: {
          ...(prev[storeKey]?.credentials || {}),
          [field]: value,
        },
      },
    }));
  }, []);

  const toggleAutoGenerate = useCallback((storeKey: StoreKey) => {
    setCredentials((prev) => ({
      ...prev,
      [storeKey]: {
        ...prev[storeKey],
        auto_generate: !prev[storeKey].auto_generate,
      },
    }));
  }, []);

  const toggleBrand = useCallback((storeKey: StoreKey, brand: string) => {
    setCredentials((prev) => {
      const current = prev[storeKey]?.selected_brands || [];
      const updated = current.includes(brand)
        ? current.filter((b) => b !== brand)
        : [...current, brand];

      return {
        ...prev,
        [storeKey]: {
          ...prev[storeKey],
          selected_brands: updated,
        },
      };
    });
  }, []);

  const handleSave = useCallback(
    async (storeKey: StoreKey) => {
      setSaving(storeKey);

      try {
        if (!user?.id) {
          toast({
            title: t('pages.affiliatePrograms.errors.authError'),
            description: t('pages.affiliatePrograms.errors.authErrorMessage'),
            variant: 'destructive',
          });
          setSaving(null);
          return;
        }

        const cred = credentials[storeKey];
        if (!cred) {
          toast({
            title: t('pages.affiliatePrograms.errors.credentialNotFound'),
            description: t('pages.affiliatePrograms.errors.credentialNotFoundMessage'),
            variant: 'destructive',
          });
          setSaving(null);
          return;
        }

        const config = STORE_CONFIGS.find((c) => c.key === storeKey);

        if (storeKey === 'ml') {
          const tokenRaw = String(cred.credentials?.connectionToken || '').trim();
          const currentTag = String(cred.credentials?.tag || '').trim();

          if (!tokenRaw) {
            if (!cred?.is_active) {
              toast({
                title: t('pages.affiliatePrograms.errors.tokenRequired'),
                description: t('pages.affiliatePrograms.errors.tokenRequiredMessage'),
                variant: 'destructive',
              });
              setSaving(null);
              return;
            }

            if (!currentTag) {
              toast({
                title: t('pages.affiliatePrograms.errors.tagRequired'),
                description: t('pages.affiliatePrograms.errors.tagRequiredMessage'),
                variant: 'destructive',
              });
              setSaving(null);
              return;
            }

            await mlConnect({
              tag: currentTag,
              autoGenerate: Boolean(cred.auto_generate),
            });

            await upsertCredential({
              store: 'ml',
              credentials: { tag: currentTag },
              auto_generate: cred.auto_generate,
              is_active: true,
            });

            toast({ title: t('pages.affiliatePrograms.ml.updatedSuccess') });
            await refetchCredentials();
            return;
          }

          const decoded = decodeMlConnectionToken(tokenRaw);
          if ('error' in decoded) {
            toast({
              title: t('pages.affiliatePrograms.ml.tokenInvalid'),
              description: decoded.error,
              variant: 'destructive',
            });
            setSaving(null);
            return;
          }

          const tag = String(currentTag || decoded.tag || '').trim();
          const cookie = String(decoded.cookie || '').trim();
          const csrfToken = decoded.csrfToken ? String(decoded.csrfToken).trim() : '';

          if (!tag || !cookie) {
            toast({
              title: t('pages.affiliatePrograms.errors.fieldsRequired'),
              description: t('pages.affiliatePrograms.errors.fieldsRequiredMessage'),
              variant: 'destructive',
            });
            setSaving(null);
            return;
          }

          await mlConnect({
            tag,
            cookie,
            csrfToken: csrfToken || undefined,
            autoGenerate: Boolean(cred.auto_generate),
          });

          await upsertCredential({
            store: 'ml',
            credentials: { tag },
            auto_generate: cred.auto_generate,
            is_active: true,
          });

          setCredentials((prev) => ({
            ...prev,
            ml: {
              ...cred,
              credentials: {
                ...cred.credentials,
                connectionToken: '',
              },
            },
          }));

          toast({ title: t('pages.affiliatePrograms.ml.connectedSuccess') });
          await refetchCredentials();
          return;
        }

        if (storeKey === 'shein' && cred.credentials?.affiliateLink) {
          const link = cred.credentials.affiliateLink.trim();
          if (!link.startsWith('https://onelink.shein.com/')) {
            toast({
              title: t('pages.affiliatePrograms.shein.invalidLink'),
              description: t('pages.affiliatePrograms.shein.invalidLinkMessage'),
              variant: 'destructive',
            });
            setSaving(null);
            return;
          }
        }

        const allFieldsFilled = config?.fields.every((field) =>
          cred.credentials?.[field.key]?.trim()
        );

        const credentialsToSave = {
          ...cred.credentials,
          ...(storeKey === 'awin' && cred.selected_brands
            ? { selected_brands: JSON.stringify(cred.selected_brands) }
            : {}),
        };

        // Use ConnectRPC to upsert credential
        await upsertCredential({
          store: storeKey,
          credentials: credentialsToSave,
          auto_generate: cred.auto_generate,
          is_active: allFieldsFilled || false,
        });

        await refetchCredentials();
      } catch (error: unknown) {
        captureHookError(error, 'useAffiliatePrograms', 'handleSave');
        const errorMessage =
          error instanceof Error ? error.message : t('pages.affiliatePrograms.errors.saveUnknown');
        toast({
          title: t('pages.affiliatePrograms.errors.save'),
          description: errorMessage,
          variant: 'destructive',
        });
      } finally {
        setSaving(null);
      }
    },
    [credentials, user?.id, toast, t, refetchCredentials, mlConnect, upsertCredential]
  );

  const handleTest = useCallback(
    async (storeKey: StoreKey) => {
      setTesting(storeKey);
      try {
        const cred = credentials[storeKey];
        const config = STORE_CONFIGS.find((c) => c.key === storeKey);
        const allFieldsFilled =
          storeKey === 'ml'
            ? Boolean(cred?.is_active)
            : config?.fields.every((field) => cred.credentials[field.key]?.trim());

        if (!allFieldsFilled) {
          throw new Error(t('pages.affiliatePrograms.errors.fillAllFields'));
        }

        if (storeKey === 'ml') {
          if (!cred?.is_active) {
            throw new Error(t('pages.affiliatePrograms.errors.connectFirst'));
          }
          const originUrl = String(cred.credentials?.testProductUrl || '').trim();
          if (!originUrl) {
            throw new Error(t('pages.affiliatePrograms.errors.testUrlRequired'));
          }

          const linkUrls = await mlCreateLink(originUrl);
          const longUrl = linkUrls[0]?.longUrl || null;
          const shortUrl = linkUrls[0]?.shortUrl || null;

          const preview = await mlPreview(originUrl);

          setMlLastTest({
            originUrl,
            shortUrl,
            longUrl,
            title: preview?.title ?? null,
            price: preview?.price ?? null,
            currency: preview?.currency ?? null,
            thumbnail: preview?.imageUrl ?? null,
            condition: preview?.condition ?? null,
            soldQuantity: preview?.soldQuantity ?? null,
            ratingAverage: preview?.ratingAverage ?? null,
            reviewsTotal: preview?.reviewsTotal ?? null,
            originalPrice: preview?.originalPrice ?? null,
            discountPercent: preview?.discountPercent ?? null,
            installmentsText: preview?.installmentsText ?? null,
          });

          toast({
            title: t('pages.affiliatePrograms.ml.testSuccess'),
            description:
              (preview?.title && preview?.price != null
                ? `${preview.title} - ${preview.currency || 'BRL'} ${preview.price}`
                : null) ||
              shortUrl ||
              longUrl ||
              t('pages.affiliatePrograms.ml.linkGenerated'),
          });
          return;
        }

        await new Promise((resolve) => setTimeout(resolve, 1500));

        toast({
          title: t('pages.affiliatePrograms.success.validated'),
          description: t('pages.affiliatePrograms.success.validatedMessage', {
            store: config?.name,
          }),
        });
      } catch (error: unknown) {
        captureHookError(error, 'useAffiliatePrograms', 'handleTest');
        const errorMessage =
          error instanceof Error
            ? error.message
            : t('pages.affiliatePrograms.errors.testErrorMessage');
        toast({
          title: t('pages.affiliatePrograms.errors.testError'),
          description: errorMessage,
          variant: 'destructive',
        });
      } finally {
        setTesting(null);
      }
    },
    [credentials, toast, t, mlCreateLink, mlPreview]
  );

  return {
    credentials,
    loading: connectLoading,
    saving,
    testing,
    mlLastTest,
    storeConfigs: STORE_CONFIGS,
    enabledStores: ENABLED_STORES,
    awinBrands: AWIN_BRANDS,
    updateCredentialField,
    toggleAutoGenerate,
    toggleBrand,
    handleSave,
    handleTest,
    decodeMlToken: decodeMlConnectionToken,
    formatSoldQuantity,
    getStoreDescription,
  };
}
