# Instagram Stories — Fase 0 local: endurecimento e validação

**Status:** Fase 0 implementada e validada localmente; canal real ainda não validado e permanece desligado, aguardando app Meta de teste e credenciais fornecidas pelo usuário.

Nenhum commit, push, deploy ou mutação de produção foi feito nesta entrega.

---

## 1. Baseline

| Item | Valor |
|---|---|
| Commit base | `aba9244fa41df200c34dc94a08e8540f9b9bca1b` (main, já em produção) |
| Worktree isolado | `.worktrees/m3-instagram-fase0` (detached) |
| Origem da integração | `b9f379c` (OAuth, publicação, worker, trigger `{story}`) |
| Data | 23/07/2026 |

**A main avançou durante a execução.** Ao final desta entrega o `main` local estava em `ca00d2d`, três commits à frente da base (`94520aa` fix de deploy, `f1eb2d4` e `ca00d2d` da Fase B de membros de grupo). Destes, apenas `ca00d2d` toca arquivos em comum com esta entrega — `cmd/server/main.go` (+3 linhas) e `internal/repository/interfaces.go` (+5 linhas), ambos aditivos e em regiões distintas. A integração deve ser um rebase simples, mas precisa ser refeita e revalidada antes de qualquer commit.

**Desvio consciente do plano.** O plano fixava `f50bb546` como base. Entre a escrita do plano e a execução, `aba9244` (migração 076, tracking de membros) entrou na main e foi para produção, tocando seis arquivos que esta entrega também toca (`interfaces.go`, `repositories.go`, `main.go`, `config.go`, `asynq.go`, `waha_adapter.go`). Basear em `f50bb54` produziria conflito garantido na integração. A base foi trocada para `aba9244` com aprovação explícita do usuário; o isolamento em worktree e a numeração 077 como próxima migração — as duas razões declaradas do plano — seguem valendo.

---

## 2. Defeitos corrigidos

### Críticos

1. **Envelopes OAuth incompatíveis com o contrato oficial.** `ExchangeCode` e `GetMe` decodificavam o topo da resposta; a Instagram API with Instagram Login responde em `data[]`. O callback lia campos vazios ou persistia o identificador errado. Agora os dois envelopes são `data[]`, `app_scoped_user_id` e `ig_user_id` são persistidos **separadamente**, `/me` é obrigatório (sem fallback para o id app-scoped) e resposta vazia/múltipla é recusada.
2. **Callbacks públicos da Meta eram stubs sem verificação.** `Deauthorize` e `DataDeletion` respondiam `{"ok":true}` sem olhar o `signed_request` — qualquer um podia POSTar e apagar a conta de um cliente. Agora: HMAC-SHA256 em tempo constante, corpo limitado a 64 KiB, exatamente um campo `signed_request`, validação de algoritmo e tempo, exclusão transacional e resposta com `url` + `confirmation_code`.
3. **State OAuth não era atomicamente de uso único.** `verifyState` fazia `GET` e depois `DELETE`: dois callbacks concorrentes consumiam o mesmo nonce. Agora `GETDEL` atômico, Redis obrigatório (fail-closed) e o `user_id` local saiu do state — ele trafega pela Meta e fica no histórico do navegador.
4. **Idempotência de Story dependia do Redis por 1h e falhava aberto.** `SETNX` com TTL de 1h retornando `true` em erro. Redelivery tardio, corrida entre chips ou Redis fora do ar publicavam o mesmo Story duas vezes. Agora a unicidade é durável no PostgreSQL (`uq_instagram_story_posts_source_event`), via `GetOrCreate` atômico, com `TaskID` determinístico e sem PII como segunda barreira.
5. **Retry podia publicar de novo.** O container vivia só na pilha de `PublishStoryImage`. Timeout depois de a Meta aceitar o `/media_publish` fazia o retry criar outro container. Agora `container_id` é persistido **antes** de qualquer polling, serializado por conta, com consulta de status antes de repetir e `PUBLISHED` reconhecido como sucesso reconciliado.

### Altos

6. **Todo 400/401/403 virava erro de autenticação permanente.** Erro de processamento recuperável perdia o retry e pedia "reconecte sua conta". Agora há `MetaAPIError` tipado com classificação por `code`, `error_subcode`, `is_transient` e status HTTP.
7. **Cota oficial era consultável mas nunca aplicada.** Agora o guard local (90) e a consulta a `content_publishing_limit` (100) rodam antes de criar container.
8. **Sem lock por conta profissional.** Agora lock Redis por SHA-256 do `user_id`, TTL 2min, liberação compare-and-delete, fail-closed.
9. **Expiração de credencial era ignorada.** Presença da linha significava "conectado" mesmo com token vencido. Agora `Status`, `HasCredentials` e `AccessToken` exigem token não vencido; `AccessToken` devolve `ErrInstagramTokenExpired` sem vazar token/id.
10. **Feature era ativada implicitamente por segredo presente.** Agora existe `INSTAGRAM_STORIES_ENABLED` (default `false`), separado de `Configured()`.
11. **`main.go` podia entrar em panic no boot.** `instagram.NewMediaStore(redisClient.Client, …)` estava fora de guarda de nil. Agora só é construído com `redisClient != nil`.
12. **Sem retenção.** Agora 30 dias para ledger e comprovantes, em lotes de 1000 e no máximo 10 lotes por execução, no cron `instagram_retention_cleanup`.
13. **Falha transitória final deixava o ledger em `pending` para sempre.** Agora a última tentativa grava `failed/transient_exhausted` e retorna `nil` (o job não é arquivado — o payload pode conter o texto da mensagem do cliente).

### Médios

14. **Endpoint público de mídia aceitava qualquer id e devolvia o MIME armazenado.** Agora exige `^[0-9a-f]{32}$` antes de tocar o Redis, responde sempre `image/jpeg`, com `Cache-Control: no-store` e `X-Content-Type-Options: nosniff`, e 404 uniforme.
15. **Decodificação limitava bytes, não pixels.** Agora `image.DecodeConfig` antes de `Decode`, rejeitando dimensões não positivas e área acima de 20.000.000 px.
16. **Download WAHA reescrevia qualquer path e levava a API key junto.** O host já era descartado, mas o path vinha de fora — um path forjado alcançaria endpoints internos autenticados da WAHA com a nossa credencial. Agora só `/api/files/…`, com rejeição de traversal, escape divergente, backslash, query, fragmento e userinfo.
17. **Erro cru podia ser persistido/exibido.** Agora o banco só recebe códigos enumerados; o detalhe fica em log estruturado sem token, segredo, signed request, conteúdo ou URL com query.
18. **Não havia teste de service OAuth/state/refresh nem de trigger.** Ver seção 7.

---

## 3. Contrato oficial da Meta (reconfirmado em 23/07/2026)

Produto: **Instagram API with Instagram Login**, conta profissional Business/Creator, sem Facebook Page.

| Capacidade | Contrato aplicado |
|---|---|
| Escopos | exatamente `instagram_business_basic,instagram_business_content_publish` |
| Troca code → short token | `POST https://api.instagram.com/oauth/access_token`; resposta **em `data[]`**; `user_id` é o id **app-scoped** |
| Long-lived token | `GET https://graph.instagram.com/access_token`; `{access_token, token_type, expires_in}` no topo |
| Refresh | `GET /refresh_access_token`; mesmo shape; exige token com ≥24h |
| Perfil | `GET /v25.0/me?fields=user_id,username,account_type`; resposta **em `data[]`**; `user_id` é o id **profissional** usado para publicar |
| Story | `/media` com `media_type=STORIES` + `image_url` → status → `/media_publish` |
| Limite | 100 publicações por conta em janela móvel de 24h (`content_publishing_limit`) |
| Data deletion | callback recebe `signed_request` e responde URL de status + código de confirmação |

Fontes reabertas nesta execução:

- Business Login e tokens: <https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/business-login> — confirmou o envelope `data[]` da troca do code e o shape plano do long-lived/refresh.
- Setup e `/me`: <https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/get-started> — confirmou o envelope `data[]` do `/me` e os valores `Business` / `Media_Creator` de `account_type`.

**Observação de implementação.** A doc mostra `account_type` como `Business`/`Media_Creator`, mas o campo não aparece no exemplo do envelope e outros endpoints da Meta já responderam em caixa alta. A comparação é case-insensitive e `account_type` **ausente** não reprova a conta — só um valor presente e não-profissional reprova. Um match exato e obrigatório recusaria contas legítimas.

**Rota de mídia da WAHA:** `/api/files/…`, confirmada no campo `Media.url` do `waha-openapi.json` deste repositório e em `waha-plus-openapi.json:8846` na raiz do projeto — este último fica **fora** do repo git (exemplo `http://localhost:3000/api/files/false_…oga`). Em produção `WAHA_S3_PROXY_FILES=True`, então a própria WAHA proxia o S3 por essa rota — a allowlist não muda o comportamento atual.

Não aplicável nesta entrega: nenhum recebimento de comentários, Direct ou menções; nenhum `hub.challenge` ou assinatura de webhook; as permissões `instagram_business_manage_messages` e `instagram_business_manage_comments` **não** são solicitadas; nenhuma janela de resposta é implementada ou alegada.

---

## 4. Migração 077

Aditiva, sem reescrita de linhas e sem `NOT NULL` sem default.

- `instagram_credentials`: `app_scoped_user_id TEXT NOT NULL DEFAULT ''`; índices únicos parciais `uq_instagram_credentials_app_scoped_user_id` e `uq_instagram_credentials_ig_user_id` (parciais em `<> ''` para as linhas legadas da 073 não colidirem entre si).
- `instagram_story_posts`: `container_id`, `processing_started_at`, `attempt_count`, `error_code`, `published_reconciled`; constraint `instagram_story_posts_attempt_count_nonnegative`; índice único parcial `uq_instagram_story_posts_source_event (user_id, source_group, message_id) WHERE message_id <> ''`; índice `idx_instagram_story_posts_retention (created_at, id)`.
- `instagram_data_deletion_requests`: comprovantes com `subject_hash` (HMAC do id app-scoped — o id bruto não é armazenado) e `confirmation_code` aleatório de 256 bits.

O `down` existe **apenas para o banco efêmero de teste** e devolve o schema ao estado da 073. Não foi executado em produção.

---

## 5. Máquina de estados do worker

```
pending, sem lease
  ├─ TryStartProcessing OK → pending + processing_started_at + attempt_count++
  │    ├─ sucesso normal            → published
  │    ├─ status Meta PUBLISHED     → published + published_reconciled=true
  │    ├─ erro permanente           → failed (código seguro)
  │    └─ transitório antes do fim  → pending, lease liberado
  ├─ lease fresco   → retry transitório sem alterar o ledger
  └─ lease vencido (>2min) → novo TryStartProcessing permitido

published/failed → terminais e idempotentes
```

Estados públicos continuam sendo três: `pending`, `published`, `failed`. "Em processamento" não é um quarto estado — é o lease `processing_started_at` sobre uma linha ainda `pending`, para que um worker morto não trave a linha.

Códigos seguros do ledger: `feature_disabled`, `invalid_payload`, `media_expired`, `media_invalid`, `media_too_large`, `quota_local`, `quota_meta`, `auth_required`, `token_expired`, `container_error`, `container_expired`, `publish_rejected`, `enqueue_failed`, `transient_exhausted`.

**Desvio consciente da ordem do plano (§14.3).** O plano mandava preparar o JPEG (passo 11) antes de verificar se já existe container (passo 12). Com container já criado, a publicação não depende mais dos bytes — e a mídia efêmera (TTL 30min) pode ter expirado. Preparar antes transformaria um retry recuperável em `media_expired` sobre um Story possivelmente já publicado. A imagem passou a ser preparada **apenas** quando `container_id` está vazio. Coberto por `TestInstagramStoryWorker_RetryReusesPersistedContainer`.

---

## 6. Feature flag e comportamento desligado

- `INSTAGRAM_STORIES_ENABLED` (default `false`) é o kill switch dedicado.
- `Configured()` exige: app id+secret, chave de criptografia, chave de state com ≥32 bytes, Redis real, e `redirect_uri` / `BACKEND_PUBLIC_URL` / `PUBLIC_BASE_URL` HTTPS válidos.
- `FeatureEnabled() = StoriesEnabled && Configured()`.
- Flag ligada com configuração incompleta: **startup normal**, feature forçada desligada, log estruturado sem valores secretos + evento no Sentry. O processo não é derrubado por uma feature opcional mal configurada.
- `RefreshExpiring` roda com `TokenMaintenanceConfigured()` — deliberadamente mais frouxo — para que uma semana com a publicação desligada não vença silenciosamente a credencial de quem já conectou.
- Os callbacks públicos são registrados **sempre**, para que os callbacks configurados no app da Meta não virem 404.
- Nenhuma variável foi adicionada a `.env`, ao compose de produção ou ao CI.

---

## 7. Testes executados

Todos no worktree isolado, em `automatiza-go/`.

| Comando | Resultado |
|---|---|
| `go build ./...` | PASS |
| `go vet` (9 pacotes tocados) | PASS |
| `go test ./internal/services/instagram/ -count=1` | PASS — 51 testes |
| `go test -race ./internal/services/instagram/ -count=1` | PASS |
| `go test ./internal/workers/ -run 'Instagram\|Story' -count=1` | PASS — 34 testes |
| `go test -race ./internal/workers/ -run 'Instagram\|Story' -count=1` | PASS |
| `go test ./internal/handlers/ -run Instagram -count=1` | PASS — 13 testes |
| `go test ./internal/lib/cache/ -run 'GetDel\|CompareAndDelete' -count=1` | PASS — 6 testes |
| `go test ./internal/adapters/whatsapp/ -run 'SafeWAHAMediaPath' -count=1` | PASS — 4 testes |
| `go test ./internal/repository/postgres/ -run Instagram -count=1` | PASS — 14 testes (PostgreSQL efêmero) |

Total: **122 testes** desta entrega, todos verdes.
| `go test ./internal/lib/imagerender/ ./internal/lib/text/ -count=1` | PASS (regressão: `{story}` intacto) |

Os testes de PostgreSQL aplicam as migrações **reais** `073` e `077` lidas de `scripts/db/migrations/`, não um schema reescrito à mão — assim o teste prova o arquivo que iria para produção.

Provas de concorrência (com `-race`): um único vencedor no `GETDEL` do nonce, um único vencedor no `GetOrCreate`, um único vencedor no `TryStartProcessing`, e o lock por conta impedindo duas publicações simultâneas da mesma conta.

---

## 7.1 Revisão final independente e correções aplicadas

Um revisor somente-leitura (contexto novo) leu o plano, o diff completo, os arquivos novos e o código ao redor. Achados e desfechos:

| # | Severidade | Achado | Desfecho |
|---|---|---|---|
| 1 | ALTO | `finish()` engolia falha de persistência: com um blip no Postgres logo após um `/media_publish` bem-sucedido, o job retornava `nil`, o asynq o descartava e a linha ficava `pending` **para sempre** — a auditoria registrando o oposto do que aconteceu. Violação direta do plano §14.5. | **Corrigido.** `finish` devolve erro; os chamadores propagam; o retry reencontra o container em `PUBLISHED` e reconcilia. Na última tentativa, `persistFailure` registra em observabilidade e devolve `nil` para não arquivar o payload. Testes `PersistFailureIsRetriedNotSwallowed` e `PersistFailureOnLastAttemptDoesNotArchive`. |
| 2 | MÉDIO | Migração 077 podia abortar sobre duplicatas legadas da 073, e o teste nunca semeava esse caso (plano §7 pedia a pré-checagem). | **Corrigido em duas frentes.** Prova física: produção verificada em 23/07/2026 tem **0 credenciais, 0 stories, 0 duplicatas**. E o teste `Migration077_FailsLoudlyOnLegacyDuplicates` semeia um par duplicado, roda a query de pré-checagem e fixa o comportamento — falhar alto é intencional; deduplicar dado de cliente em silêncio seria pior. |
| 3 | MÉDIO | `ReleaseProcessing`/`MarkResult` não verificavam o dono do lease. Um worker zumbi (o asynq cancela o contexto no timeout, mas não mata a goroutine) liberava o lease de quem já o retomara, ou carimbava `failed` sobre uma linha já `published`. | **Corrigido.** `TryStartProcessing` devolve o carimbo do lease como token de posse; `ReleaseProcessing` exige `processing_started_at = $2`; `MarkResult` ganhou `AND status = 'pending'`. Testes `ZombieCannotReleaseAnotherLease` e a extensão de `MarkResultClearsLease`. |
| 4 | MÉDIO | `professionalAccountTypes` incluía `"creator"`, fora do contrato. | **Corrigido.** Só `business` e `media_creator`. A tolerância a `account_type` **ausente** permanece e segue documentada logo acima — recusar ali quebraria conexões legítimas, e o escopo `instagram_business_content_publish` já exclui conta pessoal. |
| 5 | MÉDIO | `safeWAHAMediaPath` rejeitava **qualquer** path percent-encodado (`RawPath != ""`), o que derrubaria o espelhamento de imagem — caminho quente compartilhado com monitoring e offer_generate — para nomes de arquivo com `%20`/acento. | **Corrigido.** A validação passou a rodar sobre o path **decodificado** (`path.Clean` + prefixo) e o que sai é o path **reescapado**. `..%2f..%2f` continua sendo recusado (decodifica para traversal), e `foto%20do%20dia.jpeg` passa. Teste `AcceptsEscapedFilenames`; a suíte de paths perigosos continua verde. |
| 6 | MÉDIO | Teste obrigatório #40 (rate limit → 429) não existia. | **Corrigido.** `TestInstagramPublicRoutes_RateLimitBlocksBeforeHandler` registra o mesmo middleware do `router.go` e prova 429 com o handler não executado. |
| 7 | BAIXO | Erro de `CountPublishedSince` silenciado. | **Corrigido.** Passou a logar; o desfecho segue não-fatal, porque a cota da Meta logo abaixo é a fonte autoritativa e é fail-closed. |
| 8 | BAIXO | Trigger logava JID do grupo e id da mensagem, fora da lista permitida do plano §17 (pré-existente no baseline). | **Corrigido.** Logs passaram a levar apenas contagens e classe; a tag `sourceGroup` saiu do Sentry. |
| 9 | BAIXO | Documento citava `waha-plus-openapi.json:8846`, arquivo que fica na raiz do projeto e **não** dentro do repo git. | **Corrigido.** A citação agora aponta o campo `Media.url` do `waha-openapi.json` do repo e explicita onde o outro arquivo vive. |
| 10 | BAIXO | Falha em `SaveContainerID` faz o retry criar um segundo container na Meta. | **Aceito, não corrigido.** Desperdiça um container, mas nunca publica duas vezes (só o último é publicado). Corrigir exigiria um segundo ponto de verdade para o container; o custo não se justifica nesta fase. Registrado aqui como dívida conhecida. |
| 11 | BAIXO | `Deauthorize` exigia `BACKEND_PUBLIC_URL` HTTPS, mais restritivo que o contrato. | **Já corrigido** antes da revisão chegar (`DeauthorizeConfigured`). |
| 12 | BAIXO | Teste `AcceptedPathsNeverEscapeMediaPrefix` era quase-tautológico (o `continue` fazia passar mesmo se tudo fosse recusado). | **Corrigido.** Agora exige que **todas** as entradas legítimas sejam aceitas. |
| 13 | BAIXO | Troca da base do commit precisa de confirmação do usuário. | **Confirmado** — a troca foi aprovada explicitamente antes da execução (ver seção 1). |

O revisor não encontrou defeito em: escopo/proibições, contrato Meta, natureza aditiva da migração, autenticação e replay, dupla publicação, retenção, regressão fora do Instagram e segurança dos fakes concorrentes.

### Regressão em pacotes tocados

`go test` completo em `internal/workers`, `internal/handlers`, `internal/lib/cache`, `internal/config`, `internal/models`: verdes.

`internal/adapters/whatsapp`: falha apenas `TestWahaAdapter_GetGroupInviteCode_Advanced`, **pré-quebrado conhecido** e listado no `CLAUDE.md`.

`internal/handlers` tem uma **falha intermitente pré-existente** (~1 em 4 execuções, com `redis: connection pool: failed to dial ... connection refused`). Foi reproduzida em um worktree limpo no **mesmo commit base, sem nenhuma das mudanças desta entrega** — portanto não é regressão daqui. A execução com `-v` não reproduziu em 17 tentativas, então o teste específico não foi identificado; o padrão sugere disputa de porta entre instâncias de Miniredis quando os pacotes rodam em paralelo. Fica registrado como achado, não corrigido (seria trabalho não relacionado a esta entrega).

---

## 8. Testes bloqueados / limitações

- **Nenhuma chamada foi feita à Meta real.** Todo o fluxo OAuth e de publicação é exercitado contra `httptest`, Miniredis e PostgreSQL efêmero. Isto **não** é validação do canal real.
- A verificação ponta a ponta do canal depende do bloqueio **B1** (app Meta de teste), abaixo.

---

## 9. Bloqueios externos

### B1 — app Meta de teste (dono: usuário)

Necessário, fora desta execução: app tipo Business com o produto Instagram API with Instagram Login; `INSTAGRAM_APP_ID`; `INSTAGRAM_APP_SECRET`; redirect URI HTTPS exata; conta profissional de teste adicionada ao dashboard do app; domínio público HTTPS para a mídia; e um segredo separado para `INSTAGRAM_STATE_SIGNING_KEY` (32 bytes aleatórios — **não** gerado nem persistido nesta entrega).

Só então cabe o sandbox real manual: conectar conta, provar state/replay, publicar uma imagem autorizada, observar container/publish/cota, refresh quando elegível, disconnect, deauthorize e data deletion na configuração da Meta. Essa execução exige autorização nova do usuário.

### B2 — Advanced Access / App Review (dono: usuário/Meta)

Standard Access cobre apenas contas próprias/gerenciadas adicionadas ao dashboard. Contas de clientes exigem Advanced Access e App Review aprovados antes de qualquer ativação multi-tenant.

---

## 10. Arquivos

**Novos (11):** migração `077` up/down; `internal/services/instagram/signed_request.go` + teste; `internal/services/instagram/instagram_service_test.go`; `internal/services/instagram/media_store_test.go`; `internal/repository/postgres/instagram_data_deletion_repository.go`; `internal/repository/postgres/instagram_repository_test.go`; `internal/workers/instagram_story_trigger_test.go`; `internal/handlers/instagram_public_handler_test.go`; este documento.

**Alterados (21):** `cmd/server/main.go`; `internal/config/config.go`; `internal/models/instagram.go`; `internal/repository/interfaces.go`; `internal/repository/postgres/{repositories,instagram_credentials_repository,instagram_story_posts_repository}.go`; `internal/lib/cache/cache.go` + teste; `internal/services/instagram/{api_client,instagram_service,media_store}.go` + `api_client_test.go`; `internal/handlers/{instagram_public_handler,router}.go`; `internal/workers/{asynq,instagram_story_trigger,instagram_story_worker}.go` + `instagram_story_worker_test.go`; `internal/adapters/whatsapp/waha_adapter.go` + teste.

Intocados, como exigido: proto, `internal/gen`, `client/`, tag parser, templates e as migrações 073/075/076.

---

## 11. Capacidades

- **Suportado localmente:** OAuth e publicação de Stories endurecidos, provados por fakes, `httptest` e PostgreSQL efêmero.
- **Dependente de credenciais/revisão:** sandbox Meta real, Standard Access, Advanced Access.
- **Não suportado nesta entrega:** qualquer inbound (comentários, Direct, menções), janela de resposta, inbox, draft, IA e `auto_reply`.
- **Produção / M3:** não aplicável — nada foi deployado e a feature permanece desligada.
