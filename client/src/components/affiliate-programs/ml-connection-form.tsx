import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import type { MlLastTest } from '@/hooks/use-affiliate-programs';
import { useTranslation } from 'react-i18next';
import { MlTestResult } from './ml-test-result';

interface MlConnectionFormProps {
  connectionToken: string;
  tag: string;
  testProductUrl: string;
  mlLastTest: MlLastTest | null;
  onConnectionTokenChange: (value: string) => void;
  onTagChange: (value: string) => void;
  onTestProductUrlChange: (value: string) => void;
  onValidateToken: () => void;
  formatSoldQuantity: (q: number | null | undefined) => string | null;
}

/**
 * Form fields specific to Mercado Livre connection.
 * Handles connection token input, validation, tag field, and test product URL.
 */
export function MlConnectionForm({
  connectionToken,
  tag,
  testProductUrl,
  mlLastTest,
  onConnectionTokenChange,
  onTagChange,
  onTestProductUrlChange,
  onValidateToken,
  formatSoldQuantity,
}: MlConnectionFormProps) {
  const { t } = useTranslation();

  return (
    <>
      <div className="space-y-2">
        <Label htmlFor="ml-connection-token">
          {t('pages.affiliatePrograms.ml.connectionToken')}
        </Label>
        <Textarea
          id="ml-connection-token"
          placeholder={t('pages.affiliatePrograms.ml.connectionTokenPlaceholder')}
          value={connectionToken}
          onChange={(e) => onConnectionTokenChange(e.target.value)}
          className="min-h-[90px]"
        />
        <Button type="button" variant="outline" onClick={onValidateToken}>
          {t('pages.affiliatePrograms.ml.validateToken')}
        </Button>
        <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          {t('pages.affiliatePrograms.ml.extensionNote')}
        </p>
        <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          {t('pages.affiliatePrograms.ml.securityNote')}
        </p>
      </div>

      <div className="space-y-2">
        <Label htmlFor="ml-tag">{t('pages.affiliatePrograms.form.tag')}</Label>
        <Input
          id="ml-tag"
          type="text"
          placeholder={t('pages.affiliatePrograms.form.tagPlaceholder')}
          value={tag}
          onChange={(e) => onTagChange(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="ml-testProductUrl">{t('pages.affiliatePrograms.ml.testProductUrl')}</Label>
        <Input
          id="ml-testProductUrl"
          type="text"
          placeholder={t('pages.affiliatePrograms.ml.testProductUrlPlaceholder')}
          value={testProductUrl}
          onChange={(e) => onTestProductUrlChange(e.target.value)}
        />
        <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          {t('pages.affiliatePrograms.ml.testProductUrlHint')}
        </p>

        {mlLastTest?.originUrl && (
          <MlTestResult result={mlLastTest} formatSoldQuantity={formatSoldQuantity} />
        )}
      </div>
    </>
  );
}
