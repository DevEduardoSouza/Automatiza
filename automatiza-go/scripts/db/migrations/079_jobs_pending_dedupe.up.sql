-- automatiza:migration-mode=transactional
--
-- Melhoria 8 — contencao P0: impedir que o mesmo agendamento vire N envios.
--
-- O QUE ESTA SENDO CORRIGIDO
-- O modal de agendamento cria 1 job por (ocorrencia, grupo, acao) num loop de
-- N RPCs sequenciais. O botao NAO ficava travado durante o loop
-- (schedule-multiple-groups.tsx: `isCreatingJob` so cobria a mutation unaria),
-- entao um segundo clique disparava a rajada inteira de novo. Como a tabela
-- `jobs` nao tinha NENHUM UNIQUE alem da PK, as copias entravam todas e cada
-- uma virava um envio real: em 23/07/2026 um unico usuario tinha 144 grupos de
-- jobs pending identicos (1.849 linhas, ate 19 copias do MESMO horario) —
-- 19 mensagens iguais no mesmo grupo, no mesmo minuto.
--
-- POR QUE A CHAVE E LOGICA (conteudo) E NAO UM TOKEN DO CLIENTE
-- Um `client_token` no request so protegeria clientes com o bundle novo, e o
-- Cloudflare serve o index.html antigo por ~1h. Derivar a chave do proprio
-- conteudo (usuario + acao + horario + payload) protege TAMBEM as abas que
-- ainda rodam o frontend antigo — que sao exatamente as que causam o problema.
--
-- POR QUE `type` E NAO `action_type`
-- As duas colunas existem e sao SEMPRE gravadas com o mesmo valor
-- (job_repository.go: `job.ActionType` alimenta as duas). `type` e NOT NULL;
-- `action_type` e nullable, e em indice unico NULL nao colide com NULL — uma
-- linha com action_type NULL escaparia do indice silenciosamente.
--
-- POR QUE PARCIAL EM status='pending'
-- Recorrencia legitima cria horarios DISTINTOS (verificado: o caso de 27 jobs
-- reportado como "27 mensagens" tinha 27 `scheduled_for` diferentes — expansao
-- futura, nao duplicata). So conteudo+horario+acao identicos com job ainda
-- pending sao bloqueados. Job ja `done`/`failed` sai do indice, entao reagendar
-- o mesmo conteudo para um horario que ja passou continua possivel, e o indice
-- fica pequeno (indexa so a fila viva).
--
-- md5(payload::text): jsonb::text e canonico (ordem de chaves e espacos
-- normalizados pelo Postgres), entao {"a":1,"b":2} e {"b":2,"a":1} produzem a
-- MESMA chave. Verificado empiricamente antes de escrever esta migracao.

-- (a) Dedupe defensivo COM backup: o indice unico nao pode ser criado enquanto
-- existirem duplicatas. Mantem min(id) por grupo (deterministico e idempotente)
-- — a intencao do usuario e preservada, some so a repeticao.
CREATE TABLE IF NOT EXISTS _bkp_jobs_dedup_079 AS
SELECT a.*
FROM jobs a
WHERE a.status = 'pending'
  AND EXISTS (
    SELECT 1 FROM jobs b
    WHERE b.status = 'pending'
      AND b.user_id = a.user_id
      AND b.type = a.type
      AND b.scheduled_for = a.scheduled_for
      AND md5(b.payload::text) = md5(a.payload::text)
      AND b.id < a.id
  );

DELETE FROM jobs a
WHERE a.status = 'pending'
  AND EXISTS (
    SELECT 1 FROM jobs b
    WHERE b.status = 'pending'
      AND b.user_id = a.user_id
      AND b.type = a.type
      AND b.scheduled_for = a.scheduled_for
      AND md5(b.payload::text) = md5(a.payload::text)
      AND b.id < a.id
  );

-- (b) A garantia propriamente dita. Tabela com ~5k linhas: o lock de escrita
-- dura milissegundos, entao CONCURRENTLY (que exigiria migracao
-- nao-transacional + verifier) seria custo sem beneficio.
CREATE UNIQUE INDEX IF NOT EXISTS uq_jobs_pending_logical
    ON jobs (user_id, type, scheduled_for, md5(payload::text))
    WHERE status = 'pending';
