# Runbook — backup, PITR e restore de sessões

## Escopo e proibições

Este runbook prepara WP-001. Todos os comandos que alteram staging ou produção abaixo são para operador humano em janela aprovada; não foram executados pelo agente. Nunca combinar os Compose de restore com `docker/docker-compose.prod.yml`. Nunca reutilizar volume de restore. Nunca copiar para tickets/logs `.env`, dumps, chaves, cookies, tokens ou dados de clientes.

## Diagnóstico confirmado

- PostgreSQL: serviço `postgres`, container `automatiza-postgres`, imagem declarada `postgres:16-alpine`, volume `docker_postgres_data`.
- WAHA shard 1: container `automatiza-waha`, volume `docker_waha_data`, montado em `/app/.sessions`.
- WAHA shard 2: container `automatiza-waha-2`, volume `docker_waha2_data`, montado em `/app/.sessions`.
- Mídia WAHA: S3 configurado por ambiente; bucket, conta, versionamento e Object Lock reais não foram lidos.
- Migrações: `automatiza-go/scripts/db/migrations/`; o serviço `migrate` executa `init.sql` e `run-migrations.sh`.
- Scripts antigos: dump lógico, somente primeiro shard, retenção por delete no job e falhas sem estado verificável.
- Observabilidade: há collector Prometheus, mas o repositório não confirma node_exporter/textfile collector nem o carregamento de `observability/backup-alerts.yml`. Como ponte, `ops/waha-fix/monitor.py` lê os estados do controlador e usa o backend de e-mail já existente quando o sentinel de backup está habilitado.

## Pré-requisitos e gates

O operador registra no change record:

- digest exato da imagem PostgreSQL atualmente aprovada;
- destino em conta separada, versionamento/Object Lock e política de retenção;
- capacidade e custo aprovados, com `BACKUP_DESTINATION_BUDGET_BYTES`;
- role pgBackRest de mínimo privilégio (`List/Get/Put`, sem `DeleteObject`), role de restore administrativa separada e acesso auditado;
- chave privada `age` e cópia da passphrase pgBackRest mantidas fora da VPS de origem;
- snapshot consistente dos volumes WAHA ou janela/quiesce oficialmente validado;
- host de restore isolado com espaço para artefato cifrado, dump e volume restaurado;
- dono do alerta e integração do arquivo de regras.
- dois custodiantes nomeados: Custodiante A mantém o material em cofre corporativo; Custodiante B mantém uma cópia em cofre/conta independente. Cada trimestre, ambos autorizam um restore em staging e registram fingerprint, data e resultado sem expor a chave.

Pare se qualquer item faltar, se o schema/volume divergir ou se o destino não for comprovadamente separado e imutável.

## Preparação de staging

1. Revisar `ops/backup/config.example` e criar `/etc/automatiza-backup/config.env` com modo `staging`, permissões `0600` e sem levar valores ao repositório.
2. Instalar `backupctl.py` em `/opt/automatiza/backup/`, a allowlist aprovada em `/etc/automatiza-backup/config-allowlist.txt` e somente a chave pública em `age-recipient.txt`. Instalar versões aprovadas de `age` e `rclone`; o preflight apenas detecta dependências, não instala pacotes.
3. Fixar `POSTGRES_BACKUP_BASE_IMAGE` como `postgres@sha256:...`; não usar tag móvel em produção.
4. Criar a configuração pgBackRest a partir do exemplo. Preferir identidade/role da instância; se houver segredo, manter o arquivo `0600` fora do Git. A passphrase precisa estar disponível ao produtor e, portanto, deve ter escrow separado. A role do produtor não recebe `DeleteObject`; `expire-auto=n` mantém expiração/lifecycle sob gate humano.
5. Criar o role local `automatiza_backup` a partir de `ops/backup/create-readonly-role.sql.example`, após revisão humana. Ele é read-only, sem senha e limitado ao socket local via `docker exec`.
6. Validar sem mutação:

```sh
python3 /opt/automatiza/backup/backupctl.py --config /etc/automatiza-backup/config.env preflight
docker compose -f docker/docker-compose.prod.yml -f docker/docker-compose.backup.yml config
```

7. O operador cria `/etc/automatiza-backup/ENABLED` contendo exatamente `AUTOMATIZA_BACKUP_ENABLED_V1` somente depois da aprovação.

## Ativação pgBackRest em staging

O rollout tem duas recriações deliberadas. A primeira instala a imagem pgBackRest com archiving desligado; a stanza é criada e validada antes da segunda recriação habilitar `archive_command`. `BACKUP_PITR_ENABLED` permanece `false` até a validação final, evitando falso crítico antes do canário.

```sh
export PGBACKREST_ENABLED=false
docker compose -f docker/docker-compose.prod.yml -f docker/docker-compose.backup.yml build postgres
docker compose -f docker/docker-compose.prod.yml -f docker/docker-compose.backup.yml up -d postgres
docker exec automatiza-postgres pgbackrest --stanza=automatiza stanza-create

# Gate humano: confirmar stanza criada antes de habilitar archive_mode.
export PGBACKREST_ENABLED=true
docker compose -f docker/docker-compose.prod.yml -f docker/docker-compose.backup.yml up -d postgres
docker exec automatiza-postgres psql -U automatiza -d automatiza -tA -c "SHOW archive_mode"
docker exec automatiza-postgres pgbackrest --stanza=automatiza check

# Só agora alterar BACKUP_PITR_ENABLED=true no config.env.
python3 /opt/automatiza/backup/backupctl.py --config /etc/automatiza-backup/config.env backup postgres-full --execute
du -sb /var/lib/docker/volumes/docker_postgres_data/_data/pg_wal
```

Abortar se PostgreSQL não ficar healthy no limite normal, `archive_mode` não retornar `on`, `pg_stat_archiver.failed_count` crescer, último WAL exceder 300 segundos, `pg_wal` atingir 10 GiB/crescer sem estabilizar, disco/I/O ultrapassar orçamento ou qualquer backup falhar. Em 20 GiB de `pg_wal`, rollback imediato. Retirar o overlay e recriar somente `postgres` com a imagem anterior; não apagar repositório nem volume. A configuração expandida permanece compatível.

## Backups e verificação

Primeiro executar em dry-run (sem `--execute`), depois um componente por vez:

```sh
python3 /opt/automatiza/backup/backupctl.py --config /etc/automatiza-backup/config.env backup postgres-logical
python3 /opt/automatiza/backup/backupctl.py --config /etc/automatiza-backup/config.env backup waha-1
python3 /opt/automatiza/backup/backupctl.py --config /etc/automatiza-backup/config.env backup waha-2
```

Após aprovação, repetir com `--execute`. Verificar:

```sh
python3 /opt/automatiza/backup/backupctl.py --config /etc/automatiza-backup/config.env verify
python3 /opt/automatiza/backup/backupctl.py --config /etc/automatiza-backup/config.env verify --remote
python3 /opt/automatiza/backup/backupctl.py --config /etc/automatiza-backup/config.env inventory
```

`verify --remote` relê o último artefato cifrado, compara tamanho/SHA-256 com manifesto e marker e executa `pgbackrest verify` no repositório físico. `inventory` mede orçamento do destino, prefixes S3 allowlisted e crescimento diário das tabelas allowlisted. Um exit code `2` significa alerta; `1`, erro operacional/configuração.

As units/timers são instaladas apenas depois do canário. O verificador local roda a cada cinco minutos; checksum remoto e inventário, diariamente. Com `BACKUP_PITR_ENABLED=false`, WAL/full/diff são reportados como ainda não habilitados e não geram falso crítico. Aponte `BACKUP_METRICS_DIR` para o textfile collector e carregue `observability/backup-alerts.yml`. Até esse wiring ser confirmado, implantar também o `monitor.py` atualizado: ele pagina por e-mail frescor por shard, verificação remota, inventário e error codes. Testar o e-mail é gate de rollout.

## Restore lógico e de sessões

No host isolado, criar `/etc/automatiza-backup/RESTORE_ALLOWED` com `AUTOMATIZA_STAGING_RESTORE_V1`, instalar a chave privada com `0600` e selecionar artefatos já verificados. Fixar `POSTGRES_RESTORE_IMAGE` por digest. O script recusa Compose de produção, serviço inesperado, projeto ou volume já existente. Dump cifrado, plaintext e cópia em `/tmp` do container são removidos por padrão; `RESTORE_KEEP_PLAINTEXT=true` exige aprovação explícita e storage cifrado:

```sh
export POSTGRES_LOGICAL_ARTIFACT='postgres-logical/YYYY/MM/DD/arquivo.dump.age'
export WAHA1_ARTIFACT='waha-1/YYYY/MM/DD/arquivo.tar.age'
export WAHA2_ARTIFACT='waha-2/YYYY/MM/DD/arquivo.tar.age'
export POSTGRES_RESTORE_IMAGE='postgres@sha256:...'
/opt/automatiza/backup/restore-drill.sh
```

O restore recusa modo diferente de staging, sentinel incorreto, diretório não vazio, caminho protegido, archive traversal, link/dispositivo e checksum divergente. O banco temporário deve estar vazio. O smoke exige ao menos uma linha em `profiles` e `instances` por padrão (`RESTORE_MIN_*`); os dois archives WAHA devem conter arquivos.

Em seguida, o operador humano:

1. confirma o UID/GID da imagem WAHA aprovada e ajusta ownership apenas nos diretórios temporários restaurados;
2. executa as migrações da versão candidata contra o banco temporário;
3. inicia aplicação temporária sem workers/crons e confirma `/api/v1/health`;
4. usa conta e sessão descartáveis de teste, confirma login e conexão WAHA sem envio real;
5. registra timestamps, contagens agregadas, duração e evidência sem PII;
6. somente após revisão remove o projeto/volume temporário explicitamente.

## Drill PITR

Use `ops/backup/restore-pitr.sh` em host isolado. O identificador cria um projeto/volume novo e o script recusa volume já existente.

```sh
export PITR_DRILL_ID='20260723a'
export PITR_TARGET='2026-07-23 03:00:00+00'
export PGBACKREST_RESTORE_IMAGE='automatiza-postgres-pgbackrest@sha256:...'
/opt/automatiza/backup/restore-pitr.sh
```

Confirmar no pgBackRest `info` que o alvo está coberto por full/diff e WAL antes do comando. Comparar o horário efetivo de recovery com o alvo e provar RPO ≤ 5 min e RTO ≤ 60 min. Não promover esse banco para produção; o procedimento de disaster recovery real exige change record próprio.

## S3 essencial

Os prefixes essenciais devem incluir mídia WAHA e artefatos necessários ao negócio, definidos pelo data owner. `inventory` mede-os, mas não cria segunda cópia. O operador configura replicação/versionamento/Object Lock para conta separada, bloqueia delete na role de origem e realiza restore amostral da réplica. Sem essa evidência, RPO/RTO S3 permanece blocker.

## Alertas e abort thresholds

- WAL: warning > 300 s; crítico/indisponível > 600 s.
- `pg_wal`: warning em 10 GiB; crítico em 20 GiB; calibrar após baseline sem elevar os defaults durante o canário.
- WAHA por shard: crítico sem sucesso por 8 h (RPO de 6 h mais margem operacional).
- PostgreSQL lógico/config: crítico após 26 h; diff 26 h; full 8 dias.
- Disco e inode: warning 70%, warning de prioridade alta 80%, critical 90%.
- Destino: warning 70%, warning de prioridade alta 80%, critical 90% do orçamento.
- Abortar rollout por checksum inválido, failed WAL, falta de página, espaço abaixo do dobro do maior artefato esperado, sessão descartável irrecuperável ou RTO/RPO excedido.

## Rollback

Desabilitar timers novos e remover o sentinel interrompe novas escritas. Restaurar a imagem PostgreSQL anterior retirando o overlay; a base continua legível porque não há migration de schema. Manter objetos e stanza intactos para investigação. Não reativar delete/retention legado no rollback. Cleanup de projetos/volumes de restore é manual, com alvo exato e aprovação.

## Incidentes e riscos pré-existentes

- O shard `waha-2` sem backup e o crescimento anormal do dump são blockers urgentes para o operador medir antes do dimensionamento.
- Qualquer Access Key encontrada em log deve ser tratada como comprometida: abrir incidente, rotacionar a credencial, restringir permissões do log e validar redaction/logrotate. Este pacote não executa a rotação.
- Arquivos de credenciais soltos fora do Git devem ser movidos pelo responsável de segurança para cofre aprovado; agentes não devem lê-los.
- As units usam root e Docker socket RW. Restringir instalação aos arquivos deste pacote, auditar checksums e não reutilizar a conta para outras automações.
