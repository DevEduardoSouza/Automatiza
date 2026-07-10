-- Migration: 060_automations_humanize_mode.up.sql
-- Adiciona humanize_mode em automations: preset de intensidade da
-- "humanização" (anti-ban) aplicada aos disparos. Valores: off, low,
-- medium, high. Quanto maior, mais o backend disfarça o comportamento
-- (delays variáveis, jitter, etc.) para reduzir risco de ban.
--
-- Default 'high' porque é o comportamento mais seguro por padrão
-- (máxima proteção anti-ban em novas automations).

ALTER TABLE automations
    ADD COLUMN IF NOT EXISTS humanize_mode TEXT NOT NULL DEFAULT 'high';

COMMENT ON COLUMN automations.humanize_mode IS
    'Intensidade da humanização anti-ban aplicada aos disparos: off, low, medium, high. Default high.';
