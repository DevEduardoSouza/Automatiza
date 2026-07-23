import { buildOccurrences } from '@/hooks/use-jobs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/**
 * Gate 0 da melhoria 8 — a classificação do caso "27" virando regressão.
 *
 * O relato de produção era "27 mensagens". A investigação provou que os 27 jobs
 * tinham 27 `scheduled_for` DISTINTOS (24→29/07): expansão futura de uma
 * recorrência, não duplicata. Esta suíte trava esse comportamento: se alguém um
 * dia "consertar" a multiplicação achando que é bug, estes testes quebram.
 *
 * A duplicata real (mesmo horário repetido) é um fenômeno separado, coberto
 * pelo índice único do backend e por job_repository_test.go.
 */
describe('buildOccurrences', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Data-base fixa: buildOccurrences depende de `new Date()` para pular
    // horários já passados, então sem relógio congelado o teste é instável.
    vi.setSystemTime(new Date(2026, 6, 21, 12, 34, 0));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const keyOf = (d: Date): number => d.getTime();

  it('expande recorrência em ocorrências com horários todos distintos', () => {
    // 9 dias (24/07 a 01/08) × 3 horários = 27 — o formato do caso relatado.
    const occurrences = buildOccurrences({
      recurrence: 'daily',
      scheduledDate: '2026-07-24',
      timeList: ['09:00', '13:00', '19:00'],
      weekdays: [],
      intervalDays: 1,
      repeatUntil: '2026-08-01',
    });

    expect(occurrences).toHaveLength(27);

    // A prova do Gate 0: nenhum par de ocorrências compartilha horário, logo
    // nenhuma delas colide na chave lógica do backend.
    const distinct = new Set(occurrences.map(keyOf));
    expect(distinct.size).toBe(27);
  });

  it('mantém a ordem cronológica e não gera ocorrência no passado', () => {
    const occurrences = buildOccurrences({
      recurrence: 'daily',
      scheduledDate: '',
      timeList: ['08:00', '20:00'],
      weekdays: [],
      intervalDays: 1,
      repeatUntil: '2026-07-23',
    });

    const now = Date.now();
    for (const dt of occurrences) {
      expect(dt.getTime()).toBeGreaterThan(now);
    }
    for (let i = 1; i < occurrences.length; i++) {
      expect(occurrences[i].getTime()).toBeGreaterThan(occurrences[i - 1].getTime());
    }
  });

  it('recorrência semanal só cai nos dias escolhidos, sem repetir horário', () => {
    // Segundas (1) e quintas (4) de 22/07 a 12/08.
    const occurrences = buildOccurrences({
      recurrence: 'weekly',
      scheduledDate: '2026-07-22',
      timeList: ['10:00'],
      weekdays: [1, 4],
      intervalDays: 1,
      repeatUntil: '2026-08-12',
    });

    expect(occurrences.length).toBeGreaterThan(0);
    for (const dt of occurrences) {
      expect([1, 4]).toContain(dt.getDay());
    }
    expect(new Set(occurrences.map(keyOf)).size).toBe(occurrences.length);
  });

  it('"uma vez" cria exatamente um agendamento por horário informado', () => {
    const occurrences = buildOccurrences({
      recurrence: 'once',
      scheduledDate: '2026-07-25',
      timeList: ['09:00', '15:00'],
      weekdays: [],
      intervalDays: 1,
      repeatUntil: '',
    });

    expect(occurrences).toHaveLength(2);
    expect(new Set(occurrences.map(keyOf)).size).toBe(2);
  });

  it('respeita o teto de 200 ocorrências por submit', () => {
    // Sem "repetir até", o horizonte é 90 dias; 4 horários/dia passariam de 300.
    const occurrences = buildOccurrences({
      recurrence: 'daily',
      scheduledDate: '',
      timeList: ['06:00', '12:00', '18:00', '22:00'],
      weekdays: [],
      intervalDays: 1,
      repeatUntil: '',
    });

    expect(occurrences).toHaveLength(200);
    expect(new Set(occurrences.map(keyOf)).size).toBe(200);
  });
});
