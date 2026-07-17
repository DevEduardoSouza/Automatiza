-- =============================================================================
-- Seed do Painel de Suporte — clientes fake para desenvolvimento local
-- =============================================================================
--
-- Popula o painel /support com clientes que reproduzem os casos REAIS de
-- atendimento, e não só "um cliente feliz": chip caído, conta suspensa,
-- espelho com credencial Shopee inválida, backlog preso na fila, e conta
-- sem chip nenhum. É contra esses cenários que o painel precisa ser julgado.
--
-- Uso:
--   docker exec -i automatiza-postgres psql -U automatiza -d automatiza \
--     -v ON_ERROR_STOP=1 < automatiza-go/scripts/db/seed_support.sql
--
-- Idempotente: apaga e recria os clientes de seed a cada execução. Só toca em
-- contas @seed.local (e no cliente.teste@example.com legado) — nenhuma outra
-- linha do banco é afetada.
--
-- Todos os clientes fake logam com a senha: Seed@2026
--
-- ATENÇÃO: este script NUNCA deve rodar em produção. Ele apaga dados e cria
-- contas. A guarda abaixo aborta se o banco parecer produção (>100 profiles) —
-- produção tem ~874, o dev tem uma mão cheia.
-- =============================================================================

DO $$
DECLARE
    profile_count INT;
BEGIN
    SELECT count(*) INTO profile_count FROM profiles;
    IF profile_count > 100 THEN
        RAISE EXCEPTION
            'ABORTADO: % profiles neste banco — isto parece PRODUÇÃO. O seed de suporte apaga e cria contas e nunca deve rodar aqui.',
            profile_count;
    END IF;
END $$;

-- Limpeza dos clientes de seed. ON DELETE CASCADE em instances/automations/
-- groups/ml_link_lists/user_credentials cuida das dependências; dispatch_logs
-- e automation_runs saem explicitamente por não terem cascade a partir daqui.
DO $$
DECLARE
    seed_ids UUID[];
BEGIN
    SELECT array_agg(id) INTO seed_ids
    FROM profiles
    WHERE email LIKE '%@seed.local' OR email = 'cliente.teste@example.com';

    IF seed_ids IS NOT NULL THEN
        DELETE FROM automation_runs WHERE automation_id IN (
            SELECT id FROM automations WHERE user_id = ANY(seed_ids)
        );
        DELETE FROM dispatch_logs WHERE user_id = ANY(seed_ids);
        DELETE FROM profiles WHERE id = ANY(seed_ids);
        RAISE NOTICE 'seed: % clientes anteriores removidos', array_length(seed_ids, 1);
    END IF;
END $$;

DO $$
DECLARE
    -- Argon2id de "Seed@2026" (gerado com: go run ./cmd/hashpw "Seed@2026").
    -- O login lê user_credentials.password_hash — profiles.password_hash NÃO
    -- é usado na autenticação.
    seed_hash TEXT := '$argon2id$v=19$m=65536,t=3,p=2$8VBcA7YuzfyNualGigUIVA$1ODH8JSWjjGccWofs8HsPYj37stlNm53n/W2DU/A0C8';

    uid   UUID;
    chip1 INT;
    chip2 INT;
    chip3 INT;
    lista UUID;
    a_search  UUID;
    a_list    UUID;
    a_monitor UUID;
    a_espelho UUID;
BEGIN

-- =============================================================================
-- 1) Maria Oliveira Souza — operação saudável, com um chip reserva caído.
--    O caso "está tudo bem, mas o cliente acha que não".
-- =============================================================================
INSERT INTO profiles (email, name, display_name, phone_number, plan, billing_status,
                      is_active, email_verified, email_verified_at,
                      max_instances, max_automations, max_groups, last_login_at)
VALUES ('maria.oliveira@seed.local', 'Maria Oliveira Souza', 'Maria', '+55 31 98888-7777',
        'pro', 'active', true, true, NOW(), 5, 20, 100, NOW() - INTERVAL '10 minutes')
RETURNING id INTO uid;
INSERT INTO user_credentials (user_id, password_hash) VALUES (uid, seed_hash);

INSERT INTO instances (user_id, instance_id, friendly_name, status, phone_number)
VALUES (uid, 'maria-chip-01', 'Chip Principal', 'connected', '5531988887777') RETURNING id INTO chip1;
INSERT INTO instances (user_id, instance_id, friendly_name, status, phone_number)
VALUES (uid, 'maria-chip-02', 'Chip Reserva', 'disconnected', '5531977776666') RETURNING id INTO chip2;

-- participating_instance_ids é int4[] e referencia instances.id (a PK numérica),
-- NÃO o instance_id textual.
INSERT INTO groups (user_id, instance_id, wa_group_id, name, is_admin, participating_instance_ids, admin_instance_ids)
VALUES (uid, chip1, '120363111111111111@g.us', 'Promoções Shopee BH', true, ARRAY[chip1, chip2], ARRAY[chip1]),
       (uid, chip1, '120363222222222222@g.us', 'Achadinhos ML', false, ARRAY[chip1], ARRAY[]::int[]),
       (uid, chip1, '120363333333333333@g.us', 'Cupons Relâmpago', true, ARRAY[chip1], ARRAY[chip1]);

INSERT INTO ml_link_lists (user_id, name, description, is_active)
VALUES (uid, 'Lista Black Friday', 'Links da campanha de novembro', true) RETURNING id INTO lista;
-- NB: a coluna é product_url (o cheat-sheet do CLAUDE.md diz "origin_url", que não existe).
INSERT INTO ml_link_list_items (list_id, user_id, product_url, consumed_at)
SELECT lista, uid, 'https://mercadolivre.com.br/p/MLB' || (1000 + g),
       CASE WHEN g <= 12 THEN NOW() - (g || ' minutes')::interval ELSE NULL END
FROM generate_series(1, 40) g;

INSERT INTO automations (user_id, name, type, status, interval_minutes, start_time, end_time,
                         instance_ids, send_groups, last_run_at, next_run_at)
VALUES (uid, 'Busca Shopee - Eletrônicos', 'search', 'active', 30, '08:00', '22:00',
        ARRAY[chip1], to_jsonb(ARRAY['120363111111111111@g.us']),
        NOW() - INTERVAL '40 seconds', NOW() + INTERVAL '29 minutes')
RETURNING id INTO a_search;

INSERT INTO automations (user_id, name, type, status, interval_minutes,
                         instance_ids, send_groups, ml_link_list_id, last_run_at, next_run_at)
VALUES (uid, 'Disparo Lista Black Friday', 'list', 'active', 15,
        ARRAY[chip1], to_jsonb(ARRAY['120363111111111111@g.us', '120363222222222222@g.us']),
        lista, NOW() - INTERVAL '3 minutes', NOW() + INTERVAL '12 minutes')
RETURNING id INTO a_list;

-- Monitoring/offer_generate sao event-driven: monitor_group_ids = grupos de
-- ORIGEM (escutados); send_groups = DESTINO. O painel mostra os dois.
INSERT INTO automations (user_id, name, type, status, instance_ids,
                         monitor_group_ids, send_groups)
VALUES (uid, 'Monitor Grupo Fonte', 'monitoring', 'active', ARRAY[chip1],
        to_jsonb(ARRAY['120363111111111111@g.us']), to_jsonb(ARRAY['120363222222222222@g.us']))
RETURNING id INTO a_monitor;

INSERT INTO automations (user_id, name, type, status, instance_ids,
                         monitor_group_ids, send_groups)
VALUES (uid, 'Espelho Ofertas Premium', 'offer_generate', 'active', ARRAY[chip1],
        to_jsonb(ARRAY['120363222222222222@g.us']), to_jsonb(ARRAY['120363333333333333@g.us']))
RETURNING id INTO a_espelho;

-- Pausada "limpa" (sem cobranca, sem erro): a unica ELEGIVEL para o religar
-- em massa do dashboard. As outras pausadas do seed (billing do Joao, credencial
-- da Fernanda) devem ser puladas.
INSERT INTO automations (user_id, name, type, status, interval_minutes,
                         instance_ids, send_groups)
VALUES (uid, 'Busca Pausada pelo Cliente', 'search', 'paused', 30, ARRAY[chip1],
        to_jsonb(ARRAY['120363111111111111@g.us']));

-- Run aberto: a busca aparece como "disparando" (sinal run_active).
INSERT INTO automation_runs (automation_id, status, started_at, lock_until, messages_sent)
VALUES (a_search, 'running', NOW() - INTERVAL '15 seconds', NOW() + INTERVAL '5 minutes', 6);

-- Envios recentes → alimentam a velocidade real (msgs/min medidos).
INSERT INTO dispatch_logs (user_id, automation_id, group_id, status, created_at)
SELECT uid, a_search, '120363111111111111@g.us', 'sent', NOW() - (g || ' seconds')::interval
FROM generate_series(3, 290, 10) g;

-- Monitor com envio agorinha → sinal recent_dispatch (event-driven ativo).
INSERT INTO dispatch_logs (user_id, automation_id, group_id, status, created_at)
SELECT uid, a_monitor, '120363222222222222@g.us', 'sent', NOW() - (g || ' seconds')::interval
FROM generate_series(8, 120, 35) g;

-- Espelho sem envio recente → sinal event_driven_idle ("aguardando origem").
INSERT INTO dispatch_logs (user_id, automation_id, group_id, status, created_at)
VALUES (uid, a_espelho, '120363333333333333@g.us', 'sent', NOW() - INTERVAL '48 minutes');

INSERT INTO dispatch_logs (user_id, automation_id, group_id, status, created_at)
SELECT uid, a_list, '120363222222222222@g.us', 'queued', NOW() - INTERVAL '20 minutes'
FROM generate_series(1, 4);

-- =============================================================================
-- 2) João Pedro Almeida — conta SUSPENSA por inadimplência.
--    Automações pausadas pelo billing: o painel deve deixar isso óbvio antes
--    do atendente procurar defeito técnico que não existe.
-- =============================================================================
INSERT INTO profiles (email, name, display_name, phone_number, plan, billing_status,
                      billing_suspension_kind, billing_suspended_at, billing_suspension_reason,
                      is_active, email_verified, email_verified_at,
                      max_instances, max_automations, max_groups, last_login_at)
VALUES ('joao.almeida@seed.local', 'João Pedro Almeida', 'João', '+55 11 97777-1234',
        'pro', 'suspended', 'payment_failed', NOW() - INTERVAL '3 days',
        'Pagamento recusado pela operadora — assinatura Kiwify cancelada',
        true, true, NOW(), 3, 10, 50, NOW() - INTERVAL '4 days')
RETURNING id INTO uid;
INSERT INTO user_credentials (user_id, password_hash) VALUES (uid, seed_hash);

INSERT INTO instances (user_id, instance_id, friendly_name, status, phone_number)
VALUES (uid, 'joao-chip-01', 'Chip Vendas', 'connected', '5511977771234') RETURNING id INTO chip1;

INSERT INTO groups (user_id, instance_id, wa_group_id, name, is_admin, participating_instance_ids, admin_instance_ids)
VALUES (uid, chip1, '120363444444444444@g.us', 'Ofertas SP', true, ARRAY[chip1], ARRAY[chip1]);

INSERT INTO automations (user_id, name, type, status, interval_minutes, instance_ids, send_groups,
                         paused_by_billing, last_run_at)
VALUES (uid, 'Busca Diária', 'search', 'paused', 60, ARRAY[chip1],
        to_jsonb(ARRAY['120363444444444444@g.us']), true, NOW() - INTERVAL '3 days');

-- =============================================================================
-- 3) Carla Mendes Ribeiro — TODOS os chips caídos, automações ativas.
--    O clássico "parou de enviar do nada": o painel deve mostrar automação
--    ativa + zero envio + chip desconectado, que é o diagnóstico inteiro.
-- =============================================================================
INSERT INTO profiles (email, name, display_name, phone_number, plan, billing_status,
                      is_active, email_verified, email_verified_at,
                      max_instances, max_automations, max_groups, last_login_at)
VALUES ('carla.ribeiro@seed.local', 'Carla Mendes Ribeiro', 'Carla', '+55 21 96666-4321',
        'elite', 'active', true, true, NOW(), 10, 50, 200, NOW() - INTERVAL '2 hours')
RETURNING id INTO uid;
INSERT INTO user_credentials (user_id, password_hash) VALUES (uid, seed_hash);

INSERT INTO instances (user_id, instance_id, friendly_name, status, phone_number)
VALUES (uid, 'carla-chip-01', 'Chip Rio', 'disconnected', '5521966664321') RETURNING id INTO chip1;
INSERT INTO instances (user_id, instance_id, friendly_name, status, phone_number)
VALUES (uid, 'carla-chip-02', 'Chip Niterói', 'disconnected', '5521955553210') RETURNING id INTO chip2;

INSERT INTO groups (user_id, instance_id, wa_group_id, name, is_admin, participating_instance_ids, admin_instance_ids)
VALUES (uid, chip1, '120363555555555555@g.us', 'Promo Rio de Janeiro', true, ARRAY[chip1, chip2], ARRAY[chip1]),
       (uid, chip1, '120363666666666666@g.us', 'Descontos Niterói', true, ARRAY[chip2], ARRAY[chip2]);

INSERT INTO automations (user_id, name, type, status, interval_minutes, instance_ids, send_groups,
                         last_run_at, next_run_at, last_error, last_error_at)
VALUES (uid, 'Busca Moda Feminina', 'search', 'active', 20, ARRAY[chip1],
        to_jsonb(ARRAY['120363555555555555@g.us']),
        NOW() - INTERVAL '3 hours', NOW() - INTERVAL '10 minutes',
        'instance carla-chip-01 is not connected', NOW() - INTERVAL '3 hours')
RETURNING id INTO a_search;

-- Espelho com a conta Shopee sem Open API (10035) — problema por-usuário, o
-- caso de atendimento Shopee mais comum. last_error_at recente prova que é atual.
INSERT INTO automations (user_id, name, type, status, instance_ids, send_groups,
                         last_error, last_error_at)
VALUES (uid, 'Espelho Grupo Parceiro', 'offer_generate', 'active', ARRAY[chip2],
        to_jsonb(ARRAY['120363666666666666@g.us']),
        'relay withheld: error code: 10035', NOW() - INTERVAL '90 seconds');

-- Falhas de 3h atrás e nada desde então: o silêncio é o sintoma.
INSERT INTO dispatch_logs (user_id, automation_id, group_id, status, error, created_at)
SELECT uid, a_search, '120363555555555555@g.us', 'failed',
       'instance carla-chip-01 is not connected', NOW() - INTERVAL '3 hours' - (g || ' minutes')::interval
FROM generate_series(1, 8) g;

-- =============================================================================
-- 4) Ricardo Souza Lima — onboarding incompleto: nenhum chip, nenhum grupo.
--    Estado vazio de verdade — a UI precisa aguentar isso sem quebrar.
-- =============================================================================
INSERT INTO profiles (email, name, display_name, phone_number, plan, billing_status,
                      is_active, email_verified,
                      max_instances, max_automations, max_groups)
VALUES ('ricardo.lima@seed.local', 'Ricardo Souza Lima', 'Ricardo', '+55 41 95555-8899',
        'free', 'active', true, false, 1, 5, 50)
RETURNING id INTO uid;
INSERT INTO user_credentials (user_id, password_hash) VALUES (uid, seed_hash);

-- =============================================================================
-- 5) Fernanda Costa Barbosa — volume alto e BACKLOG preso na fila.
--    Muitos itens em 'queued' há tempo: exercita o número "Na fila", que é
--    proposital sem recorte de janela (job preso há horas ainda é backlog vivo).
-- =============================================================================
INSERT INTO profiles (email, name, display_name, phone_number, plan, billing_status,
                      is_active, email_verified, email_verified_at,
                      max_instances, max_automations, max_groups, last_login_at)
VALUES ('fernanda.barbosa@seed.local', 'Fernanda Costa Barbosa', 'Fernanda', '+55 51 94444-2211',
        'elite', 'active', true, true, NOW(), 15, 80, 300, NOW() - INTERVAL '5 minutes')
RETURNING id INTO uid;
INSERT INTO user_credentials (user_id, password_hash) VALUES (uid, seed_hash);

INSERT INTO instances (user_id, instance_id, friendly_name, status, phone_number)
VALUES (uid, 'fernanda-chip-01', 'Chip POA 1', 'connected', '5551944442211') RETURNING id INTO chip1;
INSERT INTO instances (user_id, instance_id, friendly_name, status, phone_number)
VALUES (uid, 'fernanda-chip-02', 'Chip POA 2', 'connected', '5551933331100') RETURNING id INTO chip2;
INSERT INTO instances (user_id, instance_id, friendly_name, status, phone_number)
VALUES (uid, 'fernanda-chip-03', 'Chip Backup', 'pending', NULL) RETURNING id INTO chip3;

INSERT INTO groups (user_id, instance_id, wa_group_id, name, is_admin, participating_instance_ids, admin_instance_ids)
SELECT uid, chip1, '1203637777777777' || LPAD(g::text, 2, '0') || '@g.us',
       'Atacado POA ' || g, (g % 2 = 0), ARRAY[chip1, chip2], ARRAY[chip1]
FROM generate_series(1, 6) g;

INSERT INTO ml_link_lists (user_id, name, description, is_active)
VALUES (uid, 'Lista Atacado', 'Reposição semanal', true) RETURNING id INTO lista;
INSERT INTO ml_link_list_items (list_id, user_id, product_url, consumed_at)
SELECT lista, uid, 'https://mercadolivre.com.br/p/MLB' || (5000 + g),
       CASE WHEN g <= 180 THEN NOW() - (g || ' seconds')::interval ELSE NULL END
FROM generate_series(1, 200) g;

INSERT INTO automations (user_id, name, type, status, interval_minutes, instance_ids, send_groups,
                         ml_link_list_id, last_run_at, next_run_at)
VALUES (uid, 'Disparo Atacado (alto volume)', 'list', 'active', 5, ARRAY[chip1, chip2],
        to_jsonb(ARRAY['120363777777777701@g.us', '120363777777777702@g.us', '120363777777777703@g.us']),
        lista, NOW() - INTERVAL '90 seconds', NOW() + INTERVAL '3 minutes')
RETURNING id INTO a_list;

-- Invalid Signature auto-pausa a automação e grava last_error_at (é assim na
-- produção). last_error_at recente é o que faz o painel tratar como problema
-- ATUAL, sem depender das falhas ainda estarem dentro da janela de atividade.
INSERT INTO automations (user_id, name, type, status, instance_ids, send_groups,
                         last_error, last_error_at)
VALUES (uid, 'Espelho Fornecedor', 'offer_generate', 'paused', ARRAY[chip2],
        to_jsonb(ARRAY['120363777777777704@g.us']), 'Invalid Signature',
        NOW() - INTERVAL '2 minutes')
RETURNING id INTO a_espelho;

INSERT INTO automation_runs (automation_id, status, started_at, lock_until, messages_sent)
VALUES (a_list, 'running', NOW() - INTERVAL '45 seconds', NOW() + INTERVAL '4 minutes', 34);

INSERT INTO dispatch_logs (user_id, automation_id, group_id, status, created_at)
SELECT uid, a_list, '120363777777777701@g.us', 'sent', NOW() - (g || ' seconds')::interval
FROM generate_series(2, 300, 4) g;

INSERT INTO dispatch_logs (user_id, automation_id, group_id, status, error, created_at)
SELECT uid, a_espelho, '120363777777777704@g.us', 'failed', 'Invalid Signature',
       NOW() - (g || ' seconds')::interval
FROM generate_series(20, 260, 40) g;

-- Backlog: 120 jobs presos há mais de uma hora.
INSERT INTO dispatch_logs (user_id, automation_id, group_id, status, created_at)
SELECT uid, a_list, '120363777777777702@g.us', 'queued', NOW() - INTERVAL '75 minutes'
FROM generate_series(1, 120);

RAISE NOTICE 'seed: 5 clientes fake criados (senha: Seed@2026)';
END $$;

-- Resumo do que foi criado.
SELECT p.email, p.name, p.billing_status,
       (SELECT count(*) FROM instances i WHERE i.user_id = p.id) AS chips,
       (SELECT count(*) FROM instances i WHERE i.user_id = p.id AND i.status = 'connected') AS chips_on,
       (SELECT count(*) FROM automations a WHERE a.user_id = p.id) AS autos,
       (SELECT count(*) FROM groups g WHERE g.user_id = p.id) AS grupos,
       (SELECT count(*) FROM ml_link_lists l WHERE l.user_id = p.id) AS listas,
       (SELECT count(*) FROM dispatch_logs d WHERE d.user_id = p.id AND d.status = 'queued') AS na_fila
FROM profiles p
WHERE p.email LIKE '%@seed.local'
ORDER BY p.email;
