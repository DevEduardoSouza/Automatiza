import type {
  GetUserActivityResponse,
  GetUserOverviewResponse,
  SupportAutomation,
  SupportChip,
} from '@/gen/support/v1/support_pb';

/**
 * Motor de diagnóstico do painel de suporte.
 *
 * Traduz o estado bruto de uma conta (chips, automações, cobrança, filas) nos
 * problemas concretos que causam "parou de enviar" — em linguagem que um
 * atendente não técnico entende, com a ação que resolve e a distinção clara
 * entre "você resolve com o cliente" e "encaminhe ao time técnico".
 *
 * A cópia de cada diagnóstico mora AQUI, junto da regra que o detecta, e não
 * no i18n: título/explicação/ação são inseparáveis da condição que os dispara
 * (é a tabela de erros do runbook virada em texto de atendimento). Num produto
 * de um idioma só, colocar os dois no mesmo lugar evita que a regra e o texto
 * saiam de sincronia. O "chrome" do painel (rótulos, botões) segue no i18n.
 *
 * Princípio (regra M2 do runbook): só vira problema o que tem prova ATUAL. Um
 * `last_error` de dias atrás pode já estar resolvido — por isso erros só entram
 * quando corroborados por estado vivo: automação pausada, erro recente
 * (`last_error_at` < 24h) ou falha na janela de atividade.
 */

export type Severity = 'high' | 'medium' | 'low' | 'info';

/** Quem resolve: o próprio atendimento com o cliente, ou o time técnico. */
export type Resolution = 'support' | 'escalate';

export interface Diagnosis {
  /** Chave estável para React key e dedupe. */
  id: string;
  severity: Severity;
  /** Título em uma linha, linguagem de cliente. */
  title: string;
  /** O que está acontecendo. */
  detail: string;
  /** O que fazer para resolver. */
  action: string;
  resolution: Resolution;
}

const SEVERITY_WEIGHT: Record<Severity, number> = { high: 0, medium: 1, low: 2, info: 3 };

/** Erro recente o bastante para ser tratado como problema atual (24h). */
function isRecent(iso: string | undefined, hours = 24): boolean {
  if (!iso) return false;
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return false;
  return Date.now() - t < hours * 3600_000;
}

function chipLabel(c: SupportChip): string {
  return c.friendlyName || c.instanceId;
}

function truncate(s: string, max = 140): string {
  return s.length > max ? `${s.slice(0, max)}…` : s;
}

/**
 * Traduz o texto de um `last_error` para um diagnóstico, seguindo a tabela de
 * assinaturas do runbook (CLAUDE.md §Assinaturas de erro → causa).
 * Retorna null quando o erro é melhor explicado por outra regra (ex.: chip
 * desconectado, que a regra de chips já cobre com mais precisão).
 */
function diagnoseError(auto: SupportAutomation): Omit<Diagnosis, 'id'> | null {
  const err = (auto.lastError ?? '').toLowerCase();
  const name = auto.name;

  // "instance X is not connected" já é coberto, com mais precisão, pela regra
  // de chip desconectado — não duplicar como erro de automação.
  if (/not connected|não conect|nao conect|desconect/.test(err)) return null;

  if (/invalid signature/.test(err)) {
    return {
      severity: 'medium',
      resolution: 'support',
      title: 'Credencial da Shopee incorreta',
      detail: `A automação "${name}" ${
        auto.status === 'paused' ? 'foi pausada porque' : 'está com'
      } a credencial da Shopee inválida (Invalid Signature).`,
      action:
        'Peça ao cliente para corrigir as credenciais da Shopee em Configurações → Programas de Afiliado. Depois reative a automação.',
    };
  }

  if (/10035/.test(err)) {
    return {
      severity: 'medium',
      resolution: 'support',
      title: 'Conta Shopee sem API de Afiliados',
      detail: `A conta de afiliado Shopee do cliente não tem a Open API ativa (erro 10035), então a automação "${name}" não consegue gerar links.`,
      action:
        'Oriente o cliente a ativar a Open API no painel de afiliado da Shopee. É permanente até ele ativar — não normaliza sozinho.',
    };
  }

  if (/10030/.test(err)) {
    return {
      severity: 'low',
      resolution: 'support',
      title: 'Limite temporário da Shopee',
      detail: `A Shopee aplicou um limite temporário de requisições (erro 10030) na automação "${name}".`,
      action:
        'Normaliza sozinho, geralmente em minutos. Não precisa ação. Se persistir por horas em horário de pico, encaminhe ao time técnico.',
    };
  }

  if (/client outdated|405/.test(err)) {
    return {
      severity: 'high',
      resolution: 'escalate',
      title: 'Sistema de WhatsApp desatualizado',
      detail:
        'O WhatsApp rejeitou o cliente de conexão (erro 405). Isso costuma afetar a plataforma inteira, não só esta conta.',
      action: 'Encaminhe IMEDIATAMENTE ao time técnico — não é algo que o cliente resolva.',
    };
  }

  if (/not participating|in that group/.test(err)) {
    return {
      severity: 'low',
      resolution: 'support',
      title: 'Chip removido de um grupo',
      detail: `Um chip da automação "${name}" foi removido de um grupo de destino.`,
      action: 'Peça ao cliente para readicionar o número do chip ao grupo.',
    };
  }

  if (/session not found|does not exist|órfã|orfa/.test(err)) {
    return {
      severity: 'medium',
      resolution: 'escalate',
      title: 'Sessão de WhatsApp sem cadastro',
      detail: `Um chip da automação "${name}" está com sessão órfã (existe no WhatsApp mas não no sistema).`,
      action: 'Encaminhe ao time técnico — há recuperação sem precisar de novo QR.',
    };
  }

  // Erro real, recente, mas fora da tabela conhecida.
  return {
    severity: 'medium',
    resolution: 'escalate',
    title: 'Erro recente na automação',
    detail: `A automação "${name}" registrou um erro: ${truncate(auto.lastError ?? '')}`,
    action: 'Encaminhe ao time técnico com este texto de erro.',
  };
}

/**
 * Deriva a lista de diagnósticos de uma conta, ordenada por severidade.
 * Sem problema detectado → lista vazia (o painel mostra o estado saudável).
 */
export function diagnose(
  overview: GetUserOverviewResponse | undefined,
  activity: GetUserActivityResponse | undefined
): Diagnosis[] {
  if (!overview?.profile) return [];

  const { profile, chips, automations } = overview;
  const out: Diagnosis[] = [];

  const activeAutos = automations.filter((a) => a.status === 'active');
  const failedByAuto = new Map(
    (activity?.automations ?? []).map((a) => [a.automationId, a.failedInWindow])
  );

  // 1) Cobrança: explica sozinha um "parou tudo", e some no resto da tela.
  if (profile.billingStatus && profile.billingStatus !== 'active') {
    out.push({
      id: 'billing',
      severity: 'high',
      resolution: 'support',
      title: 'Conta com cobrança pendente',
      detail: `A cobrança está com status "${profile.billingStatus}".${
        profile.billingSuspensionReason ? ` Motivo: ${profile.billingSuspensionReason}.` : ''
      } Enquanto isso, os envios ficam suspensos.`,
      action:
        'Peça ao cliente para regularizar o pagamento (Kiwify). Assim que o pagamento for confirmado, o acesso e os envios voltam automaticamente.',
    });
  }

  const connectedChips = chips.filter((c) => c.status === 'connected');

  // 2) Nenhum WhatsApp conectado.
  if (chips.length === 0) {
    const hasActive = activeAutos.length > 0;
    out.push({
      id: 'no-chips',
      severity: hasActive ? 'high' : 'info',
      resolution: 'support',
      title: 'Nenhum WhatsApp conectado',
      detail: hasActive
        ? 'O cliente não tem nenhum chip conectado, mas tem automações ativas — nada será enviado.'
        : 'O cliente ainda não conectou nenhum WhatsApp.',
      action:
        'Oriente o cliente a conectar um WhatsApp em Configurações → WhatsApp e escanear o QR code.',
    });
  } else if (connectedChips.length === 0) {
    // 3) Tem chips, todos caídos.
    out.push({
      id: 'all-chips-down',
      severity: 'high',
      resolution: 'support',
      title: 'Todos os WhatsApp desconectados',
      detail: 'Nenhum dos chips do cliente está conectado — nada será enviado até reconectar.',
      action:
        'Peça ao cliente para reconectar cada chip: Configurações → WhatsApp → reconectar e escanear o QR code.',
    });
  } else {
    // 4) Chip específico caído E usado por automação ativa (senão não é
    //    problema — um chip reserva parado ninguém sente).
    for (const chip of chips) {
      if (chip.status === 'connected') continue;
      const usedByActive = activeAutos.some((a) => a.instanceIds.includes(chip.instanceId));
      if (!usedByActive) continue;
      out.push({
        id: `chip-down:${chip.instanceId}`,
        severity: 'medium',
        resolution: 'support',
        title: `WhatsApp "${chipLabel(chip)}" desconectado`,
        detail: `O chip "${chipLabel(chip)}" está ${chip.status} e há automações ativas usando ele — elas não enviam por esse número.`,
        action: `Peça ao cliente para reconectar o "${chipLabel(chip)}" (Configurações → WhatsApp → reconectar e escanear o QR).`,
      });
    }
  }

  // 5) Erros de automação — só com prova atual (pausada, erro recente, ou
  //    falha na janela). Evita alarme com last_error velho já resolvido.
  for (const auto of automations) {
    if (!auto.lastError) continue;
    const fresh =
      auto.status === 'paused' ||
      isRecent(auto.lastErrorAt) ||
      (failedByAuto.get(auto.id) ?? 0) > 0;
    if (!fresh) continue;
    const d = diagnoseError(auto);
    if (d) out.push({ id: `err:${auto.id}`, ...d });
  }

  // 6) Backlog congestionado (número global do usuário).
  if ((activity?.queuedNow ?? 0) > 50) {
    out.push({
      id: 'backlog',
      severity: 'medium',
      resolution: 'escalate',
      title: 'Fila de envio congestionada',
      detail: `Há ${activity?.queuedNow} mensagens acumuladas aguardando envio.`,
      action:
        'Costuma escoar sozinho. Se o número não cair em alguns minutos, encaminhe ao time técnico.',
    });
  }

  // 7) Todas as automações pausadas sem ser por cobrança — "nada acontece"
  //    que a tela de cobrança não explica.
  if (
    automations.length > 0 &&
    activeAutos.length === 0 &&
    (!profile.billingStatus || profile.billingStatus === 'active')
  ) {
    out.push({
      id: 'all-paused',
      severity: 'low',
      resolution: 'support',
      title: 'Todas as automações estão pausadas',
      detail: 'Nenhuma automação do cliente está ativa, então nada é enviado.',
      action: 'Se o cliente espera que rodem, reative na tela de automações do painel dele.',
    });
  }

  // 8) Limite do plano atingido (informativo, ordena por último).
  if (profile.maxInstances > 0 && chips.length >= profile.maxInstances) {
    out.push({
      id: 'limit-chips',
      severity: 'info',
      resolution: 'support',
      title: 'Limite de chips do plano atingido',
      detail: `O cliente usa ${chips.length} de ${profile.maxInstances} chips do plano.`,
      action: 'Para adicionar mais chips, é preciso subir de plano.',
    });
  }
  if (profile.maxAutomations > 0 && automations.length >= profile.maxAutomations) {
    out.push({
      id: 'limit-autos',
      severity: 'info',
      resolution: 'support',
      title: 'Limite de automações do plano atingido',
      detail: `O cliente usa ${automations.length} de ${profile.maxAutomations} automações do plano.`,
      action: 'Para criar mais automações, é preciso subir de plano.',
    });
  }

  return out.sort((a, b) => SEVERITY_WEIGHT[a.severity] - SEVERITY_WEIGHT[b.severity]);
}
