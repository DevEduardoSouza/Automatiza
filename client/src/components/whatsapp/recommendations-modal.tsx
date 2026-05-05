import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import {
  AlertTriangle,
  Archive,
  CheckCircle2,
  Layers,
  Smartphone,
  XCircle,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

const STORAGE_KEY = 'whatsapp-recommendations-seen';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.06)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  green: '#16a34a',
  amber: '#d97706',
  red: '#dc2626',
};

interface RecommendationsModalProps {
  open: boolean;
  onClose: () => void;
}

interface Section {
  icon: typeof Smartphone;
  iconColor: string;
  title: string;
  bullets: string[];
}

/**
 * Educational modal listing best practices for operating multiple WhatsApp
 * chips without getting banned. Mirrors the operational guidance from the
 * affilipro competitor research, adapted to our quotas.
 *
 * Auto-opens on first visit to /whatsapp via the localStorage flag managed
 * by markRecommendationsSeen()/hasSeenRecommendations() exported below.
 * Re-openable via the "Recomendações" button on the page header.
 */
export function RecommendationsModal({ open, onClose }: RecommendationsModalProps) {
  const { t } = useTranslation();

  const sections: Section[] = [
    {
      icon: Smartphone,
      iconColor: colors.green,
      title: t(
        'pages.whatsapp.recommendations.phoneActive.title',
        'Mantenha o celular sincronizado sempre ativo'
      ),
      bullets: [
        t(
          'pages.whatsapp.recommendations.phoneActive.b1',
          'Deixe o aparelho ligado e conectado à internet'
        ),
      ],
    },
    {
      icon: Archive,
      iconColor: colors.green,
      title: t('pages.whatsapp.recommendations.archive.title', 'Organize suas conversas'),
      bullets: [
        t(
          'pages.whatsapp.recommendations.archive.b1',
          'Arquive todas as conversas do número para reduzir o volume de mensagens carregadas e melhorar o desempenho'
        ),
        t(
          'pages.whatsapp.recommendations.archive.b2',
          'Arquivar conversas não impede o recebimento de mensagens pelo sistema'
        ),
      ],
    },
    {
      icon: XCircle,
      iconColor: colors.red,
      title: t('pages.whatsapp.recommendations.noMultiSession.title', 'Evite múltiplas conexões'),
      bullets: [
        t(
          'pages.whatsapp.recommendations.noMultiSession.b1',
          'Não use o mesmo número em outros sistemas ou navegadores'
        ),
        t(
          'pages.whatsapp.recommendations.noMultiSession.b2',
          'Conexões simultâneas podem causar desconexões e perda de sessão'
        ),
      ],
    },
    {
      icon: Layers,
      iconColor: colors.wine,
      title: t(
        'pages.whatsapp.recommendations.structure.title',
        'Estrutura recomendada'
      ),
      bullets: [
        t(
          'pages.whatsapp.recommendations.structure.b1',
          'Limite de 10 números por conta. Distribua os grupos entre eles para evitar sobrecarga.'
        ),
        t(
          'pages.whatsapp.recommendations.structure.b2',
          'Até 20 grupos por número. Acima disso, o WhatsApp começa a interpretar como spam.'
        ),
        t(
          'pages.whatsapp.recommendations.structure.b3',
          'Use números dedicados para envio (não monitoramento) — números focados sofrem menos rate-limit'
        ),
      ],
    },
    {
      icon: AlertTriangle,
      iconColor: colors.red,
      title: t('pages.whatsapp.recommendations.banRisk.title', 'Atenção ao risco de banimento'),
      bullets: [
        t(
          'pages.whatsapp.recommendations.banRisk.b1',
          'O WhatsApp possui sistemas automáticos que analisam o comportamento das contas'
        ),
        t(
          'pages.whatsapp.recommendations.banRisk.b2',
          'Envios excessivos, repetitivos ou fora de padrão humano podem resultar em bloqueio temporário, limitação de envio ou banimento'
        ),
        t(
          'pages.whatsapp.recommendations.banRisk.b3',
          'Use número dedicado, não use seu número pessoal e respeite os limites recomendados'
        ),
      ],
    },
  ];

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-start gap-3 mb-2">
          <div
            className="rounded-full p-2 shrink-0"
            style={{ backgroundColor: 'rgba(34,197,94,0.1)' }}
          >
            <CheckCircle2 className="h-5 w-5" style={{ color: colors.green }} />
          </div>
          <div className="flex-1">
            <DialogTitle className="text-lg">
              {t(
                'pages.whatsapp.recommendations.title',
                'Recomendações para o melhor funcionamento'
              )}
            </DialogTitle>
            <DialogDescription className="mt-1 text-sm">
              {t(
                'pages.whatsapp.recommendations.subtitle',
                'Para evitar desconexões, sincronizações fora de hora e banimento dos números, siga as orientações abaixo.'
              )}
            </DialogDescription>
          </div>
        </div>

        <div className="space-y-4 mt-4">
          {sections.map((section) => {
            const Icon = section.icon;
            return (
              <div
                key={section.title}
                className="rounded-xl border p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="flex items-start gap-3 mb-2">
                  <Icon className="h-5 w-5 shrink-0 mt-0.5" style={{ color: section.iconColor }} />
                  <h4 className="font-semibold text-sm" style={{ color: '#18181b' }}>
                    {section.title}
                  </h4>
                </div>
                <ul className="space-y-1 ml-8">
                  {section.bullets.map((bullet) => (
                    <li
                      key={bullet}
                      className="text-xs leading-relaxed"
                      style={{ color: colors.wineTextMuted }}
                    >
                      • {bullet}
                    </li>
                  ))}
                </ul>
              </div>
            );
          })}
        </div>

        <div className="mt-5 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="inline-flex items-center justify-center rounded-full bg-[#8D2D53] px-5 py-2 text-sm font-semibold text-white shadow-sm transition-all hover:-translate-y-0.5 hover:bg-[#732242] hover:shadow-md"
          >
            {t('pages.whatsapp.recommendations.understood', 'Entendi')}
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

/**
 * Returns true once the user has dismissed the recommendations modal at
 * least once. Used by the WhatsApp page to auto-open the modal on first
 * visit only — subsequent visits keep it closed unless the user clicks
 * the "Recomendações" button manually.
 */
export function hasSeenRecommendations(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'true';
  } catch {
    // localStorage may be unavailable in strict-cookie browsers; treat as seen
    // to avoid spamming the modal on every page load.
    return true;
  }
}

export function markRecommendationsSeen(): void {
  try {
    localStorage.setItem(STORAGE_KEY, 'true');
  } catch {
    // Ignore — the only consequence is the modal opening again next visit.
  }
}
