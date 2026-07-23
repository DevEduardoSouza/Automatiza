import {
  FAILOVER_DISABLED_REASON,
  isFailoverDisabled,
  newIdempotencyKey,
} from '@/hooks/use-failover';
import { FailoverMode } from '@/gen/failover/v1/failover_pb';
import { describe, expect, it } from 'vitest';

describe('isFailoverDisabled', () => {
  // 13.8.1 — a seção só some pelo reason code correto.
  it('reconhece o reason code na metadata', () => {
    const metadata = new Headers();
    metadata.set('failover-reason', FAILOVER_DISABLED_REASON);
    expect(isFailoverDisabled({ metadata })).toBe(true);
  });

  it('reconhece o reason code na mensagem', () => {
    expect(
      isFailoverDisabled({ message: `${FAILOVER_DISABLED_REASON}: instance failover is disabled` })
    ).toBe(true);
  });

  // 13.8.2 — erro de rede/servidor NÃO pode ser confundido com "desligado".
  // Se fosse, um backend fora do ar faria o recurso "sumir" do painel — o tipo
  // de bug que ninguém reporta porque parece intencional.
  it('não confunde erro de rede com feature desligada', () => {
    expect(isFailoverDisabled({ message: 'Failed to fetch' })).toBe(false);
    expect(isFailoverDisabled({ message: '[internal] internal error' })).toBe(false);
    expect(isFailoverDisabled({ message: '[unavailable] connection refused' })).toBe(false);
    expect(isFailoverDisabled(null)).toBe(false);
    expect(isFailoverDisabled(undefined)).toBe(false);
  });

  it('não confunde outro FailedPrecondition com feature desligada', () => {
    // Cobertura bloqueada também é FailedPrecondition, mas a seção continua
    // visível — o usuário precisa ver o motivo.
    expect(
      isFailoverDisabled({ message: '[failed_precondition] failover coverage is incomplete' })
    ).toBe(false);
  });
});

describe('newIdempotencyKey', () => {
  // O backend exige 16..128 bytes ASCII imprimíveis (CHECK da migração 077).
  it('respeita o contrato de chave do backend', () => {
    const key = newIdempotencyKey();
    expect(key.length).toBeGreaterThanOrEqual(16);
    expect(key.length).toBeLessThanOrEqual(128);
    expect(/^[\x20-\x7E]+$/.test(key)).toBe(true);
  });

  it('gera uma chave nova a cada chamada', () => {
    // Cada tentativa é uma operação distinta; reusar a chave faria a segunda
    // virar replay da primeira.
    expect(newIdempotencyKey()).not.toBe(newIdempotencyKey());
  });
});

describe('contrato do enum de modo', () => {
  // 13.8.3 — não existe opção automática em lugar nenhum.
  it('só expõe UNSPECIFIED, MANUAL e ASSISTED', () => {
    const values = Object.values(FailoverMode).filter((v) => typeof v === 'number');
    expect(values.sort()).toEqual([0, 1, 2]);
    expect(FailoverMode.MANUAL).toBe(1);
    expect(FailoverMode.ASSISTED).toBe(2);
    expect(Object.keys(FailoverMode).some((k) => /AUTOMATIC/i.test(k))).toBe(false);
  });
});
