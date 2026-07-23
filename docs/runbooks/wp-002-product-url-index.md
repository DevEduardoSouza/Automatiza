# WP-002 — migration zumbi de `product_url`

## Estado esperado

O estado normal é a ausência de
`public.idx_dispatch_logs_product_url_trgm`. A extensão `pg_trgm` não faz parte
do contract deste pacote e não deve ser criada ou removida por este runbook.

O deploy automático apenas executa migrations com modo explícito. Uma migration
não transacional exige verificador; falha, estado parcial, checksum divergente
ou execução concorrente bloqueiam o deploy e exigem intervenção. Não há retry
automático.

A quarentena automática reconhece o nome exato
`idx_dispatch_logs_product_url_trgm`. Um GIN semanticamente equivalente sob
outro nome ainda exige modo não transacional e verifier, mas depende de revisão
humana; não trate a blocklist nominal como detector semântico.

Bootstrap de banco vazio está bloqueado pela cadeia legada e é rastreado em
`docs/blockers/WP-002-bootstrap-zero.md`. Enquanto aberto, ambiente novo deve
ser criado por restore validado de snapshot que preserve o ledger.

## Evidência de origem

Nos 19 runs de deploy inspecionados, de `29499064152` a `29595057969`, a
migration 019 criou o GIN de `product_url` em aproximadamente 40–56 segundos e
a migration 037 o removeu no mesmo deploy. O DDL não falhou nesses runs; o
runner antigo repetia todos os arquivos e mascarava 31 erros SQL legados não
relacionados com `ON_ERROR_STOP=0`.

## Human gates

Nenhum DDL deste runbook é automático. O operador deve parar antes de produção
se qualquer item estiver ausente:

- staging restaurado de snapshot N-1 e validado;
- evidência recente de backup e restore do WP-001;
- owner de banco e janela aprovados no change record;
- baseline de 15 minutos para latência p95, taxa de erro, I/O e lock waits;
- espaço livre conhecido. Para a criação de recuperação, reservar no mínimo
  duas vezes o tamanho estimado do índice mais 20% de margem;
- conexão `psql` aprovada, sem credenciais na linha de comando ou no log;
- uma única sessão de operador para DDL.

## 1. Diagnóstico somente leitura

Execute a partir de um checkout do mesmo commit, usando uma conexão já
autenticada por flags seguras, `PGSERVICE` ou variáveis protegidas:

```sh
psql -X -v ON_ERROR_STOP=1 \
  -f automatiza-go/scripts/db/operations/wp-002/diagnose-product-url-index.sql
```

Salve a saída sanitizada no change record. Antes do primeiro rollout,
`_deploy_migration_attempts` pode aparecer como ainda não inicializada; depois
do rollout isso é blocker. Não prossiga se `_deploy_migrations` não existir, se
019/037 não constarem aplicadas, se o schema não for `public`, ou se a definição
encontrada divergir do SQL operacional.

Interpretação:

- nenhuma linha para o índice: estado final esperado; não execute DDL;
- `indisvalid=true`: pare. Este pacote não autoriza remover um índice válido;
- `indisvalid=false`: é elegível apenas para a mudança separada de remoção;
- linha em `pg_stat_progress_create_index`: existe DDL concorrente em curso;
  não inicie outro.

## 2. Deploy do congelamento

Primeiro faça um deploy canário em staging. O log deve conter
`event=migration_skipped` para 019 e 037, o mesmo `trace_id` do run e nenhum
`migration_started` para esses arquivos. Rode o diagnóstico novamente e
confirme que o índice continua ausente.

Promova um único deploy de produção somente após aprovação. Observe por 15
minutos. O critério principal é zero execução de
`CREATE INDEX ... idx_dispatch_logs_product_url_trgm` e zero aumento de I/O
atribuível a esse DDL.

## 3. Remoção separada de índice inválido

Esta etapa requer outro change record e outra aprovação. O script recusa índice
ausente, válido ou com definição inesperada.

```sh
psql -X -v ON_ERROR_STOP=1 \
  -v wp002_confirm=DROP_INVALID_PRODUCT_URL_INDEX \
  -f automatiza-go/scripts/db/operations/wp-002/drop-invalid-product-url-index.sql
```

O comando usa `DROP INDEX CONCURRENTLY`, fora de transação, com
`lock_timeout=5s`. Timeout ou falha encerram a tentativa; não repita sem novo
diagnóstico e nova decisão do operador.

## 4. Criação somente para recuperação excepcional

Este não é o rollout normal. Use apenas se uma análise de workload aprovada
decidir restaurar o índice. Exige change record próprio, staging, capacidade,
janela e aprovação. O script não instala `pg_trgm`.

Em outra sessão, monitore:

```sql
SELECT pid, phase, lockers_total, lockers_done, blocks_total, blocks_done,
       tuples_total, tuples_done
FROM pg_stat_progress_create_index
WHERE relid = to_regclass('public.dispatch_logs');
```

Na sessão de DDL:

```sh
psql -X -v ON_ERROR_STOP=1 \
  -v wp002_confirm=CREATE_PRODUCT_URL_INDEX_RECOVERY \
  -f automatiza-go/scripts/db/operations/wp-002/create-product-url-index-recovery.sql
```

O script usa `CREATE INDEX CONCURRENTLY`, `lock_timeout=5s` e
`statement_timeout=2h`, e só termina com `indisvalid=true`, `indisready=true` e
definição exata. Se a conexão cair ou a pós-validação falhar, o estado é
desconhecido: não repita. Volte ao diagnóstico; um índice inválido só pode ser
removido pela etapa separada.

## Abort thresholds

Aborte/cancele a operação e preserve evidências quando ocorrer qualquer sinal:

- lock wait causado pelo DDL acima de 5 segundos;
- latência p95 ou taxa de erro 20% acima da baseline por 5 minutos;
- latência de I/O 20% acima da baseline por 5 minutos;
- espaço livre abaixo da reserva calculada;
- progresso sem avanço por 10 minutos;
- `error_code=migration_*`, divergência de checksum ou outro deploy tentando
  adquirir o advisory lock.

Cancelamento de uma criação concorrente pode deixar índice inválido. Trate-o
como estado parcial; não faça retry e não use `DROP INDEX` bloqueante.

## Rollback

O congelamento é compatível com N-1 porque não contrai schema nem remove
extensão. Em falha do deploy, restaure a imagem anterior; mantenha o ledger e o
schema expandido. Não reverta 019 para a versão que recriava o GIN.

Rollback lógico do DDL:

- remoção concluída: o estado desejado já é índice ausente;
- criação excepcional concluída: removê-la exige novo change record usando o
  mesmo fluxo concorrente;
- falha/timeout: diagnosticar `indisvalid`/`indisready`, registrar `trace_id` e
  exigir intervenção.

## Evidências de encerramento

- logs sanitizados do runner e `trace_id`;
- saída before/after do diagnóstico;
- captura de `pg_stat_progress_create_index` quando houve DDL;
- baseline e 15 minutos de observação;
- aprovação do operador e auditor independente;
- comprovação de que nenhum deploy futuro iniciou o DDL aposentado.
