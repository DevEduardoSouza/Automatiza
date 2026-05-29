-- Migration: 052_automations_repeat_when_finished.up.sql
-- Adiciona repeat_when_finished em automations: quando true (default),
-- o modo LIST reseta automaticamente os items consumidos da lista
-- vinculada assim que todos foram enviados, e a automation continua
-- rodando em loop. Quando false, a automation para após enviar todos
-- os items (precisa do reset manual via UI).
--
-- Default TRUE porque é o comportamento que o cliente quer por padrão
-- (paridade com o que ele esperava do affilipro).

ALTER TABLE automations
    ADD COLUMN IF NOT EXISTS repeat_when_finished BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN automations.repeat_when_finished IS
    'LIST mode: quando true, reseta itens consumidos automaticamente ao esgotar a lista e continua o loop. Default TRUE.';
