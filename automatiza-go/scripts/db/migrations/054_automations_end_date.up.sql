-- Migration: 054_automations_end_date.up.sql
-- Adiciona end_date em automations: data calendário até a qual o
-- scheduler ativa a automation. NULL (default) = sem expiração, roda
-- enquanto a automation estiver ativa. Quando setado e CURRENT_DATE >
-- end_date, scheduler para de disparar (mesmo padrão silencioso do
-- start_date — não muda status, só pula).
--
-- Útil para campanhas temporárias (Black Friday, Dia das Mães, etc)
-- onde o cliente já sabe o último dia que quer enviar.
--
-- Schema permite end_date < start_date (validation fica no app);
-- nesse caso, automation nunca dispara — comportamento esperado por
-- ser config explícita do user.

ALTER TABLE automations
    ADD COLUMN IF NOT EXISTS end_date DATE;

COMMENT ON COLUMN automations.end_date IS
    'Data calendário até a qual o scheduler ativa a automation. NULL = sem expiração.';
