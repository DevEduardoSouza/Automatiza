-- Down documental da 076. Existe pela convenção do repositório e para testes
-- locais; NUNCA é executado em produção (o runner exclui *.down.sql e as
-- migrações do projeto são forward-only). Rollback operacional real =
-- GROUP_MEMBER_TRACKING_ENABLED=false + migração corretiva aditiva.
--
-- Ordem inversa da up: primeiro o que depende de `groups`, depois snapshots,
-- projeção e por fim o ledger.

DROP INDEX IF EXISTS idx_groups_member_reconcile_claim;

ALTER TABLE groups DROP COLUMN IF EXISTS members_reconcile_claimed_at;
ALTER TABLE groups DROP COLUMN IF EXISTS members_reconciled_at;
ALTER TABLE groups DROP COLUMN IF EXISTS member_tracking_started_at;

DROP TABLE IF EXISTS group_member_daily_snapshots;
DROP TABLE IF EXISTS group_memberships;
DROP TABLE IF EXISTS group_member_events;
