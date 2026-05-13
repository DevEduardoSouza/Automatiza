-- Multi-chip fan-out por automação (Gerar Oferta).
--
-- Modelo da 045: cada automação aponta pra UM chip via instance_id.
-- Pra disparar a mesma oferta de 4 chips, o operador precisava criar
-- 4 automações idênticas, cada uma com seu chip — quadruplicava a
-- API call da Shopee/ML por dispatch e dificultava manutenção.
--
-- A partir daqui a automação carrega uma LISTA de chips. O dispatcher
-- chama a API do produto 1x e itera sobre os chips na lista — pra
-- cada grupo destino, escolhe o primeiro chip da lista que participa
-- daquele grupo (via groups.participating_instance_ids da migration
-- 044) e envia através dele.
--
-- instance_id (single) é mantida no schema só pra rollback rápido —
-- o código novo lê apenas instance_ids. Migrations seguintes podem
-- dropar a coluna antiga depois que estabilizar.

ALTER TABLE automations
    ADD COLUMN IF NOT EXISTS instance_ids INTEGER[];

-- Backfill: automações com instance_id setado herdam um array de 1
-- elemento. Idempotente — só popula quando ainda está NULL.
UPDATE automations
SET instance_ids = ARRAY[instance_id]
WHERE instance_id IS NOT NULL
  AND instance_ids IS NULL;

-- Index GIN pra acelerar buscas "automações que usam o chip X" e
-- enforcement de FK lógica (se um chip for deletado, dá pra varrer
-- automations e remover o ID do array). Sem isso o EXPLAIN cai em
-- Seq Scan em qualquer query de pertinência.
CREATE INDEX IF NOT EXISTS idx_automations_instance_ids
    ON automations USING GIN (instance_ids);
