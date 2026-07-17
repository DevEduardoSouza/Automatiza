import { Kanban } from '@/components/support/kanban/kanban';
import { SupportShell } from './support';

/**
 * Aba Kanban do painel de suporte (/support/kanban): quadro estilo Trello para
 * a equipe acompanhar a correção de bugs — cadastrar cards, movê-los pelas
 * colunas de resolução e anexar documentos. Reusa o shell do painel.
 */
export default function SupportKanban() {
  return (
    <SupportShell>
      <Kanban />
    </SupportShell>
  );
}
