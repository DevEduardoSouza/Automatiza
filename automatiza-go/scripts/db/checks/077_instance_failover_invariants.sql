-- Teste dos INVARIANTES de schema da migração 077 (chip reserva).
--
-- SOMENTE para banco local descartável. NUNCA rodar em produção: o script
-- insere fixtures e provoca violações de constraint de propósito.
--
-- POR QUE ESTE ARQUIVO NÃO MORA EM migrations/ E NÃO SE CHAMA .verify.sql:
-- o runner (scripts/db/run-migrations.sh) trata `<migração>.verify.sql` como
-- VERIFICADOR e o executa CONTRA PRODUÇÃO junto da migração. Com o nome antigo,
-- este arquivo rodou no deploy de 23/07/2026: a primeira violação proposital
-- virou `migration_verify_failed` e travou todos os deploys do repositório.
-- (Produção saiu ilesa — o runner roda o verificador em transação e o erro
-- reverteu as fixtures — mas por sorte, não por desenho.)
-- Um comentário "não rode em produção" não protege nada: o runner lê a
-- extensão, não o texto.
--
-- Uso:
--   psql -v ON_ERROR_STOP=0 -f 077_instance_failover.verify.sql
--
-- Cada bloco marcado "deve FALHAR" tem que produzir ERROR. Um bloco que passa
-- em silêncio é uma garantia que NÃO existe — e o desenho inteiro da feature
-- depende de o banco, não do código, impedir estes estados.

\set ON_ERROR_STOP 0

-- =============================================================================
-- Estado inicial limpo (o script é reexecutável)
-- =============================================================================
UPDATE instance_failover_policies
   SET state = 'active', current_operation_id = NULL
 WHERE id IN (900500, 900501, 900502);
-- A política cai primeiro: o CASCADE leva as roles junto, e sem ela nada mais
-- prende as instâncias (as FKs de papel são RESTRICT).
DELETE FROM instance_failover_policies WHERE id IN (900500, 900501, 900502);
DELETE FROM instance_failover_events WHERE policy_id_snapshot IN (900500, 900501, 900502);
DELETE FROM instance_failover_operations WHERE policy_id_snapshot IN (900500, 900501, 900502);
DELETE FROM instance_failover_dry_runs WHERE policy_id_snapshot IN (900500, 900501, 900502);
DELETE FROM instance_failover_roles WHERE instance_id IN (9001, 9002, 9003, 9004, 9005);
DELETE FROM instances WHERE id IN (9001, 9002, 9003, 9004, 9005);
DELETE FROM profiles WHERE id IN
  ('11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222');

-- =============================================================================
-- Fixtures
-- =============================================================================
INSERT INTO profiles (id, email) VALUES
 ('11111111-1111-1111-1111-111111111111', 'verify-a@test.local'),
 ('22222222-2222-2222-2222-222222222222', 'verify-b@test.local')
ON CONFLICT DO NOTHING;

INSERT INTO instances (id, user_id, instance_id) VALUES
 (9001, '11111111-1111-1111-1111-111111111111', 'verify-a1'),
 (9002, '11111111-1111-1111-1111-111111111111', 'verify-a2'),
 (9003, '11111111-1111-1111-1111-111111111111', 'verify-a3'),
 (9005, '11111111-1111-1111-1111-111111111111', 'verify-a5'),
 (9004, '22222222-2222-2222-2222-222222222222', 'verify-b1')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- 13.1.1 — mesma conta garantida por FK composta
-- =============================================================================
\echo '### cross-tenant (reserva de outro dono) -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_policies (user_id, primary_instance_id, backup_instance_id)
VALUES ('11111111-1111-1111-1111-111111111111', 9001, 9004);
ROLLBACK;

-- =============================================================================
-- 13.1.2 — primária = reserva
-- =============================================================================
\echo '### primaria = reserva -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_policies (user_id, primary_instance_id, backup_instance_id)
VALUES ('11111111-1111-1111-1111-111111111111', 9001, 9001);
ROLLBACK;

-- =============================================================================
-- 13.1.5 — modo automático é impossível no banco
-- =============================================================================
\echo '### mode = automatic -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_policies (user_id, primary_instance_id, backup_instance_id, mode)
VALUES ('11111111-1111-1111-1111-111111111111', 9001, 9002, 'automatic');
ROLLBACK;

-- =============================================================================
-- Caminho feliz: política + 2 papéis na MESMA transação (trigger DEFERRED)
-- =============================================================================
\echo '### policy + 2 papeis na mesma tx -> deve PASSAR'
BEGIN;
INSERT INTO instance_failover_policies (id, user_id, primary_instance_id, backup_instance_id)
VALUES (900500, '11111111-1111-1111-1111-111111111111', 9001, 9002);
INSERT INTO instance_failover_roles (instance_id, policy_id, user_id, role) VALUES
 (9001, 900500, '11111111-1111-1111-1111-111111111111', 'primary'),
 (9002, 900500, '11111111-1111-1111-1111-111111111111', 'backup');
COMMIT;
SELECT 'policies=' || count(*) AS ok FROM instance_failover_policies WHERE id = 900500;

-- =============================================================================
-- 13.1.4 — trigger exige exatamente 2 papéis corretos
-- =============================================================================
\echo '### policy com 1 papel so -> deve FALHAR no commit'
BEGIN;
INSERT INTO instance_failover_policies (id, user_id, primary_instance_id, backup_instance_id)
VALUES (900501, '11111111-1111-1111-1111-111111111111', 9003, 9005);
INSERT INTO instance_failover_roles (instance_id, policy_id, user_id, role)
VALUES (9003, 900501, '11111111-1111-1111-1111-111111111111', 'primary');
COMMIT;

\echo '### trocar a reserva SEM reescrever os papeis -> deve FALHAR'
BEGIN;
UPDATE instance_failover_policies SET backup_instance_id = 9003 WHERE id = 900500;
COMMIT;

-- =============================================================================
-- 13.1.3 — um chip ocupa no máximo UM papel no sistema
-- =============================================================================
\echo '### mesmo chip em 2o papel -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_roles (instance_id, policy_id, user_id, role)
VALUES (9001, 900500, '11111111-1111-1111-1111-111111111111', 'backup');
ROLLBACK;

\echo '### chip ja e primaria de outra politica -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_policies (user_id, primary_instance_id, backup_instance_id)
VALUES ('11111111-1111-1111-1111-111111111111', 9001, 9003);
ROLLBACK;

\echo '### A<->B (reserva de A vira primaria de B) -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_policies (id, user_id, primary_instance_id, backup_instance_id)
VALUES (900502, '11111111-1111-1111-1111-111111111111', 9002, 9003);
INSERT INTO instance_failover_roles (instance_id, policy_id, user_id, role) VALUES
 (9002, 900502, '11111111-1111-1111-1111-111111111111', 'primary'),
 (9003, 900502, '11111111-1111-1111-1111-111111111111', 'backup');
COMMIT;

-- =============================================================================
-- Coerência de estado × operação corrente
-- =============================================================================
\echo '### state=switching sem operacao corrente -> deve FALHAR'
BEGIN;
UPDATE instance_failover_policies SET state = 'switching' WHERE id = 900500;
COMMIT;

\echo '### limiar abaixo do minimo (900s) -> deve FALHAR'
BEGIN;
UPDATE instance_failover_policies SET unhealthy_after_seconds = 300 WHERE id = 900500;
COMMIT;

-- =============================================================================
-- 13.1.9 — chip em papel não pode ser apagado (RESTRICT)
-- =============================================================================
\echo '### delete de chip em papel -> deve FALHAR (RESTRICT)'
BEGIN;
DELETE FROM instances WHERE id = 9001;
ROLLBACK;

-- =============================================================================
-- 13.1.7 — idempotência
-- =============================================================================
\echo '### chave de idempotencia curta (<16 bytes) -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_dry_runs
  (policy_id, policy_id_snapshot, user_id, direction, idempotency_key,
   request_hash, policy_version, dry_run_ref, status, expires_at)
VALUES (900500, 900500, '11111111-1111-1111-1111-111111111111', 'failover', 'curta',
   repeat('a', 64), 1, repeat('b', 64), 'pass', now() + interval '15 min');
COMMIT;

\echo '### dry-run valido -> deve PASSAR'
BEGIN;
INSERT INTO instance_failover_dry_runs
  (id, policy_id, policy_id_snapshot, user_id, direction, idempotency_key,
   request_hash, policy_version, dry_run_ref, status, expires_at)
VALUES ('33333333-3333-3333-3333-333333333333', 900500, 900500,
   '11111111-1111-1111-1111-111111111111', 'failover', 'chave-idempotente-01',
   repeat('a', 64), 1, repeat('b', 64), 'pass', now() + interval '15 min');
COMMIT;

-- =============================================================================
-- 13.1.6 — no máximo UMA operação não terminal por usuário
-- =============================================================================
\echo '### 1a operacao nao terminal -> deve PASSAR'
BEGIN;
INSERT INTO instance_failover_operations
  (id, policy_id, policy_id_snapshot, user_id, kind, status, return_policy_state,
   idempotency_key, request_hash, dry_run_id, start_policy_version)
VALUES ('44444444-4444-4444-4444-444444444444', 900500, 900500,
   '11111111-1111-1111-1111-111111111111', 'failover', 'preparing', 'active',
   'chave-operacao-0001', repeat('c', 64), '33333333-3333-3333-3333-333333333333', 1);
COMMIT;

\echo '### 2a operacao nao terminal do MESMO usuario -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_operations
  (policy_id, policy_id_snapshot, user_id, kind, status, return_policy_state,
   idempotency_key, request_hash, dry_run_id, start_policy_version)
VALUES (900500, 900500, '11111111-1111-1111-1111-111111111111', 'failover',
   'preparing', 'active', 'chave-operacao-0002', repeat('d', 64),
   '33333333-3333-3333-3333-333333333333', 1);
COMMIT;

\echo '### vincular a operacao a politica -> deve PASSAR'
BEGIN;
UPDATE instance_failover_policies
   SET state = 'switching', current_operation_id = '44444444-4444-4444-4444-444444444444'
 WHERE id = 900500;
COMMIT;
SELECT 'policy state=' || state AS ok FROM instance_failover_policies WHERE id = 900500;

\echo '### apagar o dry-run que autorizou a operacao -> deve FALHAR (RESTRICT)'
BEGIN;
DELETE FROM instance_failover_dry_runs WHERE id = '33333333-3333-3333-3333-333333333333';
ROLLBACK;

-- =============================================================================
-- Ledger: tipos fechados e recibo idempotente
-- =============================================================================
\echo '### evento com recibo idempotente -> deve PASSAR'
BEGIN;
INSERT INTO instance_failover_events
  (policy_id, policy_id_snapshot, user_id, event_type, actor_type,
   idempotency_scope, idempotency_key)
VALUES (900500, 900500, '11111111-1111-1111-1111-111111111111',
   'policy_created', 'user', 'set_policy', 'chave-evento-000001');
COMMIT;

\echo '### mesmo recibo de novo -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_events
  (policy_id, policy_id_snapshot, user_id, event_type, actor_type,
   idempotency_scope, idempotency_key)
VALUES (900500, 900500, '11111111-1111-1111-1111-111111111111',
   'policy_created', 'user', 'set_policy', 'chave-evento-000001');
COMMIT;

\echo '### tipo de evento fora do vocabulario -> deve FALHAR'
BEGIN;
INSERT INTO instance_failover_events (policy_id_snapshot, user_id, event_type, actor_type)
VALUES (900500, '11111111-1111-1111-1111-111111111111', 'banimento_detectado', 'user');
COMMIT;

-- =============================================================================
-- Limpeza
-- =============================================================================
UPDATE instance_failover_policies
   SET state = 'active', current_operation_id = NULL
 WHERE id IN (900500, 900501, 900502);
DELETE FROM instance_failover_policies WHERE id IN (900500, 900501, 900502);
DELETE FROM instance_failover_events WHERE policy_id_snapshot IN (900500, 900501, 900502);
DELETE FROM instance_failover_operations WHERE policy_id_snapshot IN (900500, 900501, 900502);
DELETE FROM instance_failover_dry_runs WHERE policy_id_snapshot IN (900500, 900501, 900502);
DELETE FROM instance_failover_roles WHERE instance_id IN (9001, 9002, 9003, 9004, 9005);
DELETE FROM instances WHERE id IN (9001, 9002, 9003, 9004, 9005);
DELETE FROM profiles WHERE id IN
  ('11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222');

\echo '### verificacao concluida'
