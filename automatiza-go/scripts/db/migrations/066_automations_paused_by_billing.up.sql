-- Simetria pausa/reativação do billing (root cause do "automação pausada pra sempre").
--
-- handleRefundOrLate pausava TODAS as automações ativas do usuário via
-- PauseUserAutomations, mas handlePurchaseApproved (aprovação/renovação) limpava
-- a suspensão SEM religar nada — não existia ResumeUserAutomations. Resultado:
-- quem regularizava o pagamento ficava com as automações pausadas em silêncio.
--
-- A coluna abaixo marca EXATAMENTE quais automações foram pausadas pelo billing,
-- para que o resume religue só essas — nunca as que o próprio usuário pausou de
-- propósito (isso evita o loop pausa/reativação e respeita a escolha do dono).
ALTER TABLE automations
    ADD COLUMN IF NOT EXISTS paused_by_billing BOOLEAN NOT NULL DEFAULT false;

-- Índice parcial: o resume filtra por (user_id, paused_by_billing=true); mantém a
-- religação barata mesmo com 2k+ automações.
CREATE INDEX IF NOT EXISTS idx_automations_paused_by_billing
    ON automations (user_id)
    WHERE paused_by_billing = true;

-- Backfill seguro: usuários que ESTÃO suspensos por billing agora tiveram todas
-- as automações pausadas por PauseUserAutomations no passado (coluna não existia
-- → paused_by_billing=false). Sem esta marcação, o fix do resume não religaria
-- quem já está suspenso ao regularizar. Marca só automações PAUSADAS de perfis
-- suspensos — seguro por definição (esses só voltam a rodar após pagamento).
UPDATE automations a
SET paused_by_billing = true
FROM profiles p
WHERE p.id = a.user_id
  AND a.status = 'paused'
  AND lower(coalesce(p.billing_status, '')) = 'suspended';
