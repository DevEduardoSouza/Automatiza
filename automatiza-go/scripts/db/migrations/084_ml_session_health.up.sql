-- automatiza:migration-mode=transactional
-- Saúde da sessão do Mercado Livre.
--
-- MOTIVAÇÃO (medido em 30/07/2026): as 7 credenciais ML em produção estavam
-- paradas há 111-144 dias (updated_at entre 2026-03-08 e 2026-04-10) e geraram
-- ZERO links em 7 dias. Ninguém — nem o cliente, nem nós — foi avisado, porque
-- não existia nenhuma verificação de sessão no sistema. Estas colunas são o
-- estado que o health check escreve e o painel lê.
--
-- ADITIVA: só acrescenta colunas nullable/com default a uma tabela de 7 linhas.
-- Nenhum DROP, nenhum ALTER destrutivo, nenhuma reescrita. Em PostgreSQL 11+,
-- ADD COLUMN com DEFAULT constante não reescreve a tabela (grava só o catálogo),
-- então é instantâneo mesmo que a tabela cresça. Código antigo que não conhece
-- estas colunas continua funcionando: o SELECT do repositório é explícito.
--
-- Por que NÃO uma tabela nova: ml_credentials já tem UNIQUE(user_id) e é
-- exatamente a granularidade do estado (uma sessão por usuário). Mesmo padrão
-- da 073_instagram_stories: expiry/estado em CLARO ao lado do blob cifrado,
-- para o cron listar quem precisa de checagem sem decifrar nada.

-- Teto curto de espera por lock (exigência do guardrail GOV-SQL-003).
-- ADD COLUMN pega ACCESS EXCLUSIVE, mas a tabela é minúscula e raramente
-- escrita; o teto é rede de segurança contra uma transação longa em curso.
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Estado da última verificação. 'unknown' até o primeiro check rodar.
--   unknown — nunca verificada (ou verificação indeterminada: rede/WAF/5xx)
--   live    — o painel de afiliados respondeu autenticado
--   expired — o ML respondeu 401/403 de forma determinística
--   blocked — 429/desafio de WAF: NÃO é sessão morta, não pedir reconexão
ALTER TABLE ml_credentials
    ADD COLUMN IF NOT EXISTS session_status TEXT NOT NULL DEFAULT 'unknown';

-- Quando a sessão foi verificada pela última vez. NULL = nunca.
-- É por esta coluna que o cron ordena para dar a vez a quem está mais atrasado.
ALTER TABLE ml_credentials
    ADD COLUMN IF NOT EXISTS last_verified_at TIMESTAMPTZ;

-- Etiqueta de afiliado descoberta automaticamente no painel do ML. Fica em
-- claro de propósito: não é segredo (aparece em toda URL de afiliado gerada) e
-- o painel precisa exibi-la sem decifrar o blob.
ALTER TABLE ml_credentials
    ADD COLUMN IF NOT EXISTS discovered_tag TEXT NOT NULL DEFAULT '';

-- Como a credencial entrou: 'manual' (token colado pelo cliente) ou
-- 'extension' (capturada pela extensão de navegador, fase 2). Serve para medir
-- a adoção da extensão antes de decidir o rollout.
ALTER TABLE ml_credentials
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'manual';

-- O índice da fila do health check vive na 085: o guardrail GOV-SQL-002 exige
-- CREATE INDEX CONCURRENTLY, que não roda dentro de transação — e esta
-- migração é transactional por causa dos ALTERs acima.
