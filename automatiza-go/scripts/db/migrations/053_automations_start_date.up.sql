-- Migration: 053_automations_start_date.up.sql
-- Adiciona start_date em automations: data calendário a partir da qual o
-- scheduler pode ativar a automation. NULL (default) = sem agendamento,
-- automation roda imediatamente como hoje.
--
-- LIST mode usa pra começar a enviar a lista numa data futura (paridade
-- com o "Data de início" do affilipro). Modos search/monitor podem usar
-- o mesmo campo no futuro — sem restrição de tipo no schema.
--
-- TIMEZONE NOTE: DATE não carrega timezone. Comparação no scheduler usa
-- CURRENT_DATE (server timezone, configurada em postgres.conf). Em prod
-- o postgres roda em UTC e o servidor da app em America/Sao_Paulo — a
-- diferença pode adiantar/atrasar o "começa hoje" em até 3h dependendo
-- da hora local. Tratamos como aceitável pro caso de uso (cliente quer
-- "a partir de DD/MM"); precisão por segundo nunca foi requisito.

ALTER TABLE automations
    ADD COLUMN IF NOT EXISTS start_date DATE;

COMMENT ON COLUMN automations.start_date IS
    'Data calendário a partir da qual o scheduler ativa a automation. NULL = sem agendamento (roda imediatamente).';
