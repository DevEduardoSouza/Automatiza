# ADR 0002 — Backup, PITR e recuperação de sessões

- Status: proposto para canário em staging
- Data: 2026-07-23
- Pacote: WP-001

## Contexto

O Compose confirmado usa PostgreSQL 16 no volume `docker_postgres_data`, WAHA shard 1 em `docker_waha_data`, WAHA shard 2 em `docker_waha2_data` e mídia WAHA em S3. Os scripts anteriores faziam somente `pg_dump`, copiavam apenas o primeiro shard e apagavam objetos por idade no próprio job. Não havia WAL/PITR, checksum remoto, marcador de conclusão, restore automatizado, alerta por shard ou inode.

Um dump válido não prova que as sessões WAHA são recuperáveis. Da mesma forma, objetos no bucket operacional não são uma cópia fora da conta nem uma evidência de restore.

## Decisão

1. PostgreSQL terá duas camadas:
   - dump lógico diário, criptografado com `age`, para portabilidade e inspeção por `pg_restore`;
   - backup físico pgBackRest com WAL contínuo em repositório fora da VPS e, para produção, fora da conta operacional.
2. Os dois volumes WAHA e a allowlist de configuração da VPS serão artefatos independentes, criptografados, imutáveis e identificados por UUID. Cada artefato recebe manifesto SHA-256 e marcador `.COMPLETED`; não existe `sync`, overwrite ou delete no produtor.
3. Retenção, Object Lock/versionamento e expiração serão políticas humanas no destino. `expire-auto=n` impede que o backup pgBackRest execute expiração implicitamente; nenhum produtor deste pacote apaga objetos.
4. A chave privada `age` fica fora da VPS de origem. O pgBackRest precisa da passphrase de cifra e de `List/Get/Put` no host de origem para manter/verificar o repositório; essa co-localização é um risco de confidencialidade aceito, não equivalência ao desenho `age`. As compensações obrigatórias são arquivo `0600`, role sem `DeleteObject`, bucket em conta separada, Object Lock/versionamento e escrow independente da passphrase.
5. Restore é aceito somente em host isolado com `BACKUP_MODE=staging`, sentinel exato e destino vazio. Os Compose de restore não expõem portas e usam projeto/volume separados.
6. Métricas têm labels limitadas a componentes, mounts enumerados, prefixes enumerados e uma allowlist de tabelas. Logs estruturados incluem `event` e `error_code`, sem nomes de sessão, tenant, objeto ou conteúdo.

## RPO e RTO

| Componente | RPO | RTO | Evidência exigida |
|---|---:|---:|---|
| PostgreSQL | 5 min | 60 min | WAL ≤ 300 s e PITR em volume temporário |
| Sessões WAHA, por shard | 6 h | 4 h | restore dos dois volumes e conexão de uma sessão descartável |
| Objetos S3 essenciais | 24 h | 4 h | inventário e restore da réplica fora da conta |
| Configuração da VPS | 24 h | 2 h | archive allowlisted restaurado sem chaves privadas |

## Sequência expandir → migrar → observar → contrair

- Expandir: instalar scripts desabilitados, destino imutável, métricas, alertas e imagem pgBackRest; manter dumps legados sem removê-los.
- Migrar: inicializar stanza, confirmar primeiro full/WAL, executar backups dos dois shards e configurar cópia S3 fora da conta.
- Observar: sete dias de frescor/checksum/capacidade e ao menos um restore lógico, um PITR e um restore de sessão em staging.
- Contrair: somente outro change record pode retirar crons legados ou retenções antigas, depois das evidências e aprovação humana.

## Consequências e limites

O custo de armazenamento e leitura diária aumenta. O checksum remoto relê o artefato mais recente. Backup vivo do diretório WAHA é apenas crash-consistent; até existir snapshot de filesystem/CSI ou mecanismo oficial de quiesce validado, o restore de sessão continua um gate obrigatório e não pode ser declarado garantido.

O compromisso integral da VPS pode capturar a passphrase pgBackRest em uso. Object Lock e a ausência de permissão de delete protegem contra destruição do repositório, mas não contra leitura: o funcionamento escolhido exige `List/Get`. Restore administrativo usa identidade separada, mas isso não elimina o risco no produtor. Dois custodiantes humanos independentes mantêm cópias do material de restore em cofres distintos e testam trimestralmente a recuperação em staging.

Este PR não habilita WAL, não inicializa repositório, não configura replicação/lifecycle S3, não instala units e não executa restore. Essas ações pertencem ao operador humano.
