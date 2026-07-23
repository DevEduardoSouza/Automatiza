# Blocker: WP-002 bootstrap-zero da cadeia legada

- Status: Open
- Owner: database/migrations owner — pending assignment
- Opened: 2026-07-23
- Blocks: banco novo criado somente pela cadeia de migrations
- Does not block: restore de snapshot/backup que preserve `_deploy_migrations`

## Evidence

O runner fail-closed rejeita uma migration pendente sem
`automatiza:migration-mode`. Em banco vazio, `000_postgresql_init.sql` é a
primeira migration e não possui marker. Antes desse guard, a execução estrita
da cadeia histórica também falhou em `000_postgresql_init.sql:1024`.

O compose local monta o mesmo runner em
`/docker-entrypoint-initdb.d/02-run-migrations.sh`; portanto, onboarding a
partir de volume PostgreSQL vazio não é suportado enquanto este blocker estiver
aberto.

## Impact

- bootstrap zero de desenvolvimento e disaster recovery por migrations fica
  bloqueado;
- restore de snapshot/backup com ledger permanece o caminho suportado;
- não é seguro liberar migrations legadas em modo permissivo para contornar o
  bloqueio.

## Next safe action

Abrir WP próprio para classificar e testar toda a cadeia legada, corrigir a
falha histórica, adicionar markers/verifiers e provar:

1. bootstrap PostgreSQL 16 do zero;
2. upgrade sobre snapshot N-1;
3. restart após falha sem repetição de DDL;
4. compatibilidade com imagem N-1;
5. restore/login/smoke test em staging.

Até esse WP ser concluído, ambientes novos devem nascer por restore validado de
snapshot, nunca relaxando `ON_ERROR_STOP` ou o bloqueio do runner.
