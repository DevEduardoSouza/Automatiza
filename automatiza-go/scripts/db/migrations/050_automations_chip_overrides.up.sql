-- Roteamento manual chip->grupo no /offer-generate.
--
-- Multi-chip (migration 049) deixa a automação carregar N chips e o
-- dispatcher escolhe automaticamente qual chip envia em qual grupo. O
-- algoritmo atual ("primeiro preferido que participa") concentra a
-- carga no primeiro chip da lista quando vários chips estão no mesmo
-- grupo. Daí veio a queixa "só um número manda em tudo".
--
-- Esta coluna permite override por grupo: o operador fixa "este grupo
-- sai pelo chip X" no UI; o dispatcher consulta o mapa antes de cair
-- pro algoritmo automático. Grupos sem override seguem o automático.
--
-- Formato do JSON: { "wa_group_id": "instance_id_string", ... }
--   - chave  = groups.wa_group_id
--   - valor  = instances.id como string (consistente com instance_ids
--             que é INTEGER[] convertido pra string no proto)
--
-- Default '{}' mantém comportamento atual: automações antigas vêem
-- mapa vazio → dispatcher usa o algoritmo automático.

ALTER TABLE automations
    ADD COLUMN IF NOT EXISTS chip_overrides JSONB NOT NULL DEFAULT '{}'::jsonb;
