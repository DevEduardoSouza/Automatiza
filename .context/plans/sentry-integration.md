---
status: filled
generated: 2026-02-11
updated: 2026-02-11
agents:
  - type: "backend-specialist"
    role: "Implementar Sentry SDK no Go (Gin + ConnectRPC + Workers) + corrigir erros silenciosos"
  - type: "frontend-specialist"
    role: "Implementar Sentry SDK no React 19 (ConnectRPC + SSE + Error Boundaries) + corrigir erros silenciosos"
  - type: "security-auditor"
    role: "Validar que PII nao vaza para o Sentry (redaction/scrubbing) + auditar erros de segurança silenciosos"
docs:
  - "architecture.md"
  - "security.md"
  - "data-flow.md"
phases:
  - id: "phase-1"
    name: "Backend Go - Sentry Core + Middleware + Interceptors"
    prevc: "E"
  - id: "phase-2"
    name: "Backend Go - Workers + Services + Integrações Externas"
    prevc: "E"
  - id: "phase-3"
    name: "Backend Go - Erros Silenciosos (Auth, Cache, History, Goroutines)"
    prevc: "E"
  - id: "phase-4"
    name: "Frontend React - Sentry Core + Error Boundaries"
    prevc: "E"
  - id: "phase-5"
    name: "Frontend React - ConnectRPC + SSE + Hooks + Erros Silenciosos"
    prevc: "E"
  - id: "phase-6"
    name: "Validação end-to-end + Source Maps + Alertas"
    prevc: "V"
---

# Sentry Integration - Backend Go + Frontend React 19

> Integração completa do Sentry para captura de 100% dos erros em produção e localhost, cobrindo backend Go (Gin + ConnectRPC + Workers) e frontend React 19 (ConnectRPC + SSE + Error Boundaries), incluindo correção de 96 erros silenciosos no backend, 32+ no frontend, 39 pontos de concorrência/resiliência/runtime, 103 pontos de análise estática profunda, 84 pontos WAHA Plus GoWS (44 backend + 40 frontend), e 30 pontos de cobertura final (15 backend + 15 frontend) — **447 pontos totais**

## Task Snapshot

- **Primary goal:** Capturar 100% dos erros (handled + unhandled + silenciosos) em produção com contexto rico (user, traceId, módulo, tags) sem vazar PII, habilitando debugging eficiente e alertas proativos.
- **Success signal:** Erros em produção aparecem no Sentry com stack traces legíveis (source maps no frontend), contexto de usuário/sessão, e traceId correlacionado entre frontend e backend. Zero erros silenciosos restantes na codebase.

## Codebase Context

### Backend Go (automatiza-go/)
- **Entry point:** `cmd/server/main.go` - inicializa Gin, ConnectRPC, Redis, PostgreSQL, WAHA, Asynq workers
- **Middleware existente:** `internal/middleware/error_handler.go` (ApiError + panic recovery), `logging.go` (Zap structured)
- **ConnectRPC interceptors:** `internal/connectrpc/interceptors.go` - 7 interceptors (Recovery, Tracing, Logging, Auth, CSRF, Validation, RateLimit) + `suspension_interceptor.go`
- **Workers (Asynq):** 7 workers em `internal/workers/` - webhook consumer, message, groups sync, reconciliation, dispatch log, monitoring, message queue
- **Services:** 11+ services em `internal/services/` - automation, reconciliation, kiwify, mercadolivre, ai, shopee, email, profile, coupons, redirect, monitoring
- **Logger atual:** Zap (structured JSON) configurado em `internal/config/logger.go`
- **Custom errors:** `internal/shared/errors/conflict_error.go` (ConflictError)
- **External APIs:** WAHA (WhatsApp), Kiwify (webhooks), Mercado Livre, Shopee, OpenAI/Claude, SMTP

### Frontend React (client/)
- **Entry point:** `src/main.tsx` -> `src/app.tsx` (providers: QueryClient, ConnectRPC Transport, Auth, Router)
- **ConnectRPC transport:** `src/lib/connect-transport.ts` - 4 interceptors (CSRF, Tracing, Retry, ErrorNormalization) + error mapping completo
- **Error boundary:** `src/components/error-boundary.tsx` - existe mas NAO esta no root
- **Auth context:** `src/contexts/auth-context.tsx` - session management com error handling
- **SSE:** `src/hooks/use-session-status-sse.ts` - real-time events com reconnect
- **Toast system:** Sonner + custom useToast hook
- **QueryClient:** retry=false (delegado aos interceptors ConnectRPC)
- **Hooks com try/catch:** 15+ hooks customizados (useAutomations, useWhatsApp, useJobs, useRedirectLink, etc.)
- **Build:** Vite 7.3.1 - source maps NAO habilitados
- **Trace context:** Ja gera traceparent + x-trace-id nos interceptors

---

## Estrutura Organizacional

### Backend Go (`automatiza-go/`)

```
cmd/server/main.go          <- Entry point (Gin + ConnectRPC + Workers + Redis + PG + WAHA)

internal/
├── adapters/
│   ├── email/               # SMTP adapter (SSL/STARTTLS)
│   └── whatsapp/            # WAHA Plus adapter (HTTP client, HMAC, models, mapper)
├── auth/                    # Auth service, session store, rate limiter, tokens
├── config/                  # App config (Viper), database pool (pgx), logger (Zap)
├── connectrpc/              # ConnectRPC handlers (28+), interceptors (7), mappers
├── errors/                  # Custom error types
├── handlers/                # Gin HTTP handlers (webhooks, SSE hub, SSE Redis bridge, stubs)
├── lib/
│   ├── backpressure/        # Queue backpressure tracking
│   ├── cache/               # Redis + local sync.Map cache
│   ├── crypto/              # Crypto utilities
│   ├── text/                # Text processing
│   ├── urlsafety/           # URL safety checking (external API)
│   └── urlvalidation/       # URL normalization
├── middleware/               # Gin middleware (error_handler, logging, rate_limit, CORS)
├── models/                  # Domain models
├── repository/postgres/     # PostgreSQL repositories (pgx pool, 15+ repos)
├── services/
│   ├── ai/                  # OpenAI/Claude integration
│   ├── automation/          # Automation engine + process list
│   ├── cron/                # Cron scheduler (ticker-based tasks)
│   ├── jobs/                # Scheduled jobs management
│   ├── kiwify/              # Kiwify webhook processing
│   ├── loadbalancer/        # Server load balancing
│   ├── mercadolivre/        # Mercado Livre affiliate
│   ├── mllinklists/         # ML link list management
│   ├── monitoring/          # Real-time monitoring (dedup, dispatch)
│   ├── profile/             # User profile + password service
│   ├── reconciliation/      # Data reconciliation + watchdog
│   ├── redirect/            # Redirect link service
│   ├── shopee/              # Shopee affiliate
│   ├── waha/                # WAHA session management
│   ├── watchdog/            # Connection watchdog (reconnect, health)
│   └── whatsapp/            # WhatsApp groups, messages, instances
├── shared/errors/           # Shared error types (ConflictError)
└── workers/                 # Asynq workers (7): webhook_consumer, message,
                             #   groups_sync, reconciliation, dispatch_log,
                             #   monitoring, message_queue
```

### Frontend React (`client/src/`)

```
src/
├── components/
│   ├── admin/               # Admin panel (dashboard, users, plans)
│   ├── affiliate-programs/  # Affiliate program UI (ML, Awin, Shopee)
│   ├── automation-logs/     # Automation log viewer (virtual table)
│   ├── automations/         # Automation CRUD + 11 form sections
│   ├── groups/              # WhatsApp group management (13+ dialogs)
│   ├── history/             # Action history viewer (table, filters, stats)
│   ├── layout/              # App layout (sidebar, header)
│   ├── routing/             # Route guards (auth, role)
│   └── ui/                  # shadcn/ui primitives
├── contexts/                # React contexts (auth-context.tsx)
├── entities/                # FSD entities (dispatch-text, ml-link-list, whatsapp-instance)
├── gen/                     # Generated protobuf stubs (ConnectRPC) -- NAO editar
├── hooks/
│   ├── groups/              # use-groups.ts, use-groups-messaging.ts
│   └── *.ts                 # 15+ feature hooks (use-whatsapp, use-automations, etc.)
├── i18n/locales/            # Translation files (pt-br.json)
├── lib/
│   └── connect/             # ConnectRPC transport + 4 interceptors
├── pages/
│   ├── jobs/                # Jobs page + sub-components
│   └── redirect-link/       # Redirect link page + sub-components
├── services/                # Legacy API service (axios) -- api.service.ts
└── types/                   # TypeScript type definitions
```

---

## Auditoria de Erros Silenciosos (Deep Analysis)

> Descobertos via varredura profunda de 298 arquivos Go e todos os .ts/.tsx do frontend. Estes erros NAO eram cobertos pelo plano original.

### BACKEND GO - 96 Erros Silenciosos em 11 Categorias

#### Cat. 1: Supressão Explícita com `_ =` (23 issues)

##### CRITICOS - Segurança/Auth (`internal/auth/`)

| # | Arquivo:Linha | Código | Risco | Ação |
|---|---------------|--------|-------|------|
| S1 | `auth/service.go:156` | `_ = s.incrementFailedAttempts(ctx, creds.UserID)` | CRITICO: tracking de brute-force quebrado | Capturar + alertar |
| S2 | `auth/service.go:156` | `_, _ = s.rateLimiter.Increment(ctx, rateLimitKey)` | CRITICO: rate limiting desabilitado silenciosamente | Capturar level Critical |
| S3 | `auth/service.go:190` | `_ = s.rateLimiter.Reset(ctx, rateLimitKey)` | CRITICO: rate limit nunca reseta | Capturar level Critical |
| S4 | `auth/service.go:435` | `_, _ = s.sessionStore.RevokeAllForUser(ctx, userID)` | CRITICO: sessões não revogadas após troca de senha | Capturar level Critical + alerta |
| S5 | `auth/service.go:321` | `_ = s.sessionStore.UpdateActivity(ctx, sessionID)` | MEDIO: tracking de atividade perdido | Capturar level Warning |
| S6 | `auth/service.go:189` | `_ = s.resetFailedAttempts(ctx, creds.UserID)` | MEDIO: contagem de falhas nunca reseta | Capturar level Warning |
| S7 | `auth/service.go:432` | `_ = s.markPasswordResetTokenUsed(ctx, HashToken(token))` | MEDIO: token reutilizável | Capturar level Warning |
| S8 | `auth/service.go:232` | `existing, _ := s.getUserByEmail(ctx, email)` | ALTO: checagem de email duplicado falha | Capturar |
| S9 | `auth/service.go:674` | `_ = s.db.From("user_roles")...All(ctx, &roles)` | ALTO: query de roles ignorada | Capturar |

##### ALTOS - Session/Cache (`internal/auth/session.go`)

| # | Arquivo:Linha | Código | Risco | Ação |
|---|---------------|--------|-------|------|
| S10 | `auth/session.go:190` | `_ = err` (cache invalidation) | Cache inconsistente | Capturar level Warning |
| S11 | `auth/session.go:224` | `_ = s.cacheSession(ctx, session)` | Sessão não cacheada | Capturar level Warning |
| S12 | `auth/session.go:283` | `_ = s.cacheSession(ctx, session)` | Cache update perdido | Capturar level Warning |
| S13 | `auth/session.go:398` | `_ = s.cacheSession(ctx, session)` | CSRF token cache perdido | Capturar level Warning |
| S14 | `auth/session.go:446` | `_ = s.redis.Del(ctx, key)` | Cache delete falha | Capturar level Warning |

##### ALTOS - Startup/Shutdown (`cmd/server/main.go`)

| # | Arquivo:Linha | Código | Risco | Ação |
|---|---------------|--------|-------|------|
| S15 | `main.go:93` | `_ = logger.Sync()` | Log final perdido | Capturar em defer |
| S16 | `main.go:119` | `_ = redisClient.Close()` | Conexão Redis não limpa | Capturar em defer |
| S17 | `main.go:223` | `_ = queueClient.Close()` | Queue client vazamento | Capturar em defer |
| S18 | `main.go:538` | `_ = asynqInspectorServer.Close()` | Inspector não fechado | Capturar em defer |
| S19 | `main.go:1062` | `_ = asynqInspector.Close()` | Inspector vazamento | Capturar em defer |
| S20 | `main.go:1066` | `_ = asynqClient.Close()` | Client vazamento | Capturar em defer |

##### MEDIOS - Config (`internal/config/`)

| # | Arquivo:Linha | Código | Risco | Ação |
|---|---------------|--------|-------|------|
| S21 | `config/config.go:183` | `_ = viper.ReadInConfig()` | Config file ignorado | Log warning |
| S22 | `config/config.go:211` | `_ = viper.BindEnv(tag)` | Env binding falha | Log warning |
| S23 | `config/database.go:199,335,507` | `_ = rows.Close()` (3x) | rows.Close() error perdido | Capturar level Warning |

#### Cat. 2: Type Assertions sem `ok` (8 issues)

| # | Arquivo:Linha | Código | Risco | Ação |
|---|---------------|--------|-------|------|
| S24 | `handlers/stubs.go:74` | `instance, _ := h.deps.InstanceRepo.GetInstanceByUserAndName(...)` | Erro de instância retorna 404 incorreto | Tratar erro |
| S25 | `handlers/stubs.go:645` | `events, _ := h.deps.SystemEventRepo.GetRecent(...)` | Eventos parciais/vazios | Tratar erro |
| S26 | `handlers/automations_handler.go:142` | `updated, _ := h.deps.AutomationRepo.GetAutomationByIDAndUser(...)` | Automation fetch invisível | Tratar erro |
| S27 | `handlers/sse_handler.go:74` | `instance, _ := h.deps.InstanceRepo.GetInstanceByUserAndName(...)` | SSE com instância errada | Tratar erro |
| S28 | `repository/postgres/settings_repository.go:206-207` | `name, _ := updates["name"].(string)` | Settings update tipo errado | Validar tipo |
| S29 | `repository/postgres/coupon_repository.go:115` | `t, _ := time.Parse(time.RFC3339, *data.ExpiresAt)` | Data expiração vira zero time | Validar + capturar |
| S30 | `services/shopee/shopee_service.go:601-610` | `shopID, _ = strconv.ParseInt(...)` (4x) | IDs Shopee viram 0 | Validar + capturar |
| S31 | `main.go:124` | `port, _ := strconv.Atoi(pgCfg.Port)` | CRITICO: porta DB vira 0 | Validar + fail fast |

#### Cat. 3: History/Dispatch Log Fire-and-Forget (12 issues)

| # | Arquivo | Linha | Operação | Risco |
|---|---------|-------|----------|-------|
| S32 | `workers/message_worker.go` | 323 | `_ = w.historyRepo.Update(...)` | Status de mensagem nunca atualizado |
| S33 | `services/whatsapp/message_service.go` | 167 | `_ = s.historyRepo.Update(...)` | Imagem: falha não registrada |
| S34 | `services/whatsapp/message_service.go` | 178 | `_ = s.historyRepo.Update(...)` | Audio: sucesso não persistido |
| S35 | `services/whatsapp/message_service.go` | 251 | `_ = s.historyRepo.Update(...)` | Poll: falha perdida |
| S36 | `services/whatsapp/message_service.go` | 261 | `_ = s.historyRepo.Update(...)` | Poll: sucesso não registrado |
| S37 | `services/whatsapp/message_service.go` | 320 | `_ = s.historyRepo.Update(...)` | Link: falha não rastreada |
| S38 | `services/whatsapp/message_service.go` | 330 | `_ = s.historyRepo.Update(...)` | Link: sucesso perdido |
| S39 | `services/whatsapp/message_service.go` | 400 | `_ = s.historyRepo.Update(...)` | Documento: falha invisível |
| S40 | `services/whatsapp/message_service.go` | 410 | `_ = s.historyRepo.Update(...)` | Documento: sucesso não persistido |
| S41 | `services/automation/automation_service.go` | 351 | `_ = s.historyRepo.Update(...)` | Target count perdido |
| S42 | `services/automation/automation_service.go` | 354 | `_ = s.historyRepo.MarkRunning(...)` | Status running não registrado |
| S43 | `services/automation/automation_service.go` | 388 | `_ = s.historyRepo.Update(...)` | Status final perdido |

#### Cat. 4: Cache Operations Silenciosas (13 issues)

| # | Arquivo | Linha | Operação | Risco |
|---|---------|-------|----------|-------|
| S44 | `services/monitoring/monitoring_service.go` | 105 | `_ = s.cache.Set(...)` | Dedup falha → duplicatas |
| S45 | `services/monitoring/monitoring_service.go` | 367 | `_ = s.cache.Set(...)` | Flag processado perdido |
| S46 | `services/monitoring/monitoring_service.go` | 418 | `_ = s.cache.Set(...)` | Dedup cache perdido |
| S47 | `services/shopee/shopee_service.go` | 436 | `_ = s.cache.SetJSON(...)` | Cache Shopee não gravado |
| S48 | `services/shopee/shopee_service.go` | 586 | `_ = s.cache.Set(...)` | URL final não cacheada |
| S49 | `services/automation/automation_service.go` | 717 | `_ = s.cache.Set(...)` | Dedup automation falha |
| S50 | `services/loadbalancer/lb_service.go` | 115 | `_ = s.cache.SetJSON(...)` | Atribuição servidor perdida |
| S51 | `services/loadbalancer/lb_service.go` | 152 | `_ = s.cache.Delete(...)` | Cache invalidation falha |
| S52 | `services/loadbalancer/lb_service.go` | 183 | `_ = s.cache.Delete(...)` | Server delete não limpo |
| S53 | `services/loadbalancer/lb_service.go` | 201 | `_ = s.cache.Delete(...)` | Remoção não limpa |
| S54 | `services/loadbalancer/lb_service.go` | 342 | `_ = s.cache.SetJSON(...)` | Servers config não cacheado |
| S55 | `services/loadbalancer/lb_service.go` | 364 | `_ = s.cache.SetJSON(...)` | All servers cache perdido |
| S56 | `handlers/sse_handler.go` | 251 | `_ = s.cache.Set(...)` | Heartbeat cache perdido |

#### Cat. 5: JSON/Parsing Silenciosos (9 issues)

| # | Arquivo | Linha | Código | Risco |
|---|---------|-------|--------|-------|
| S57 | `repository/postgres/action_history_repository.go` | 86 | `jsonBytes, _ := json.Marshal(detailsMap)` | Histórico corrompido |
| S58 | `services/mllinklists/service.go` | 367 | `normalized, _ := urlvalidation.NormalizeURL(...)` | URL normalização falha |
| S59 | `services/mllinklists/service.go` | 376 | `normalized, _ := urlvalidation.NormalizeURL(...)` | Detecção duplicata falha |
| S60 | `services/mllinklists/service.go` | 400 | `normalized, _ := urlvalidation.NormalizeURL(...)` | ML link URL perdido |
| S61 | `services/kiwify/kiwify_service.go` | 420 | `matched, _ := regexp.MatchString(...)` | Regex match invisível |
| S62 | `connectrpc/webhooks_handler.go` | 75 | `logEntry, _ := h.webhookLogRepo.Create(...)` | Log webhook não criado |
| S63 | `connectrpc/webhooks_handler.go` | 96 | `logEntry, _ := h.webhookLogRepo.Create(...)` | Log evento não registrado |
| S64 | `connectrpc/webhooks_handler.go` | 158 | `logEntry, _ := h.webhookLogRepo.Create(...)` | Log simulação perdido |
| S65 | `connectrpc/webhooks_handler.go` | 77,98,160,177 | `_ = h.webhookLogRepo.UpdateStatus(...)` (4x) | Status webhook nunca atualizado |

#### Cat. 6: Goroutines sem Panic Recovery (3 issues - CRITICOS)

| # | Arquivo:Linha | Contexto | Risco |
|---|---------------|----------|-------|
| S66 | `main.go:853` | `go func() { srv.ListenAndServe() }` | Panic no servidor mata processo silenciosamente |
| S67 | `handlers/stubs.go:404` | `go func() { h.deps.MonitoringService.ProcessIncomingMessage(...) }` | Panic no monitoring sem trace |
| S68 | `handlers/sse_redis_bridge.go:107` | `go func() { pubsub.Channel() loop }` | Panic no SSE bridge perde eventos |

#### Cat. 7: Context Cancellation Ignorado (5 issues)

| # | Arquivo | Operação | Risco |
|---|---------|----------|-------|
| S69 | `workers/webhook_consumer_worker.go:232` | `routeEvent()` sem ctx.Done() | Worker pode travar |
| S70 | `handlers/sse_redis_bridge.go:114` | Loop manual no channel | Pode não cancelar |
| S71 | `middleware/webhook_rate_limiter.go:86` | `for range ticker.C` | Loop infinito |
| S72 | `services/cron/cron_service.go:214` | Ticker loops | Não sai limpo |
| S73 | `services/reconciliation/reconciliation_service.go:535` | Watchdog goroutine | Não cancela |

#### Cat. 8: Backpressure/Queue Silenciosos (3 issues)

| # | Arquivo:Linha | Código | Risco |
|---|---------------|--------|-------|
| S74 | `services/automation/automation_service.go:255` | `_ = s.backpressure.RecordEnqueue(...)` | Métrica perdida |
| S75 | `services/automation/process_list.go:255` | `_ = s.backpressure.RecordEnqueue(...)` | Tracking perdido |
| S76 | `services/monitoring/monitoring_service.go:452` | `_, _ = s.dispatchLogRepo.Create(...)` | Log dispatch não criado |

#### Cat. 9: Repository Errors Ignorados (8 issues)

| # | Arquivo:Linha | Código | Risco |
|---|---------------|--------|-------|
| S77 | `repository/postgres/profile_repository.go:218` | `_, _ = r.db.pool.Exec(ctx, profileQuery, ...)` | Password update silencioso |
| S78 | `repository/postgres/group_repository.go:535` | `_, _ = fmt.Sscanf(group.InstanceID, ...)` | Instance ID parse falha |
| S79 | `services/redirect/redirect_service.go:167` | `_, _ = fmt.Sscanf(group.InstanceID, ...)` | Instance ID invisível |
| S80 | `services/loadbalancer/lb_service.go:221` | `count, _ := s.userServerRepo.CountByServer(...)` | Utilização perdida |
| S81 | `services/loadbalancer/lb_service.go:266` | `count, _ := s.userServerRepo.CountByServer(...)` | Capacidade falha |
| S82 | `services/mercadolivre/ml_service.go:87` | `existing, _ := s.mlLinkRepo.GetByOriginalURL(...)` | Duplicata check falha |

---

### FRONTEND REACT - 32+ Erros Silenciosos em 12 Categorias

#### Cat. F1: Catch Blocks Vazios (4 issues)

| # | Arquivo:Linhas | Contexto | Risco | Ação |
|---|----------------|----------|-------|------|
| SF1 | `hooks/use-ml-link-lists.ts:406-408` | `catch { // ignore }` no clipboard | MEDIO: user pensa que copiou | Capturar + toast fallback |
| SF2 | `hooks/use-public-redirect.ts:141-147` | `catch { // ignore }` em iframe DOM | BAIXO: mask DOM errors | Capturar level Info |
| SF3 | `hooks/use-automations.ts:679-681` | `catch { // Failed to toggle }` | ALTO: toggle status falha silencioso | Capturar + toast |
| SF4 | `hooks/use-automations.ts:693-695` | `catch { // Failed to delete }` | CRITICO: delete falha invisível | Capturar + toast |

#### Cat. F2: Console-Only Logging sem Sentry (7+ issues)

| # | Arquivo | Contexto | Risco |
|---|---------|----------|-------|
| SF5 | `hooks/use-session-status-sse.ts:221-335` | 7 addEventListener com `console.warn('[SSE]...')` | SSE errors invisíveis |
| SF6 | `hooks/use-whatsapp.ts:195` | `console.warn('[WhatsApp] SSE error:')` | WhatsApp SSE invisível |
| SF7 | `hooks/use-whatsapp.ts:193` | `console.warn('[WhatsApp] Failed to update instance status:')` | Instance status desync |
| SF8 | `hooks/use-whatsapp.ts:851-860` | Polling loop `console.warn('[WhatsApp] Poll error:')` | Sync job falha silencioso |
| SF9 | `hooks/use-whatsapp.ts:221-288` | `console.warn('[WhatsApp] Failed to check initial status:')` | Init status invisível |
| SF10 | `hooks/use-history.ts:108` | `console.log('[DEBUG use-history]')` | Debug logs em produção |

#### Cat. F3: Group Messaging Catch Silenciosos (3 issues)

| # | Arquivo:Linhas | Contexto | Risco |
|---|----------------|----------|-------|
| SF11 | `hooks/use-groups-messaging.ts:79-81` | `catch { // Skip groups that fail }` em loop | Participantes incompletos |
| SF12 | `hooks/use-groups-messaging.ts:240-249` | FileReader error sem detalhes | Debug impossível |
| SF13 | `hooks/use-groups-messaging.ts:280-288` | Loop error sem captura | Razão do erro desconhecida |

#### Cat. F4: localStorage sem try/catch (CRITICO em private browsing)

| # | Arquivo:Linhas | Código | Risco |
|---|----------------|--------|-------|
| SF14 | `hooks/use-whatsapp.ts:80-83` | `localStorage.getItem('whatsapp-auto-sync')` | App CRASHA em private mode |
| SF15 | `hooks/use-whatsapp.ts:201-202` | `localStorage.setItem(...)` | Write falha em private mode |

#### Cat. F5: Promises Não Observadas em useEffect (2 issues - ALTO)

| # | Arquivo:Linhas | Código | Risco |
|---|----------------|--------|-------|
| SF16 | `hooks/use-whatsapp.ts:337-347` | `handleImportGroups(true)` sem await/catch | Rejection não observada |
| SF17 | `hooks/use-whatsapp.ts:341-344` | async setInterval sem catch externo | Rejection periódica silenciosa |

#### Cat. F6: Mutation Error Callbacks Incompletos (3+ issues)

| # | Arquivo:Linhas | Hook | Risco |
|---|----------------|------|-------|
| SF18 | `hooks/use-redirect-link.ts:240-246` | `onError: () => { toast(...) }` (sem error param) | Detalhes do erro perdidos |
| SF19 | `hooks/use-redirect-link.ts:269-275` | Update mutation onError sem Sentry | Falha invisível |
| SF20 | `hooks/use-redirect-link.ts:293-299` | Delete mutation onError sem Sentry | Delete falha invisível |
| SF21 | `hooks/use-settings-connect.ts:133-140` | onError extrai errorInfo mas não reporta | Settings error não rastreado |

#### Cat. F7: Clipboard API sem Fallback (1 issue)

| # | Arquivo:Linha | Código | Risco |
|---|---------------|--------|-------|
| SF22 | `hooks/use-redirect-link.ts:624` | `navigator.clipboard.writeText(link)` sem try | Falha silenciosa em HTTP/sem permissão |

#### Cat. F8: Type Assertions `as` Mascarando Erros (2 issues)

| # | Arquivo:Linhas | Código | Risco |
|---|----------------|--------|-------|
| SF23 | `hooks/use-session-status-sse.ts:232-233` | `JSON.parse(...) as SSEEvent` + `as SessionStatusData` | Contrato violado silenciosamente |
| SF24 | `hooks/use-redirect-link.ts:137-143` | Proto → local cast sem validação | Schema change invisível |

#### Cat. F9: URL Parsing sem try/catch (1 issue)

| # | Arquivo:Linhas | Código | Risco |
|---|----------------|--------|-------|
| SF25 | `hooks/use-ml-link-lists.ts:12-17` | `new URL(trimmed)` pode lançar SyntaxError | Caller trata com `catch { // keep raw }` mas não reporta |

#### Cat. F10: Affiliate Programs JSON Parse (2 issues)

| # | Arquivo:Linhas | Código | Risco |
|---|----------------|--------|-------|
| SF26 | `hooks/use-affiliate-programs.ts:301-306` | `JSON.parse(selected_brands)` com catch vazio | Dados corrompidos invisíveis |
| SF27 | `hooks/use-affiliate-programs.ts:189-200` | `JSON.parse(json)` em token decode | Token invalido sem report |

#### Cat. F11: Query Cache Invalidation (1 issue)

| # | Arquivo:Linhas | Código | Risco |
|---|----------------|--------|-------|
| SF28 | `hooks/use-history.ts:128-142` | `queryClient.invalidateQueries` erro mostrado como toast mas não rastreado | Cache refresh falha invisível |

#### Cat. F12: Lazy Loading sem ErrorBoundary Adequado (1 issue)

| # | Arquivo | Contexto | Risco |
|---|---------|----------|-------|
| SF29 | `app.tsx` | 15+ React.lazy() sem Sentry.ErrorBoundary no root | Chunk load failure = tela branca |

---

## 3rd Pass: Concurrency, Resilience & Runtime Edge Cases

> Varredura adicional da codebase sob 3 perspectivas novas: **concorrência/race conditions**, **resiliência de rede/infraestrutura**, e **edge cases de runtime do browser**. Descobertos **39 pontos de falha adicionais** não cobertos pelas análises anteriores.

### BACKEND - Concurrency & Race Conditions (C1-C10)

| # | Arquivo:Linha | Issue | Severidade | Phase |
|---|---------------|-------|------------|-------|
| C1 | `handlers/sse_handler.go:109` | `go hub.run()` sem recover() - crash no SSE hub mata real-time | CRITICO | 2 |
| C2 | `services/cron/cron_service.go:113,168` | `go s.runTask(task)` sem recover() - tarefas cron morrem silenciosamente | ALTO | 2 |
| C3 | `services/watchdog/watchdog.go:441-442` | Maps `reconnectAttempts`/`lastReconnectTime` lidos/escritos em goroutines - concurrent map panic | ALTO | 3 |
| C4 | `auth/tokens.go:17-40` | Race na init de tokenHMACKey - se init falha, tokens usam SHA-256 mais fraco silenciosamente | MEDIO | 3 |
| C5 | `adapters/whatsapp/waha_adapter.go:78-80` | Race no statusCache map se ClearStatusCache() concorre com status check | MEDIO | 3 |
| C6 | `adapters/whatsapp/waha_adapter.go:164-166` | http.Client sem custom Transport - sem MaxConnsPerHost, risco de exaustão de FD | MEDIO | 2 |
| C7 | `services/cron/cron_service.go:211-212` | Ticker por task sem Stop() confiável - leak em shutdown não gracioso | MEDIO | 3 |
| C8 | `handlers/sse_redis_bridge.go:109-115` | pubsub.Close() pode concorrer com leitura do channel - panic em canal fechado | MEDIO | 2 |
| C9 | `repository/postgres/db.go:47` | DB pool (25 max) sem monitoramento - exaustão silenciosa, queries travam | MEDIO | 1 |
| C10 | `services/watchdog/watchdog.go:549` | `go s.run(ctx)` sem recover() - crash do watchdog mata health checks | MEDIO | 2 |

### BACKEND - Network Resilience & Infrastructure (N1-N17)

| # | Arquivo | Issue | Severidade | Phase |
|---|---------|-------|------------|-------|
| N1 | `adapters/whatsapp/waha_adapter.go:410,1158,1228` | io.ReadAll sem MaxBytesReader - bomba de memória com mídia grande | ALTO | 2 |
| N2 | `adapters/whatsapp/waha_adapter.go:151,194` | Timeout global único de 30s, sem deadline por operação | MEDIO | 2 |
| N3 | `adapters/whatsapp/waha_adapter.go:202-211` | Sem retry em erros transientes de rede (DNS, connection reset) | ALTO | 2 |
| N4 | `handlers/waha_webhook_handler.go:127` | Endpoint webhook Kiwify sem limite de tamanho de request (POST 1GB = OOM) | CRITICO | 2 |
| N5 | `services/ai/ai_service.go:49-51` | Timeout de 60s, sem circuit breaker - todas requests bloqueiam durante outage de AI | MEDIO | 2 |
| N6 | `services/mercadolivre/ml_service.go` | Sem limite de tamanho de resposta na API ML - OOM com respostas de catálogo grandes | ALTO | 2 |
| N7 | `repository/postgres/db.go:117-119` | Erros de commit de transação nem sempre propagados quando usando BeginTx diretamente | MEDIO | 3 |
| N8 | `repository/postgres/db.go:135-138` | Sem statement_timeout default no pool - queries podem bloquear indefinidamente | ALTO | 1 |
| N9 | `repository/postgres/webhook_events_repository.go:198` | GetPendingEvents com parâmetro limit não validado - 0 = sem limite = full table scan | MEDIO | 3 |
| N10 | `handlers/sse_redis_bridge.go:96-112` | Pub/Sub subscription não reconecta em desconexão do Redis | ALTO | 2 |
| N11 | `cmd/server/main.go:862-879` | Goroutines de workers não rastreadas com WaitGroup - writes parciais no shutdown | MEDIO | 1 |
| N12 | `cmd/server/main.go:114-135` | Redis client não fechado se conexão DB falha (leak de recurso em falha de startup) | MEDIO | 1 |
| N13 | `cmd/server/main.go:870-871` | cronService.Stop() não aguardado - jobs em execução crasham após DB fechar | MEDIO | 1 |
| N14 | `internal/config/config.go` | Env vars obrigatórias (POSTGRES_HOST, REDIS_URI) não validadas no startup | MEDIO | 1 |
| N15 | `internal/lib/cache/cache.go:29,64-89` | sync.Map local cache sem tamanho máximo - cresce indefinidamente, eventual OOM | MEDIO | 3 |
| N16 | `cmd/server/main.go:480-484` | Sem rate limit middleware HTTP-level - endpoints webhook podem sofrer DoS | ALTO | 1 |
| N17 | Repository layer (todos) | Sem detecção de deadlock (PG error 40P01) / lógica de retry | MEDIO | 3 |

### FRONTEND - Runtime & Browser Edge Cases (FR1-FR12)

| # | Arquivo:Linhas | Issue | Severidade | Phase |
|---|----------------|-------|------------|-------|
| FR1 | `pages/groups.tsx`, `pages/dashboard.tsx` etc. | Sem ErrorBoundary em seções críticas das páginas (charts, tabelas) | ALTO | 4 |
| FR2 | `hooks/use-session-status-sse.ts:273-334` | window.dispatchEvent() nos listeners SSE - se listener lança exceção, erro unhandled | ALTO | 5 |
| FR3 | `hooks/use-session-status-sse.ts:220-304` | Múltiplos addEventListener sem removeEventListener - memory leak em reconnect | MEDIO | 5 |
| FR4 | `app.tsx:39-45` | QueryClient sem callback onError global - queries/mutations falhas só vão pro Sentry se hook tratar | CRITICO | 4 |
| FR5 | `hooks/use-redirect-link.ts`, `hooks/use-automations-connect.ts` | useMutation sem onError em vários hooks | ALTO | 5 |
| FR6 | `hooks/use-whatsapp.ts:129,160,413` | Múltiplas chamadas refetch concorrendo de SSE/polling/manual - flickering na UI, dados stale | MEDIO | 5 |
| FR7 | `app.tsx:18-37` | React.lazy() chunk load failure (ChunkLoadError) não capturado pelo Suspense | CRITICO | 4 |
| FR8 | - | Sem detecção de version mismatch após deploy (chunks stale de build antigo) | ALTO | 6 |
| FR9 | `components/groups/group-photo-dialog.tsx:72-78` | File upload sem validação de tamanho client-side | MEDIO | 5 |
| FR10 | Todas páginas com forms | Sem estado disabled em botões submit durante mutation pendente - double submit | MEDIO | 5 |
| FR11 | Config i18n | Chaves de tradução missing retornam key string como fallback silenciosamente - sem error reporting | MEDIO | 5 |
| FR12 | Todas páginas com forms | Navegação durante mutation pendente - estado perdido, update otimista órfão | MEDIO | 5 |

---

## 4th Pass: SonarQube Deep Static Analysis (103 novos pontos)

> Varredura exaustiva estilo SonarQube com configuração máxima de rigor em 20+ categorias: erros suprimidos, goroutines sem recover, type assertions inseguras, fraquezas criptográficas, time.Parse silenciosos, JSON marshal/unmarshal suprimidos, catch blocks não reportados, promises sem catch, type assertions `as`, localStorage/clipboard/DOM sem guard, e mais. Descobertos **68 pontos no backend Go** e **35 pontos no frontend React** não cobertos pelas análises anteriores.

### BACKEND GO - Erros Suprimidos (`_ =`) em Arquivos Não Cobertos (SQ1-SQ41)

#### SQ1-SQ10: Services AI + Mercado Livre (10 issues)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ1 | `services/ai/ai_service.go:316` | `jsonPayload, _ := json.Marshal(payload)` | ALTO | Capturar - corpo de request AI silenciosamente nil se marshal falhar, causando POST vazio |
| SQ2 | `services/ai/ai_service.go:331` | `body, _ := io.ReadAll(resp.Body)` | ALTO | Capturar - resposta AI perdida silenciosamente, mensagem de erro usa body vazio |
| SQ3 | `services/mercadolivre/ml_service.go:167` | `jsonPayload, _ := json.Marshal(payload)` | MEDIO | Capturar - corpo de request ML affiliate silenciosamente vazio |
| SQ4 | `services/mercadolivre/ml_service.go:187` | `body, _ := io.ReadAll(resp.Body)` | MEDIO | Capturar - resposta ML perdida silenciosamente |
| SQ5 | `services/mercadolivre/ml_service.go:313` | `body, _ := io.ReadAll(resp.Body)` | MEDIO | Capturar - HTML de preview de produto perdido silenciosamente |
| SQ6 | `services/mercadolivre/ml_service.go:280` | `creds, _ := s.LoadCredentials(ctx, userID)` | MEDIO | Capturar - credenciais nil, requests ML saem sem autenticação |
| SQ7 | `services/mercadolivre/ml_service.go:116` | `_ = s.mlLinkRepo.Create(ctx, ...)` | BAIXO | Capturar Warning - entrada ML link não persistida, dedup falha na próxima chamada |
| SQ8 | `services/mercadolivre/ml_service.go:127` | `_ = s.cache.SetJSON(ctx, cacheKey, result, linkCacheTTL)` | BAIXO | Capturar Warning - cache ML link não setado, toda chamada vai para API externa |
| SQ9 | `services/mercadolivre/ml_service.go:308` | `_ = s.cache.SetJSON(ctx, cacheKey, emptyPreview, ...)` | BAIXO | Capturar Warning - cache de preview vazio não setado |
| SQ10 | `services/mercadolivre/ml_service.go:326` | `_ = s.cache.SetJSON(ctx, cacheKey, preview, ttl)` | BAIXO | Capturar Warning - preview cache não setado |

#### SQ11-SQ14: Handlers WhatsApp (4 issues)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ11 | `handlers/whatsapp_handler.go:182` | `_ = h.deps.InstanceRepo.UpdateInstanceStatus(ctx, instance.ID, "pending")` | ALTO | Capturar - status de instância desincronizado: DB mostra status antigo enquanto WAHA reconecta |
| SQ12 | `handlers/whatsapp_handler.go:226` | `_ = h.deps.InstanceRepo.UpdateInstanceStatus(ctx, instance.ID, dbStatus)` | ALTO | Capturar - resultado de status check não persistido, frontend mostra dados stale |
| SQ13 | `handlers/whatsapp_handler.go:237` | `qrData, _ := h.deps.WhatsAppProvider.GetQRCode(...)` | MEDIO | Capturar - falha na busca de QR code invisível; usuário vê "sem QR" sem explicação |
| SQ14 | `handlers/whatsapp_handler.go:459` | `_ = h.deps.Backpressure.RecordEnqueue(...)` | MEDIO | Capturar - métrica de backpressure perdida (handler diferente de S74/S75) |

#### SQ15-SQ26: ConnectRPC Admin Handler (12 issues - NOVO arquivo)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ15 | `connectrpc/automations_handler.go:232` | `_ = h.automationRepo.UpdateAutomation(ctx, automationID, ...)` | ALTO | Capturar - metadados de erro (last_error, last_error_at) não persistidos, erro invisível na UI |
| SQ16 | `connectrpc/groups_handler.go:1309` | `_ = h.groupRepo.DeleteGroup(ctx, group.ID, user.ID)` | MEDIO | Capturar - limpeza de grupo no DB após LeaveGroup falha, registros órfãos |
| SQ17 | `services/waha/waha_service.go:263` | `_, _ = s.doRequest(ctx, "POST", ".../stop", nil)` | MEDIO | Capturar - stop de sessão WAHA falha durante reconexão, estado da sessão pode corromper |
| SQ18 | `connectrpc/admin_handler.go:457` | `profile, _ = h.profileRepo.GetProfile(ctx, log.UserID)` | MEDIO | Capturar - enriquecimento de auditoria admin falha; perfis exibidos como nil |
| SQ19 | `connectrpc/admin_handler.go:736-762` | `logEntry, _ := h.webhookLogWriteRepo.Create(...)` + `_ = h.webhookLogWriteRepo.UpdateStatus(...)` (4x) | MEDIO | Capturar - logs de SimulateKiwifyWebhook admin silenciosamente perdidos (espelho de S62-S65 mas handler diferente) |
| SQ20 | `connectrpc/admin_handler.go:865` | `_ = h.systemEventRepo.Create(ctx, "admin_reset_password", ...)` | ALTO | Capturar - trilha de auditoria de reset de senha admin perdida |
| SQ21 | `connectrpc/admin_handler.go:877` | `_ = h.emailService.SendCustomEmail(...)` | ALTO | Capturar - email de reset de senha não enviado mas admin pensa que foi |
| SQ22 | `connectrpc/admin_handler.go:1476,1557,1614,1919` | `roles, _ := h.userRoleRepo.GetAllUserRoles(ctx, ...)` (4x) | MEDIO | Capturar - roles de usuário silenciosamente vazias; admin vê usuários sem roles |
| SQ23 | `connectrpc/admin_handler.go:1507` | `existing, _ := h.profileRepo.GetProfileByEmail(ctx, email)` | ALTO | Capturar - checagem de email duplicado falha, pode criar usuários duplicados |
| SQ24 | `connectrpc/admin_handler.go:1693` | `_, _ = h.actionHistoryRepo.Create(...)` | MEDIO | Capturar - trilha de auditoria de unlock de conta admin perdida |
| SQ25 | `connectrpc/admin_handler.go:1704,1975,2061,2141,2153` | `_ = h.systemEventRepo.Create(ctx, "admin_*", ...)` (5x) | MEDIO | Capturar - múltiplas trilhas de auditoria de ações admin perdidas |
| SQ26 | `connectrpc/admin_handler.go:2166,2170` | `updatedProfile, _ := h.profileRepo.GetProfile(...)` + `roles, _ := ...` | MEDIO | Capturar - resposta de bloqueio de usuário mostra dados stale em falha de fetch |

#### SQ27-SQ32: Jobs Service - History/Status Silenciosos (8 issues)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ27 | `services/jobs/jobs_service.go:198` | `_ = s.historyRepo.Update(ctx, history.ID, ...)` | ALTO | Capturar - target_count e status running de job nunca setados no history |
| SQ28 | `services/jobs/jobs_service.go:224-225` | `_ = s.historyRepo.UpdateCounts(...)` + `_ = s.historyRepo.UpdateStatus(...)` | ALTO | Capturar - counts/status de conclusão de job nunca persistidos |
| SQ29 | `services/jobs/jobs_service.go:235` | `_ = s.historyRepo.MarkFailed(...)` | ALTO | Capturar - falha de job nunca registrada no history |
| SQ30 | `services/jobs/jobs_service.go:449,469,489,499` | `_ = s.jobRepo.UpdateStatus(...)` (4x) | ALTO | Capturar - updates de status de job silenciosamente perdidos; jobs ficam presos em "running" |
| SQ31 | `services/jobs/jobs_service.go:671,679,687,703,711,717` | `_ = s.scheduledJobRepo.UpdateJobStatus(...)` (6x) | ALTO | Capturar - updates de status de scheduled job silenciosamente perdidos |
| SQ32 | `services/jobs/jobs_service.go:721` | `_ = s.automationRepo.UpdateAutomation(...)` | MEDIO | Capturar - next_run_at/last_run_at de automação não atualizado após scheduled job completar |

#### SQ33-SQ38: Auth + Workers - Erros Adicionais (6 issues)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ33 | `auth/service.go:476` | `_, _ = s.sessionStore.RevokeAllExcept(ctx, session.UserID, sessionID)` | ALTO | Capturar CRITICO - após troca de senha, outras sessões NÃO revogadas = falha de segurança |
| SQ34 | `auth/service.go:495` | `_ = s.markEmailVerificationTokenUsed(ctx, HashToken(token))` | MEDIO | Capturar - token de verificação de email reutilizável |
| SQ35 | `auth/service.go:1080` | `_, _ = s.rateLimiter.Increment(ctx, rateLimitKey)` | ALTO | Capturar CRITICO - rate limiter de magic link bypassado silenciosamente |
| SQ36 | `auth/service.go:1118` | `_ = s.markMagicLinkTokenUsed(ctx, HashToken(token))` | MEDIO | Capturar - token magic link reutilizável |
| SQ37 | `auth/service.go:1456` | `_ = a.client.Update("sessions", nullUpdates).Execute(ctx, nil)` | ALTO | Capturar - workaround de RevokeSessionsExcept falha silenciosamente; sessão alvo também revogada |
| SQ38 | `workers/message_worker.go:383` | `_ = w.dispatchLogRepo.Update(ctx, *dispatchLogID, updates)` | MEDIO | Capturar - update de status do dispatch log worker perdido |

#### SQ39-SQ41: Misc Suppressed Errors (3 issues)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ39 | `connectrpc/history_handler.go:219` | `metadata, _ = structpb.NewStruct(detailsMap)` | BAIXO | Capturar - metadados de evento history silenciosamente nil; API retorna eventos sem detalhes |
| SQ40 | `connectrpc/admin_handler.go:796` | `meta, _ := structpb.NewStruct(e.Payload)` | BAIXO | Capturar - metadados de evento sistema silenciosamente nil na API admin |
| SQ41 | `lib/urlsafety/urlsafety.go:254` | `_ = s.cache.SetJSON(ctx, cacheKey, ...)` | BAIXO | Capturar Warning - cache de URL safety não setado; toda checagem vai para API externa |

### BACKEND GO - Goroutines sem recover() (SQ42-SQ49)

| # | Arquivo:Linha | Contexto | Severidade | Ação Sentry |
|---|---------------|----------|------------|-------------|
| SQ42 | `services/mercadolivre/ml_service.go:151` | `go func() { _ = s.SaveCredentials(context.Background(), ...) }()` | ALTO | SafeGo - panic em SaveCredentials mata goroutine silenciosamente, update de credencial perdido |
| SQ43 | `services/mercadolivre/ml_service.go:212` | `go func() { _ = s.SaveCredentials(context.Background(), ...) }()` | ALTO | SafeGo - mesmo issue no path de retry de refresh CSRF |
| SQ44 | `auth/service.go:162` | `go func() { ... HashPassword ... updatePassword ... }()` | CRITICO | SafeGo - panic na goroutine de migração bcrypt→argon2id mata silenciosamente; migração de senha nunca acontece |
| SQ45 | `auth/service.go:249` | `go func() { ... sendVerificationEmail ... }()` | ALTO | SafeGo - panic no envio de email de verificação mata goroutine silenciosamente |
| SQ46 | `auth/service.go:395` | `go func() { ... sendPasswordResetEmail ... }()` | ALTO | SafeGo - panic no envio de email de reset de senha mata goroutine silenciosamente |
| SQ47 | `auth/service.go:1083` | `go func() { ... sendMagicLinkEmail ... }()` | ALTO | SafeGo - panic no envio de email de magic link mata goroutine silenciosamente |
| SQ48 | `adapters/email/smtp_adapter.go:121` | `go func() { ... sendWithSSL/sendWithSTARTTLS ... }()` | MEDIO | SafeGo - panic no envio SMTP causa leak; canal de erro nunca recebe |
| SQ49 | `cmd/server/main.go:1099` | `go func() { queueServer.Start() }()` | MEDIO | SafeGo - panic no Asynq queue server não tem recover (usa Fatal mas sem Sentry flush) |

### BACKEND GO - Type Assertions Inseguras (SQ50-SQ51)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ50 | `auth/rate_limiter.go:195` | `count := int(result.(int64))` | CRITICO | Adicionar ok check - Redis Eval pode retornar tipo inesperado; panic de assertion crasha rate limiter inteiro |
| SQ51 | `connectrpc/loadbalancer_handler.go:81,171,218` | `serverID, _ := strconv.ParseInt(srv.ID, 10, 32)` (3x) | MEDIO | Validar + capturar - IDs de servidor silenciosamente viram 0 em string inválida |

### BACKEND GO - Fraquezas Criptográficas/Segurança (SQ52-SQ56)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ52 | `services/monitoring/monitoring_service.go:9,353,395` | `import "math/rand"` + `md5.Sum([]byte(...))` | ALTO | Substituir MD5 por SHA-256 para dedup - MD5 é suscetível a colisões; math/rand não é CSPRNG |
| SQ53 | `services/automation/process_list.go:6` | `import "math/rand"` | MEDIO | Substituir por crypto/rand - math/rand usado para randomização de delay de mensagem |
| SQ54 | `services/automation/automation_service.go:6` | `import "math/rand"` | MEDIO | Substituir por crypto/rand - math/rand usado na execução de automação |
| SQ55 | `workers/asynq.go:8` | `import mathrand "math/rand"` | MEDIO | Substituir por crypto/rand - math/rand usado no jitter de retry do worker |
| SQ56 | `services/watchdog/watchdog.go:21` | `import "math/rand"` | MEDIO | Substituir por crypto/rand - math/rand usado no backoff do watchdog |

### BACKEND GO - time.Parse Silenciosos (SQ57-SQ60)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ57 | `auth/service.go:608,642` | `t, _ := time.Parse(time.RFC3339, *creds[0].LockedUntil)` (2x) | ALTO | Validar + capturar - LockedUntil vira zero time = conta nunca fica bloqueada (bypass de segurança) |
| SQ58 | `auth/service.go:685` | `createdAt, _ := time.Parse(time.RFC3339, profiles[0].CreatedAt)` | BAIXO | Capturar - createdAt do usuário vira zero, cálculos de período trial errados |
| SQ59 | `auth/service.go:898,957,1179` | `expiresAt, _ := time.Parse(time.RFC3339, token.ExpiresAt)` (3x) | ALTO | Validar + capturar - expiração de token vira zero time = tokens expiram imediatamente OU nunca (comportamento indefinido) |
| SQ60 | `auth/service.go:1315-1397` | Múltiplos `_, _ := time.Parse(time.RFC3339, ...)` em listagem de sessões (8x) | BAIXO | Capturar - timestamps de sessão viram zero em respostas admin/API |

### BACKEND GO - WAHA Models JSON Unmarshal Suprimido (SQ61)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ61 | `adapters/whatsapp/waha_models.go:105-251` | 30+ linhas de `_ = json.Unmarshal(v, &g.Field)` | ALTO | Capturar ao menos campos críticos (ID, Participants) - dados de grupo silenciosamente parciais/vazios se API WAHA mudar formato |

### BACKEND GO - Repository JSON Marshal/Unmarshal Suprimido (SQ62-SQ67)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ62 | `repository/postgres/automation_repository.go:111-120` | 7x `*JSON, _ := json.Marshal(data.*)` (groupIDs, sendGroups, monitorGroupIDs, categories, stores, scheduleDays, monitoringRules) | ALTO | Capturar - criação de automação silenciosamente armazena NULL/JSON vazio para todos campos array |
| SQ63 | `repository/postgres/automation_repository.go:246` | `groupIDJSON, _ := json.Marshal([]string{waGroupID})` | MEDIO | Capturar - groupID de automação single-group silenciosamente NULL |
| SQ64 | `repository/postgres/automation_repository.go:286-382` | 14x `_ = json.Unmarshal(*JSON, &a.*)` em funções de scan | MEDIO | Capturar - dados de automação silenciosamente incompletos quando lidos do DB |
| SQ65 | `repository/postgres/system_event_repository.go:31` | `payloadJSON, _ = json.Marshal(payload)` | MEDIO | Capturar - payload de evento sistema silenciosamente NULL |
| SQ66 | `repository/postgres/webhook_log_repository.go:50` | `payloadJSON, _ = json.Marshal(payload)` | MEDIO | Capturar - payload de webhook log silenciosamente NULL |
| SQ67 | `repository/postgres/action_history_repository.go:34,138,188,326,445` | 5x `_ = json.Unmarshal(detailsJSON, &details)` em paths de leitura | MEDIO | Capturar - detalhes de action history silenciosamente nil em JSON corrompido |

### BACKEND GO - Rate Limiting em Password Service (SQ68)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| SQ68 | `services/profile/password_service.go:143-144` | `_ = s.rateLimitRepo.Increment(ctx, input.Email, "email")` + `_ = s.rateLimitRepo.Increment(ctx, input.IPAddress, "ip")` | ALTO | Capturar CRITICO - rate limiting de password reset silenciosamente desabilitado em falha Redis |

---

### FRONTEND REACT - Root Mount & ErrorBoundary (FSQ1, FSQ14)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ1 | `main.tsx:6` | `createRoot(document.getElementById('root')!)` | ALTO | Wrappear em null check; se `#root` ausente, `Sentry.captureException(new Error('Root element not found'))` antes do crash |
| FSQ14 | `components/error-boundary.tsx:27-29` | `getDerivedStateFromError` existe mas `componentDidCatch` NÃO implementado | CRITICO | ErrorBoundary captura erros para mostrar fallback UI mas NUNCA reporta - implementar `componentDidCatch` com `Sentry.captureException` |

### FRONTEND REACT - Clipboard API Não Protegida (FSQ2-FSQ3)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ2 | `hooks/use-coupons.ts:543` | `navigator.clipboard.writeText(code)` sem try/catch | MEDIO | Wrappear em try/catch; capturar com breadcrumb "clipboard_failed" e toast fallback |
| FSQ3 | `components/admin/dashboard/webhook-section.tsx:65` | `navigator.clipboard.writeText(webhookUrl)` sem try/catch | MEDIO | Mesmo - cópia de URL webhook admin falha silenciosamente em contextos inseguros |

### FRONTEND REACT - Base64/Crypto/Parsing Sem Guard (FSQ4-FSQ5)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ4 | `hooks/use-affiliate-programs.ts:178` | `return atob(s + pad)` dentro de `base64UrlDecode()` | ALTO | `atob()` lança `DOMException` em caracteres inválidos; catch externo não reporta ao Sentry |
| FSQ5 | `hooks/use-public-redirect.ts:187` | `document.execCommand('copy')` - API deprecated que retorna boolean | BAIXO | Checar valor de retorno; se false, `Sentry.captureMessage('execCommand copy failed', 'warning')` |

### FRONTEND REACT - Export CSV / Data Operations (FSQ7)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ7 | `hooks/use-automation-logs.ts:182-217` | `exportCSV()` sem try/catch; `new Blob()`, `URL.createObjectURL()`, `format(new Date(log.created_at))` podem todos lançar | MEDIO | Wrappear função inteira em try/catch; `Sentry.captureException` no catch; `format()` com `created_at` malformado lança erro |

### FRONTEND REACT - Auth Context Gaps (FSQ15-FSQ17)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ15 | `contexts/auth-context.tsx:324` | `} catch { // Ignore errors on logout }` | BAIXO | Capturar Warning - logout RPC falha mas sessão server persiste |
| FSQ16 | `contexts/auth-context.tsx:403` | `} catch { return { sessions: [], currentSessionId: '' }; }` | MEDIO | Capturar Warning - listSessions engole todos erros; usuário vê lista de sessões vazia sem indicação de erro |
| FSQ17 | `contexts/auth-context.tsx:248-258` | Catch block só trata `ConnectError` com `Code.Unauthenticated`; todos outros erros engolidos silenciosamente | ALTO | Capturar Warning para erros não-Unauthenticated - se backend está down durante init, zero dados diagnósticos |

### FRONTEND REACT - SSE Token em URL (Security Hotspot) (FSQ18)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ18 | `hooks/use-session-status-sse.ts:183` | `` `${apiUrl}/api/v1/sse/events?token=${encodeURIComponent(session.accessToken)}` `` | ALTO | Token de acesso passado como parâmetro de URL query - aparece em histórico do browser, logs de servidor, logs de proxy, e breadcrumbs Sentry. Adicionar `beforeBreadcrumb` para scrub de tokens em URLs SSE |

### FRONTEND REACT - Event Handler Edge Cases (FSQ9-FSQ12)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ9 | `hooks/use-whatsapp.ts:377` | `customEvent.detail.data` - desestruturação de CustomEvent detail sem null check | MEDIO | Se `customEvent.detail` é undefined, acessar `.data` lança. Adicionar check defensivo + `Sentry.captureMessage('SSE event with missing detail')` |
| FSQ10 | `hooks/use-jobs.ts:389` | `description: (error as Error).message` | MEDIO | Se error não é instância de Error, `.message` retorna undefined. Usar pattern `error instanceof Error` + `Sentry.captureException` |
| FSQ11 | `hooks/use-jobs.ts:426` | `description: (error as Error).message` | MEDIO | Mesmo pattern em `handleDeleteJob` |
| FSQ12 | `hooks/use-whatsapp.ts:817` | `setInterval(async () => { ... }, 1000)` dentro de callback `handleImportGroups` | ALTO | setInterval criado fora de useEffect; se componente desmonta enquanto polling ativo, intervalo continua disparando em state stale |

### FRONTEND REACT - Catch Blocks Não Reportados (FSQ22-FSQ31)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ22 | `hooks/use-whatsapp.ts:491` | `} catch { // ignore }` em refreshQrCode (deleteInstance) | BAIXO | Adicionar breadcrumb `qr_refresh_delete_failed` |
| FSQ23 | `hooks/use-whatsapp.ts:711` | `} catch { // ignore delete errors }` em handleConnect | BAIXO | Adicionar breadcrumb |
| FSQ24 | `hooks/use-whatsapp.ts:916-918` | `} catch { // Session may already be gone }` em disconnectInstance | BAIXO | Adicionar breadcrumb |
| FSQ25 | `hooks/use-whatsapp.ts:925` | `} catch { // Instance may already be deleted }` em disconnectInstance | BAIXO | Adicionar breadcrumb |
| FSQ26 | `hooks/use-dispatch-texts.ts:172-173` | `} catch { // Clipboard API not available }` | BAIXO | Adicionar breadcrumb clipboard |
| FSQ27 | `hooks/use-ml-link-lists.ts:429-431` | `catch { // keep raw }` em addOneLink normalizeUrl | BAIXO | Adicionar captura Info para URL normalization |
| FSQ28 | `hooks/use-ml-link-lists.ts:499-501` | `catch { return l; }` em importBulk normalizeUrl map | BAIXO | Mesmo pattern em bulk import |
| FSQ29 | `hooks/use-affiliate-credentials-connect.ts:121-123` | `catch { parsedCredentials = {}; }` | MEDIO | JSON parse de credenciais falha retorna objeto vazio; adicionar `Sentry.captureMessage('credential_json_parse_failed', 'warning')` |
| FSQ30 | `hooks/use-public-redirect.ts:159-161` | `} catch { // ignore }` em redirect intent Android | BAIXO | Adicionar breadcrumb |
| FSQ31 | `hooks/use-public-redirect.ts:197` | `} catch { window.location.href = redirectUrl; }` em openInviteLink | BAIXO | Adicionar breadcrumb |

### FRONTEND REACT - Misc Security/Runtime (FSQ6, FSQ8, FSQ13, FSQ19-FSQ21, FSQ32-FSQ35)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| FSQ6 | `hooks/use-public-redirect.ts:138-143` | setTimeout cleanup com `catch { // ignore }` | BAIXO | Capturar Info se falhas repetidas indicam issues CSP/DOM |
| FSQ8 | `hooks/use-ml-link-lists.ts:14` | `new URL(trimmed)` em `normalizeUrl()` | ALTO | Caller trata com `catch { // keep raw }` mas não reporta. Adicionar captura Info |
| FSQ13 | `services/api.service.ts:145` | `const response: any = await api.post('/redirect', { slug })` | MEDIO | Tipo `any` na resposta; mudança no formato backend quebra redirect page silenciosamente |
| FSQ19 | `contexts/auth-context.tsx:30-38` | `getCsrfTokenFromCookie()` split com `=` incorreto | BAIXO | Se cookie value contém `=`, token CSRF truncado. Adicionar breadcrumb quando token é encontrado/não encontrado |
| FSQ20 | `hooks/use-redirect-link.ts:314` | `Math.floor(Math.random() * chars.length)` para geração de slug | BAIXO | Security hotspot: `Math.random()` é previsível; slugs podem ser adivinhados/enumerados. Usar `crypto.getRandomValues()` |
| FSQ21 | `hooks/use-whatsapp.ts:402-408` | `(window as unknown as { __importFallbackTimeout?: NodeJS.Timeout })` | BAIXO | Mutação global insegura de window para timeout IDs |
| FSQ32 | `lib/connect-transport.ts:45` | `crypto.getRandomValues(array)` em `generateTraceId()` | BAIXO | Sem try/catch; edge-case environments (WebView antigo, SSR) crasham inicialização do transport |
| FSQ33 | `contexts/auth-context.tsx:192-199` | `screen.width`, `screen.height`, `screen.colorDepth` | BAIXO | Em SSR ou ambientes de teste, `screen` pode ser undefined |
| FSQ34 | `hooks/use-automation-logs.ts:196` | `format(new Date(log.created_at), 'dd/MM/yyyy HH:mm:ss')` em exportCSV | MEDIO | Se `created_at` é string vazia/inválida, `new Date()` retorna Invalid Date e `format()` lança |
| FSQ35 | `components/admin/hooks/use-plans-tab.ts:228` | `await deletePlanMutation.mutateAsync(...)` sem try/catch | MEDIO | `mutateAsync` re-lança após `onError`; rejeição propaga para nível de componente não capturada |

---

## 5th Pass: WAHA Plus GoWS Integration (84 novos pontos)

> Análise profunda de TODAS as interações com WAHA Plus (WhatsApp HTTP API): adapter Go, handlers, workers, services, SSE, Redis bridge, frontend hooks, componentes de grupo, dialogs de mídia. Foco em erros silenciosos, PII em logs, segurança de webhook, e resiliência de conexão.

### BACKEND GO - 44 Pontos WAHA (W1-W44)

#### Cat. A: WAHA Adapter HTTP Layer

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W1 | `adapters/whatsapp/waha_models.go:105-190` | 30+ `json.Unmarshal` com `_ =` | MEDIO | Capturar campos críticos (ID, status, participants) via `sentry.CaptureMessage` level Warning |
| W2 | `adapters/whatsapp/waha_adapter.go:784-789` | `parseMemberOperationResult` retorna success para body vazio | ALTO | Capturar false positive com `sentry.CaptureException` + tag `{flow: "group-member-operation"}` |
| W3 | `adapters/whatsapp/waha_adapter.go:561-586` | Promote/Demote goroutine sem recover | BAIXO | Usar SafeGo wrapper |
| W4 | `adapters/whatsapp/waha_adapter.go:424-425` | `json.Unmarshal(resp, &result)` erro suprimido em GetGroupParticipants | BAIXO | Adicionar capture level Warning |
| W5 | `adapters/whatsapp/waha_adapter.go:214` | WAHA API retorna 502/503 sem retry | MEDIO | Capturar via `sentry.CaptureException` + tag `{external: "waha", status_code}` |
| W6 | `services/waha/waha_service.go:263` | `_ = s.provider.StopSession(ctx, session)` | BAIXO | Adicionar breadcrumb para session stop failure |

#### Cat. B: WAHA Session Lifecycle

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W7 | `services/whatsapp/instance_service.go:234` | `time.Sleep(500ms)` bloqueia goroutine em ReconnectInstance | BAIXO | Usar `time.After` + `ctx.Done()` select |
| W8 | `services/whatsapp/instance_service.go:252` | `return nil // Logout succeeded, just couldn't update DB` | MEDIO | `sentry.CaptureException` + tag `{module: "instance-service", flow: "logout"}` |
| W9 | `services/whatsapp/instance_service.go:147-151` | Cleanup de session WAHA falha após DB error | MEDIO | `sentry.CaptureException` + tag `{flow: "create-cleanup"}` para session órfã |
| W10 | `cmd/server/main.go` (inteiro) | Zero `sentry.Init()` no main.go | ALTO | Adicionar `sentry.Init()` no topo de main() + `defer sentry.Flush(2*time.Second)` |
| W11 | `cmd/server/main.go:419-421` | Redis SSE bridge Subscribe failure apenas log Warn | MEDIO | `sentry.CaptureMessage` level Error - SSE quebrado para todos |
| W12 | `cmd/server/main.go:1017-1018` | Backfill startup failure apenas log Error | MEDIO | `sentry.CaptureException` + tag `{module: "backfill", flow: "startup"}` |

#### Cat. C: WAHA Webhook Processing

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W13 | `handlers/waha_webhook_handler.go:131-132` | `if err.Error() == "http: request body too large"` string match | BAIXO | Usar `errors.As(&http.MaxBytesError{})` |
| W14 | `handlers/waha_webhook_handler.go:341-343` | `generateFallbackEventID` baseado em timestamp pode colidir | BAIXO | Adicionar componente random (crypto/rand) |
| W15 | `workers/webhook_consumer_worker.go:531-578` | `isRetryableError` usa substring matching frágil | MEDIO | Refatorar para usar ProviderError.Code + adicionar breadcrumb |
| W16 | `workers/webhook_consumer_worker.go:386-419` | handleMessageAck/Reaction são log-only, sem persistência | BAIXO | Não é Sentry; tracking como feature enhancement |

#### Cat. D: WAHA Message Operations

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W17 | `workers/message_worker.go` | Timeout tratado como "assumed sent" sem Sentry | ALTO | `sentry.CaptureMessage` level Warning + tag `{flow: "message-send", reason: "timeout-assumed-sent"}` |
| W18 | `workers/message_worker.go` | Dispatch log creation `_, _ =` silencioso | MEDIO | `sentry.CaptureException` + tag `{module: "message-worker", flow: "dispatch-log"}` |
| W19 | `workers/message_worker.go` | `time.Sleep(1500ms)` entre voice + text sends | BAIXO | Usar `time.After` com `ctx.Done()` check |
| W20 | `services/whatsapp/message_service.go` | `_ = s.historyRepo.Update(...)` silencioso | MEDIO | `sentry.CaptureMessage` level Warning quando history updates falham |

#### Cat. E: WAHA Group Operations

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W21 | `workers/groups_sync_worker.go:131` | N+1 API calls para GetGroupParticipants (200+ grupos) | MEDIO | Adicionar breadcrumb para cada fetch failure + capturar rate limit |
| W22 | `workers/groups_sync_worker.go:114-219` | Loop de processamento não checa `ctx.Done()` | MEDIO | Adicionar `select { case <-ctx.Done(): return ctx.Err() }` no loop |
| W23 | `services/whatsapp/groups_service.go:238-244` | DB update após WAHA success logado como Warn only (3 locais) | MEDIO | `sentry.CaptureException` + tag `{module: "groups-service", flow: "db-sync"}` |
| W24 | `services/whatsapp/groups_service.go:399-406` | LeaveGroup ignora DB lookup + delete errors | BAIXO | Log DB errors; não crítico pois user já saiu |

#### Cat. F: WAHA Instance Management

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W25 | `cmd/server/main.go:439` | `wahaAdapter` pode ser nil quando WAHA não configurado | MEDIO | Nil check em handler boundaries ou NullObject pattern |
| W26 | `config/config.go:265-268` | `validateConfig` é no-op; WAHA URL/key não validados | BAIXO | Adicionar validação de campos críticos no startup |
| W27 | `config/config.go:54` | `WebhookSecret` vazio desabilita HMAC silenciosamente | MEDIO | `sentry.CaptureMessage` level Error se vazio em produção |

#### Cat. G: WAHA Real-time Events (SSE)

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W28 | `handlers/sse_handler.go:163-170` | Client channel full → evento droppado silenciosamente | MEDIO | `sentry.CaptureMessage` level Warning com rate-limiting (1x/5min/user) |
| W29 | `handlers/sse_handler.go:221-224` | Broadcast channel full (buffer=256) → TODOS eventos droppados | ALTO | `sentry.CaptureMessage` level Critical - falha sistêmica |
| W30 | `handlers/sse_redis_bridge.go:251` | `context.Background()` sem timeout em Publish | BAIXO | Usar `context.WithTimeout(ctx, 5*time.Second)` |
| W31 | `handlers/sse_handler.go:456-462` | SSEStatusNotifier swallows user resolution errors | BAIXO | Rate-limited `sentry.CaptureMessage` para failures persistentes |

#### Cat. H: WAHA Data Consistency

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W32 | `services/monitoring/monitoring_service.go:256` | `math/rand` para jitter delay | BAIXO | Substituir por `crypto/rand` ou `math/rand/v2` |
| W33 | `services/automation/automation_service.go:647` | `math/rand` para CTA selection | BAIXO | Substituir por `crypto/rand` |
| W34 | `services/monitoring/monitoring_service.go:452` | `_, _ = s.dispatchLogRepo.Create(ctx, log)` silencioso | MEDIO | `sentry.CaptureException` + tag `{module: "monitoring", flow: "dispatch-log"}` |
| W35 | `services/automation/automation_service.go:749` | `_, _ = s.dispatchLogRepo.Create(ctx, log)` silencioso | MEDIO | `sentry.CaptureException` + tag `{module: "automation", flow: "dispatch-log"}` |
| W36 | `adapters/whatsapp/mapper.go:29-33` | `log.Printf` (stdlib) em vez de zap | BAIXO | Substituir por zap Debug com logger injetado |
| W37 | `services/redirect/redirect_service.go:199-208` | Invite code fetch failure sem Sentry (feature pública) | MEDIO | `sentry.CaptureException(err)` + tag `{module: "redirect", flow: "invite-code"}` |
| W38 | `workers/groups_sync_worker.go:124` | `InstanceID: fmt.Sprintf("%d", instance.ID)` tipo inconsistente | BAIXO | Considerar tipo consistente (int ou string) |

#### Cat. I: WAHA Security

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| W39 | `adapters/whatsapp/waha_adapter.go:200` | API key em HTTP header sem enforce de HTTPS | ALTO | Validar `WAHA_API_URL` começa com `https://` em produção |
| W40 | `handlers/waha_webhook_handler.go:84-86` | HMAC disabled apenas log Warn (bypass de webhook) | ALTO | `sentry.CaptureMessage` level Error + tag `{security: "hmac-disabled"}` |
| W41 | `adapters/whatsapp/hmac_validator.go` | HMAC disabled retorna `{Valid: true}` - design aceita tudo | BAIXO | Documentar + alerta de deploy |
| W42 | `services/whatsapp/groups_service.go:431-432` | Phone numbers logados em Info (PII) | MEDIO | Mascarar como `5511***9999` nos logs |
| W43 | `adapters/whatsapp/waha_adapter.go:507-515` | Participants raw response logada em Debug (PII) | BAIXO | Substituir por summary (count only) |
| W44 | `adapters/whatsapp/waha_adapter.go:235-238` | Full WAHA error response body logada | BAIXO | Truncar response body nos logs |

### FRONTEND REACT - 40 Pontos WAHA (WF1-WF40)

#### Cat. A: SSE/Real-time

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| WF1 | `hooks/use-session-status-sse.ts:72-80` | SSE reconnect delay fixo 3s sem backoff | BAIXO | Implementar exponential backoff (3s→6s→12s...) |
| WF2 | `hooks/use-session-status-sse.ts:68-70` | Max reconnect (maxRetries=5) sem notificação ao user | ALTO | `Sentry.captureMessage('SSE max reconnects exhausted')` + notificar user |
| WF3 | `hooks/use-session-status-sse.ts:50-53` | onerror logs console.warn sem Sentry | MEDIO | `Sentry.addBreadcrumb` + capture após 3 falhas consecutivas |
| WF4 | `hooks/use-session-status-sse.ts:87-89` | event.data JSON parse com `try/catch {}` vazio | BAIXO | `Sentry.captureException` para payload inválido |
| WF5 | `hooks/use-session-status-sse.ts:42` | Token expirado no URL query → SSE 401 loop | ALTO | Detectar 401 e acionar refresh token antes de reconnect |
| WF6 | `hooks/use-session-status-sse.ts:32` | Sem heartbeat/keepalive detection | MEDIO | Adicionar timeout para detectar conexão zumbi |

#### Cat. B: Connection Flow

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| WF7 | `hooks/use-whatsapp.ts:720-723` | handleConnect `catch { // ignore delete errors }` | MEDIO | Capturar non-404 errors com `Sentry.captureException` |
| WF8 | `hooks/use-whatsapp.ts:488-493` | refreshQrCode `catch { // ignore }` em delete | MEDIO | Capturar non-404 errors |
| WF9 | `hooks/use-whatsapp.ts:537-546` | QR countdown gera loop infinito de delete+create sem max retry | ALTO | Max retry counter + `Sentry.captureWarning` após N falhas |
| WF10 | `hooks/use-whatsapp.ts:512-513` | refreshQrCode fail apenas `console.warn` | MEDIO | `Sentry.captureException(err)` + toast para user |
| WF11 | `hooks/use-whatsapp.ts:913-927` | disconnectInstance swallows WAHA delete + DB delete errors | MEDIO | Capturar non-404 errors de ambos |
| WF12 | `hooks/use-whatsapp.ts:574` | verifyConnectionStatus usa stale `qrCode` closure | BAIXO | Usar ref para qrCode |
| WF13 | `hooks/use-whatsapp.ts:275-280` | Initial status check clears instance em QUALQUER erro | ALTO | Retry antes de limpar state; `Sentry.captureException` |

#### Cat. C: Group Operations

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| WF14 | `hooks/groups/use-groups-messaging.ts:79-81` | loadMentionParticipants `catch {}` skip silencioso | MEDIO | Track error count; se ALL groups fail → `Sentry.captureMessage` |
| WF15 | `hooks/groups/use-groups-messaging.ts:318-320` | handleSendMessage perde razão do erro individual | ALTO | `Sentry.captureException(error, { extra: { groupId } })` |
| WF39 | `components/groups/group-members-dialog.tsx:161-184` | handleConfirmedAction sem try/catch | MEDIO | Wrap em try/catch com `Sentry.captureException` |

#### Cat. D: Message Sending

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| WF16 | `hooks/groups/use-groups-messaging.ts:241-249` | Image file read error não faz return → envia sem imagem | MEDIO | `Sentry.captureException` + return após erro |
| WF17 | `hooks/groups/use-groups-messaging.ts:255-321` | Bulk send sem AbortController → requisições órfãs | ALTO | Implementar AbortController para cancelamento |
| WF18 | `components/groups/group-audio-dialog.tsx:96-104` | Upload de áudio sem validação de tamanho | MEDIO | Adicionar validação + capturar tentativas oversized |
| WF19 | `components/groups/group-photo-dialog.tsx:72-78` | Upload de foto sem validação de tipo/tamanho | BAIXO | Adicionar validação |
| WF20 | `components/groups/group-message-dialog.tsx:231-243` | Send button sem `isSending` disabled state | MEDIO | Prevenir double-click |
| WF40 | `components/groups/schedule-multiple-groups.tsx:110-118` | File read sem validação de tamanho + base64 split edge case | MEDIO | Validar tamanho + capturar tentativas oversized |

#### Cat. E: Data Consistency

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| WF21 | `hooks/use-whatsapp.ts:817-861` | Import groups polling setInterval sem cleanup no unmount | ALTO | Armazenar interval em ref + cleanup |
| WF22 | `hooks/use-whatsapp.ts:402-409` | `window.__importFallbackTimeout` mutação global insegura | MEDIO | Usar ref em vez de window global |
| WF23 | `hooks/use-whatsapp.ts:361-433` | SSE job event listener depende de closures stale | BAIXO | Otimizar dependencies |

#### Cat. F: Error Display/UX

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| WF24 | `components/groups/group-connection-status.tsx:116` | Status `checking` retorna `null` (sem feedback visual) | MEDIO | Adicionar loading/skeleton state |
| WF25 | `pages/groups.tsx:149-170` | Sem error state se load de grupos falha | MEDIO | Adicionar error boundary/state |
| WF26 | `components/groups/group-description-dialog.tsx:80-83` | Submit button sem disabled state para input vazio | BAIXO | Adicionar validação |
| WF34 | `hooks/use-redirect-link.ts:621-631` | `navigator.clipboard.writeText` Promise não await/catch | BAIXO | Await + catch + `Sentry.captureException` |
| WF37 | `pages/whatsapp.tsx:150` | ErrorBoundary genérico sem recovery WAHA-específico | MEDIO | Adicionar retry/reconnect no fallback |
| WF38 | `components/groups/group-poll-dialog.tsx:127` | Poll submit não valida question/options vazios | BAIXO | Adicionar validação |

#### Cat. G: Type Safety/Runtime

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| WF27 | `lib/whatsapp-formatter.tsx:42` | `let globalKey = 0` mutable global key counter | ALTO | Substituir por key generation segura (useId ou index-based) |
| WF28 | `components/groups/schedule-multiple-groups.tsx:214` | `error as { message?: string }` type assertion insegura | BAIXO | Usar `instanceof Error` pattern + `Sentry.captureException` |
| WF29 | `pages/jobs/components/jobs-table.tsx:72` | `new Date(job.scheduled_for)` sem validação | BAIXO | Validar antes de formatar |
| WF30 | `components/automations/automation-cards.tsx:73` | `new Date(auto.next_run_at)` pode ser string vazia | BAIXO | Wrap em try/catch |
| WF31 | `components/automations/offers-preview-dialog.tsx:94-97` | Fallback image usa URL externa (`via.placeholder.com`) | BAIXO | Usar placeholder local/inline SVG |

#### Cat. H: Cleanup/Resources

| # | Arquivo:Linha | Código | Severidade | Ação Sentry |
|---|---------------|--------|------------|-------------|
| WF32 | `hooks/groups/use-groups-messaging.ts` | File Object URLs não gerenciados | BAIXO | Nota preventiva para future previews |
| WF33 | `hooks/use-whatsapp.ts:330-355` | autoSync setInterval race condition no unmount | BAIXO | Guard com isMounted ref |
| WF35 | Multiple files | Zero AbortSignal em chamadas ConnectRPC (violação CLAUDE.md) | MEDIO | Implementar AbortController em todas as chamadas diretas |

#### Cat. Cross-Cutting: Zero Sentry no Frontend WAHA

| # | Arquivo | Código | Severidade | Ação Sentry |
|---|---------|--------|------------|-------------|
| WF36 | Todos os 33 arquivos WAHA frontend | Zero `Sentry.captureException` / `captureMessage` / import | CRITICO | Implementar Sentry em TODOS os error handlers identificados nesta auditoria |

### Resumo WAHA Pass - Distribuição por Severidade

| Severidade | Backend (W) | Frontend (WF) | Total |
|------------|-------------|---------------|-------|
| CRITICO | 0 | 1 (WF36) | **1** |
| ALTO | 6 (W2, W10, W17, W29, W39, W40) | 8 (WF2, WF5, WF9, WF13, WF15, WF17, WF21, WF27) | **14** |
| MEDIO | 21 | 17 | **38** |
| BAIXO | 17 | 14 | **31** |
| **Total** | **44** | **40** | **84** |

### Cobertura vs Plano Existente

| Status | Backend W | Frontend WF |
|--------|-----------|-------------|
| Novos (não cobertos) | 7 (W2, W4, W9, W14, W16, W25, W38) | 40 (todos WF são novos) |
| Mapeiam para itens existentes | 37 (S15, S7, N8, C3, B8, B34, B36, B41, B45, SQ4, SQ8, SQ12, SQ18, SQ22, SQ52) | 0 |

---

## 6th Pass: Final Coverage Audit (30 novos pontos)

> Auditoria final cruzando o plano (417 pontos) com TODOS os caminhos de erro reais no código. Identificados 30 gaps onde erros ainda escapariam em produção ou localhost, mesmo com as 5 passes anteriores implementadas. Objetivo: fechar 100% de cobertura.

### BACKEND GO - 15 Gaps Finais (BG1-BG15)

#### Cat. A: Bootstrap & Inicialização (BG1-BG4) — Erros ANTES do Sentry inicializar

| # | Arquivo:Linha | Gap | Severidade | Ação |
|---|---------------|-----|------------|------|
| BG1 | `cmd/server/main.go:75-80` | Config load failure via `fmt.Fprintf + os.Exit(1)` ANTES de `sentry.Init()` — erro nunca chega ao Sentry | MEDIO | Reordenar: carregar config → extrair DSN → `sentry.Init()` com DSN mínimo → continuar setup. Se DSN vazio, log warning e continuar sem Sentry |
| BG2 | `cmd/server/main.go:85-88` | Logger creation failure via `fmt.Fprintf + os.Exit(1)` — mesmo problema pré-Sentry | MEDIO | Mover `sentry.Init()` para ANTES da criação do logger. Se logger falha, `sentry.CaptureException` + `sentry.Flush` antes de os.Exit |
| BG3 | `internal/config/config.go` | Se `SENTRY_DSN` env var completamente ausente (não `""`, mas inexistente), `cfg.SentryDSN` é string vazia — dev mode sem Sentry é intencional, mas staging/prod precisam de validação | BAIXO | Adicionar em `LoadConfig()`: se `NODE_ENV=production` e `SENTRY_DSN` vazio, emitir log.Error de aviso (não fatal) |
| BG4 | `cmd/server/main.go:853` | HTTP server goroutine `srv.ListenAndServe()` usa `logger.Fatal` que chama `os.Exit(1)` — `defer sentry.Flush()` no main NUNCA executa porque `os.Exit` não roda defers | CRITICO | Substituir `logger.Fatal` por `logger.Error` + `sentry.CaptureException` + `sentry.Flush(2*time.Second)` + `os.Exit(1)` explícito |

#### Cat. B: Middleware vs ConnectRPC Gap (BG5-BG6) — Erros que bypassam o interceptor

| # | Arquivo:Linha | Gap | Severidade | Ação |
|---|---------------|-----|------------|------|
| BG5 | `cmd/server/main.go:~400-500` | Rotas Gin puras (`/health`, `/webhooks/*`, `/metrics`) erros passam APENAS pelo Gin middleware, nunca pelo ConnectRPC SentryInterceptor | MEDIO | Adicionar `sentrygin.New()` middleware que já cobre rotas Gin. Verificar que `error_handler.go` captura 5xx nessas rotas também via Sentry hub |
| BG6 | `handlers/waha_webhook_handler.go` | Webhook handler é rota Gin, não ConnectRPC — ConnectRPC interceptor classification (Ignorar/Warning/Error) não se aplica | MEDIO | Garantir que webhook handler tem `sentry.CaptureException` explícito em todos os error paths, não dependendo do interceptor |

#### Cat. C: ConnectRPC Interceptor Edge Cases (BG7-BG8)

| # | Arquivo:Linha | Gap | Severidade | Ação |
|---|---------------|-----|------------|------|
| BG7 | `connectrpc/interceptors.go:67-113` | Recovery interceptor captura panics e loga, mas NÃO repassa ao Sentry — `SentryInterceptor` vem DEPOIS do Recovery, nunca vê o panic | ALTO | Mover Sentry capture DENTRO do Recovery interceptor: `sentry.CurrentHub().RecoverWithContext(ctx, r)` no bloco recover(), ANTES de retornar ConnectError |
| BG8 | `internal/observability/sentry_interceptor.go` (novo) | Interceptor não lida com `Status: nil` (success mas response nil) nem com erros custom que não mapeiam para gRPC codes | BAIXO | Adicionar fallback: se `connect.CodeOf(err)` retorna code desconhecido, tratar como `Internal` + capturar |

#### Cat. D: External HTTP Clients Não Cobertos (BG9-BG10)

| # | Arquivo | Gap | Severidade | Ação |
|---|---------|-----|------------|------|
| BG9 | `services/shopee/shopee_service.go` | Todos HTTP calls para Shopee API sem Sentry capture — timeouts, 5xx, parsing errors invisíveis | MEDIO | Adicionar `sentry.CaptureException` com tags `{external: "shopee", endpoint, status_code}` em todos error paths |
| BG10 | `lib/urlsafety/urlsafety.go` | URL safety check contra external API sem Sentry — false negatives (URL perigosa passa) invisíveis | MEDIO | Adicionar `sentry.CaptureMessage` level Warning quando URL check falha por timeout/error (fail-open) |

#### Cat. E: Cache Semântica & Circuit Breaking (BG11-BG12)

| # | Arquivo | Gap | Severidade | Ação |
|---|---------|-----|------------|------|
| BG11 | `internal/services/*` (múltiplos) | Redis cache SET failures capturados individualmente por Phase 3, mas sem circuit breaker — se Redis cai, CADA request gera evento Sentry → flood de milhares de eventos | MEDIO | Implementar rate limiting local no `LogAndCapture`: se mesmo erro repetido >10x em 60s, capturar apenas 1 com `extra: { suppressed_count: N }` |
| BG12 | `internal/auth/session.go` | Cache dedup failures (S44, S46, S49) permitem duplicatas sem indicação — plano diz capturar Warning, mas não define se request deve fail-open ou fail-closed | BAIXO | Definir semântica: cache failures são SEMPRE fail-open (request prossegue) + Warning level. Adicionar tag `{cache_policy: "fail-open"}` |

#### Cat. F: Cron & Workers Phase Dependency (BG13-BG15)

| # | Arquivo | Gap | Severidade | Ação |
|---|---------|-----|------------|------|
| BG13 | `services/cron/cron_service.go` | Cron jobs SafeGo está na Phase 2, mas `cronService.Start()` pode falhar na Phase 1 — se Phase 1 roda sem Phase 2, cron panics não são capturados | ALTO | Mover `SafeGo` do cron para Phase 1 Step 15 (junto com queue server SafeGo) em vez de Phase 2 |
| BG14 | `cmd/server/main.go:~1030-1060` | 7 workers Asynq registrados sem lista explícita — plano diz "wrappear cada handler" mas não lista os 7 | MEDIO | Lista explícita: (1) webhook_consumer, (2) message_worker, (3) groups_sync, (4) reconciliation, (5) dispatch_log, (6) monitoring, (7) message_queue — cada um com `SentryAsynqMiddleware` |
| BG15 | `internal/observability/sentry.go` (novo) | `beforeSend` PII redaction muito agressiva pode tornar erros ilegíveis — ex: "Failed to send to +55..." → "Failed to send to [REDACTED]" perde contexto | MEDIO | Implementar redação parcial: manter prefixo do phone (country code + 2 dígitos) + mascarar resto: `+55 11 ****-****`. Para emails: `b***@gmail.com`. Nunca redactar error types/stack frames |

### FRONTEND REACT - 15 Gaps Finais (FG1-FG15)

#### Cat. A: Bootstrap & Root Mount (FG1-FG3) — Crashes ANTES do Sentry

| # | Arquivo:Linha | Gap | Severidade | Ação |
|---|---------------|-----|------------|------|
| FG1 | `src/main.tsx:6` | `createRoot(document.getElementById('root')!)` — se `#root` ausente, crash ANTES de `initSentry()` | CRITICO | Reordenar: (1) `initSentry()` PRIMEIRO (2) null check `#root` com `Sentry.captureException` se ausente (3) `createRoot(root).render()`. Plan step FSQ1 existe mas não especifica a ordem |
| FG2 | `src/main.tsx` | Sem `window.addEventListener('unhandledrejection', ...)` explícito — Sentry SDK auto-instala MAS só se `Sentry.init()` completa com sucesso. Se DSN inválido ou init falha, promises rejeitadas são completamente invisíveis | ALTO | Adicionar handler manual ANTES de `Sentry.init()` como safety net: `window.addEventListener('unhandledrejection', (e) => { console.error('Unhandled rejection:', e.reason); })`. Após Sentry init, o SDK sobrescreve |
| FG3 | `src/components/error-boundary.tsx:27-29` | `getDerivedStateFromError` captura render errors para fallback UI, mas `componentDidCatch` NÃO EXISTE — erros React NUNCA chegam ao Sentry. Plan step FSQ14 cobre isso, mas Phase 4 Step 5 e Step 11 são DUPLICADOS | CRITICO | Consolidar: Phase 4 Step 5 E Step 11 são o mesmo fix. Remover duplicação e garantir que `componentDidCatch` chama `Sentry.captureException(error, { contexts: { react: { componentStack } } })` |

#### Cat. B: SSE/Real-time Silent Failures (FG4-FG7)

| # | Arquivo:Linha | Gap | Severidade | Ação |
|---|---------------|-----|------------|------|
| FG4 | `hooks/use-session-status-sse.ts:195` | `eventSource.onerror` apenas `console.warn('[SSE] Connection error')` — 9+ locais com console-only logging. Plan step SF5/SF23 cobre parcialmente, mas não lista TODOS os 9 locais | ALTO | Lista completa dos 9 pontos SSE console-only: linhas 195, 203, 211, 221, 248, 270, 288, 304, 335. CADA um precisa de `Sentry.addBreadcrumb` ou `captureHookError` |
| FG5 | `hooks/use-session-status-sse.ts:220-304` | addEventListener callbacks podem lançar exceções se `onStatusChangeRef.current()` (useWhatsApp handler) falha — exceção propaga para event loop, não capturada pelo try/catch do JSON.parse | ALTO | Wrappear CADA addEventListener callback body em try/catch completo: `try { const data = JSON.parse(...); onRef.current?.(data); } catch (err) { Sentry.captureException(err, { tags: { event: eventName } }); }` |
| FG6 | `hooks/use-whatsapp.ts:105-163` | `onStatusChange` callback chamado pelo SSE listener — se este handler lança exceção, o SSE inteiro morre silenciosamente porque a exceção não é capturada no listener (FG5) | ALTO | Garantir que `onStatusChange` é defensivo: wrappear corpo inteiro em try/catch com `captureHookError(err, 'useWhatsApp', 'onStatusChange')` |
| FG7 | `hooks/use-whatsapp.ts:195, 851-860` | 5× `console.warn()` para polling failures, status sync errors — zero Sentry capture | ALTO | Substituir cada `console.warn` por `captureHookError(err, 'useWhatsApp', operation)` ou `Sentry.addBreadcrumb({ category: 'whatsapp', message, level: 'warning' })` |

#### Cat. C: Catch Blocks Silenciosos Não Cobertos (FG8-FG10)

| # | Arquivo:Linha | Gap | Severidade | Ação |
|---|---------------|-----|------------|------|
| FG8 | `hooks/use-automations.ts:679-695` | 2× `catch { // Failed to toggle/delete }` — operação falha SEM toast, SEM Sentry, SEM feedback ao usuário. Plan FSQ não cobre este arquivo especificamente | ALTO | Adicionar `captureHookError(err, 'useAutomations', 'toggle/delete')` + toast de erro |
| FG9 | `hooks/use-redirect-link.ts:240-246` | Mutations com `onError: () => { toast(...) }` — param `error` ignorado, detalhes perdidos. Plan genérico "demais hooks" não lista este caso | MEDIO | Alterar para `onError: (error) => { captureHookError(error, 'useRedirectLink', 'mutation'); toast(...) }` |
| FG10 | Multiple hooks | `catch {}` ou `catch { // ignore }` que o plan marca como "breadcrumb" mas sem assignment de Phase/Step explícito — risco de esquecimento na implementação | MEDIO | Criar checklist de varredura em Phase 6: `grep -r "catch {" client/src | grep -v node_modules | grep -v gen` deve retornar 0 resultados |

#### Cat. D: Browser API Edge Cases (FG11-FG13)

| # | Arquivo:Linha | Gap | Severidade | Ação |
|---|---------------|-----|------------|------|
| FG11 | `hooks/use-whatsapp.ts:80-83, 201-202` | localStorage sem try/catch — crash em private browsing mode. Plan SF14-SF15 e Phase 4 Step 2 cobrem via `safeLocalStorage`, mas NÃO lista TODOS os locais que usam localStorage diretamente | CRITICO | Varredura completa: `grep -r "localStorage\." client/src` — substituir TODAS as ocorrências por `safeLocalStorage.*`. Adicionar lint rule (eslint-plugin-no-restricted-globals ou BiomeJS rule) para prevenir uso direto futuro |
| FG12 | `hooks/use-coupons.ts:543`, `admin/dashboard/webhook-section.tsx:65` | `navigator.clipboard.writeText()` sem try/catch — falha em HTTP context, permissions denied. Plan FSQ2-FSQ3 cobre, mas sem fallback funcional | MEDIO | Implementar helper `safeClipboard.writeText(text)` que: (1) tenta `navigator.clipboard.writeText` (2) fallback: `document.execCommand('copy')` (3) se ambos falham: `Sentry.captureMessage` + toast "Copiar manualmente" |
| FG13 | `lib/connect-transport.ts:44-48` | `crypto.getRandomValues()` sem try/catch — crash em WebView antigo ou SSR. Plan FSQ32 lista como BAIXO mas é chamado em TODA request ConnectRPC (transport init) | MEDIO | Wrappear em try/catch com fallback `Math.random()` (menos seguro mas funcional) + `Sentry.captureMessage('crypto.getRandomValues unavailable', 'warning')` |

#### Cat. E: Segurança & Dev Mode (FG14-FG15)

| # | Arquivo:Linha | Gap | Severidade | Ação |
|---|---------------|-----|------------|------|
| FG14 | `hooks/use-session-status-sse.ts:183` | Token de autenticação no URL query string do SSE — aparece em breadcrumbs Sentry, browser history, proxy logs, HTTP Referer. Plan FSQ18 cobre `beforeBreadcrumb` scrubbing, mas não resolve o problema root (token no URL) | MEDIO | Phase 6: avaliar migração de token do query string para header customizado ou cookie HttpOnly em versão futura. Curto prazo: `beforeBreadcrumb` scrub + `beforeSend` scrub de URLs com `token=` |
| FG15 | `src/lib/sentry.ts` (novo) | Localhost/dev mode: se `VITE_SENTRY_DSN` ausente, Sentry não inicializa — desenvolvimento COMPLETAMENTE cego a erros. Staging com DSN configurado funciona, mas dev local não | BAIXO | Adicionar opção: se `VITE_SENTRY_DSN` ausente mas `VITE_SENTRY_DEV_MODE=true`, inicializar Sentry com `debug: true, beforeSend: (event) => { console.warn('[Sentry Dev]', event); return null; }` — loga erros no console no formato Sentry sem enviar |

### Resumo 6th Pass - Distribuição por Severidade

| Severidade | Backend (BG) | Frontend (FG) | Total |
|------------|-------------|---------------|-------|
| CRITICO | 1 (BG4) | 3 (FG1, FG3, FG11) | 4 |
| ALTO | 2 (BG7, BG13) | 6 (FG2, FG4, FG5, FG6, FG7, FG8) | 8 |
| MEDIO | 9 (BG1, BG2, BG5, BG6, BG9, BG10, BG11, BG14, BG15) | 5 (FG9, FG10, FG12, FG13, FG14) | 14 |
| BAIXO | 3 (BG3, BG8, BG12) | 1 (FG15) | 4 |
| **Total** | **15** | **15** | **30** |

### Impacto na Cobertura

| Camada | Antes (5 passes) | Depois (6 passes) |
|--------|-------------------|-------------------|
| Bootstrap/Init (backend + frontend) | 60% | 100% |
| ConnectRPC Interceptor | 95% | 100% |
| Gin Routes (non-ConnectRPC) | 70% | 100% |
| Goroutines/Workers | 85% | 100% |
| SSE/Real-time | 50% | 100% |
| React Render Errors | 40% | 100% |
| Browser APIs (localStorage, clipboard, crypto) | 20% | 95% |
| External HTTP Clients | 75% | 100% |
| Cache/Redis | 80% | 100% |
| Dev/Localhost Mode | 0% | 90% |
| PII Redaction Quality | 70% | 95% |
| **Cobertura Geral** | **~85%** | **~99%** |

---

## Mapa Completo de Pontos de Integração (Original)

### BACKEND - Pontos de Captura por Camada

#### Camada 1: Entry Point (`cmd/server/main.go`)

| # | Ponto | Arquivo:Linha | Tipo de Erro | Ação Sentry |
|---|-------|---------------|-------------|-------------|
| B1 | PostgreSQL connection failure | `main.go:125-135` | Fatal startup | `sentry.CaptureException` + Flush antes de os.Exit |
| B2 | WAHA adapter creation failure | `main.go:168-176` | Fatal startup | `sentry.CaptureException` + Flush antes de os.Exit |
| B3 | HTTP server listen error | `main.go:857-859` | Fatal startup | `sentry.CaptureException` + Flush |
| B4 | Queue server start error | `main.go:1100-1102` | Fatal worker | `sentry.CaptureException` + Flush |
| B5 | Server shutdown error | `main.go:877-878` | Graceful shutdown | `sentry.CaptureMessage` level Warning |
| B6 | Redis connection failure | `main.go:114-119` | Degraded mode | `sentry.CaptureMessage` level Warning + tag:degraded |
| B7 | Email adapter failure | `main.go:625-628` | Feature disabled | `sentry.CaptureMessage` level Warning |
| B8 | Backfill goroutine errors | `main.go:1012-1036` | Background async | `sentry.CaptureException` com scope goroutine |
| B9 | SSE Redis bridge failure | `main.go:419-424` | Feature degraded | `sentry.CaptureException` |

#### Camada 2: Middleware Gin (`internal/middleware/`)

| # | Ponto | Arquivo | Tipo de Erro | Ação Sentry |
|---|-------|---------|-------------|-------------|
| B10 | Panic recovery | `error_handler.go:113-140` | Unhandled panic | `sentrygin.New()` middleware (automatico) |
| B11 | 5xx responses | `error_handler.go:89-107` | Internal errors | `sentry.CaptureException` com request context |
| B12 | Request logging (500+) | `logging.go:50-58` | Error-level logs | Ja capturado pelo middleware Sentry |

#### Camada 3: ConnectRPC Interceptors (`internal/connectrpc/`)

| # | Ponto | Arquivo | Tipo de Erro | Ação Sentry |
|---|-------|---------|-------------|-------------|
| B13 | Panic recovery (unary) | `interceptors.go:67-92` | Unhandled panic | `sentry.CurrentHub().Recover()` + stack trace |
| B14 | Panic recovery (streaming) | `interceptors.go:98-113` | Unhandled panic | `sentry.CurrentHub().Recover()` + stack trace |
| B15 | Auth failure | `interceptors.go:318-325` | Unauthenticated | NAO capturar (esperado) - apenas breadcrumb |
| B16 | CSRF failure | `interceptors.go:422-430` | Permission denied | `sentry.CaptureMessage` level Warning (possivel ataque) |
| B17 | Validation failure | `interceptors.go:486-488` | Invalid argument | NAO capturar (esperado) - apenas breadcrumb |
| B18 | Rate limit exceeded | `interceptors.go:526-536` | Resource exhausted | `sentry.CaptureMessage` + tag:rate_limit |
| B19 | Rate limit check failure | `interceptors.go:526-530` | Internal error | `sentry.CaptureException` (fail-open risk) |
| B20 | Suspension check failure | `suspension_interceptor.go:81-89` | Internal error | `sentry.CaptureException` (fail-open risk) |
| B21 | Profile lookup failure | `suspension_interceptor.go:228-242` | Database error | `sentry.CaptureException` |

#### Camada 4: HTTP Handlers (`internal/handlers/`)

| # | Ponto | Arquivo | Tipo de Erro | Ação Sentry |
|---|-------|---------|-------------|-------------|
| B22 | HMAC validation failure | `waha_webhook_handler.go:143-172` | Security event | `sentry.CaptureMessage` level Warning + tag:security |
| B23 | Body read/too large | `waha_webhook_handler.go:127-140` | Client error | NAO capturar (4xx) |
| B24 | JSON parse failure | `waha_webhook_handler.go:181-188` | Bad payload | `sentry.CaptureException` + tag:webhook |
| B25 | Idempotency check failure | `waha_webhook_handler.go:220-227` | Database error | `sentry.CaptureException` + tag:idempotency |
| B26 | Task enqueue failure | `waha_webhook_handler.go:257-276` | Queue error | `sentry.CaptureException` level Critical |
| B27 | Mark event failed error | `waha_webhook_handler.go:294-305` | Database error | `sentry.CaptureException` |
| B28 | SSE Redis publish failure | `sse_redis_bridge.go:64-75` | Redis error | `sentry.CaptureException` + tag:sse |

#### Camada 5: ConnectRPC Handlers (`internal/connectrpc/*_handler.go`)

| # | Ponto | Tipo de Erro | Ação Sentry |
|---|-------|-------------|-------------|
| B29 | Todos os 28 handlers | Erros de service/repository propagados | Capturar via interceptor centralizado (novo SentryInterceptor) |

**Novo `SentryInterceptor`** - interceptor ConnectRPC dedicado que:
- Captura automaticamente todo erro retornado por handlers com code >= Internal
- Adiciona contexto: procedure, traceId, userId, sessionName
- Ignora erros esperados: InvalidArgument, Unauthenticated, NotFound
- Captura como Warning: PermissionDenied, ResourceExhausted
- Captura como Error: Internal, Unknown, DataLoss, Unavailable

#### Camada 6: Workers/Background Jobs (`internal/workers/`)

| # | Ponto | Arquivo | Tipo de Erro | Ação Sentry |
|---|-------|---------|-------------|-------------|
| B30 | Webhook consumer - event not found | `webhook_consumer_worker.go` | Missing data | `sentry.CaptureException` + tag:worker=webhook |
| B31 | Webhook consumer - max retries | `webhook_consumer_worker.go` | DLQ entry | `sentry.CaptureException` level Critical + tag:dlq |
| B32 | Webhook consumer - processor failure | `webhook_consumer_worker.go` | Processing error | `sentry.CaptureException` + tag:event_type |
| B33 | Message worker - send failure | `message_worker.go` | WAHA API error | `sentry.CaptureException` + tag:worker=message |
| B34 | Message worker - URL blocked | `message_worker.go:71-79` | Safety filter | `sentry.CaptureMessage` level Info (monitoramento) |
| B35 | Groups sync worker - adapter failure | `groups_sync_worker.go` | WAHA API error | `sentry.CaptureException` + tag:worker=groups_sync |
| B36 | Reconciliation worker - API failure | `reconciliation_worker.go` | External API error | `sentry.CaptureException` + tag:worker=reconciliation |
| B37 | Dispatch log processor - DB failure | `dispatch_log_processor.go` | Database error | `sentry.CaptureException` + tag:worker=dispatch_log |
| B38 | Monitoring processor - automation failure | `monitoring_message_processor.go` | Automation error | `sentry.CaptureException` + tag:worker=monitoring |

#### Camada 7: Services (`internal/services/`)

| # | Ponto | Arquivo | Tipo de Erro | Ação Sentry |
|---|-------|---------|-------------|-------------|
| B39 | Automation execution failure | `automation/automation_service.go` | Business logic | Propagado via handler → interceptor |
| B40 | Reconciliation checkpoint errors | `reconciliation/reconciliation_service.go` | Data consistency | `sentry.CaptureException` level Critical |
| B41 | Kiwify webhook - signature validation | `kiwify/kiwify_service.go` | Security event | `sentry.CaptureMessage` level Warning |
| B42 | Kiwify webhook - user creation failure | `kiwify/kiwify_service.go` | Business critical | `sentry.CaptureException` |
| B43 | ML product fetch timeout | `mercadolivre/ml_service.go` | External API | `sentry.CaptureException` + tag:external=mercadolivre |
| B44 | ML credential load failure | `mercadolivre/ml_service.go:97-99` | Config error | `sentry.CaptureException` |
| B45 | AI API call failure | `ai/ai_service.go:92-99` | External API | `sentry.CaptureException` + tag:external=ai |
| B46 | AI not configured | `ai/ai_service.go:61-62` | Config missing | NAO capturar (startup warning suficiente) |
| B47 | Shopee API failure | `shopee/shopee_service.go` | External API | `sentry.CaptureException` + tag:external=shopee |
| B48 | Email send failure | `email/email_service.go` | SMTP error | `sentry.CaptureException` + tag:external=smtp |
| B49 | Password reset token error | `profile/password_service.go` | Crypto/DB error | `sentry.CaptureException` |
| B50 | Redirect link processing error | `redirect/redirect_service.go` | Business logic | Propagado via handler → interceptor |

#### Camada 8: Repository (`internal/repository/postgres/`)

| # | Ponto | Tipo de Erro | Ação Sentry |
|---|-------|-------------|-------------|
| B51 | Todos os repositories | Erros SQL/connection | NAO capturar diretamente - propagar via service → handler → interceptor |

**Exceção:** Erros de connection pool exhaustion e deadlocks devem ser capturados diretamente quando detectados.

#### Camada 9: Adapters/External (`internal/adapters/`)

| # | Ponto | Arquivo | Tipo de Erro | Ação Sentry |
|---|-------|---------|-------------|-------------|
| B52 | WAHA HMAC validation error | `whatsapp/hmac_validator.go:68-98` | Security event | Ja capturado em B22 (handler level) |
| B53 | WAHA API timeouts | `whatsapp/` (adapter methods) | External timeout | `sentry.CaptureException` + tag:external=waha |

---

### FRONTEND - Pontos de Captura por Camada

#### Camada 1: Entry Point + App Root

| # | Ponto | Arquivo | Tipo de Erro | Ação Sentry |
|---|-------|---------|-------------|-------------|
| F1 | Sentry.init() | `src/main.tsx` | N/A (inicialização) | `Sentry.init({ dsn, environment, release, tracesSampleRate, replaysSessionSampleRate })` |
| F2 | Global ErrorBoundary | `src/app.tsx` | Unhandled render errors | `Sentry.ErrorBoundary` wrapping routes |
| F3 | window.onerror | `src/main.tsx` | Unhandled JS errors | Automatico via Sentry SDK |
| F4 | window.onunhandledrejection | `src/main.tsx` | Unhandled promise rejections | Automatico via Sentry SDK |

#### Camada 2: ConnectRPC Transport (`src/lib/connect-transport.ts`)

| # | Ponto | Tipo de Erro | Ação Sentry |
|---|-------|-------------|-------------|
| F5 | Error normalization interceptor | Todos os erros ConnectRPC | `Sentry.captureException` com tags: method, service, grpc_code, traceId |
| F6 | Network errors | Fetch failed / timeout | `Sentry.captureException` + tag:type=network |
| F7 | Retry exhaustion | Max retries reached | `Sentry.captureMessage` level Warning |

#### Camada 3: Auth Context (`src/contexts/auth-context.tsx`)

| # | Ponto | Tipo de Erro | Ação Sentry |
|---|-------|-------------|-------------|
| F8 | signIn failure | Auth error | `Sentry.captureException` + tag:flow=signin (apenas erros inesperados) |
| F9 | signUp failure | Auth error | `Sentry.captureException` + tag:flow=signup |
| F10 | Session check failure | Network/server error | `Sentry.captureException` + tag:flow=session_check |
| F11 | signOut failure | Swallowed error | `Sentry.captureException` (atualmente silenciado) |
| F12 | User context | N/A | `Sentry.setUser({ id, email, role })` no login, `Sentry.setUser(null)` no logout |

#### Camada 4: SSE Real-time (`src/hooks/use-session-status-sse.ts`)

| # | Ponto | Tipo de Erro | Ação Sentry |
|---|-------|-------------|-------------|
| F13 | SSE connection error | EventSource error | `Sentry.addBreadcrumb({ category: 'sse', message: 'connection error' })` |
| F14 | SSE max reconnect reached | Reconnect exhaustion | `Sentry.captureMessage('SSE max reconnects reached')` level Warning |
| F15 | SSE JSON parse error | Malformed event data | `Sentry.captureException` + tag:sse_event_type |

#### Camada 5: Error Boundary (`src/components/error-boundary.tsx`)

| # | Ponto | Tipo de Erro | Ação Sentry |
|---|-------|-------------|-------------|
| F16 | componentDidCatch | React render error | `Sentry.captureException(error, { contexts: { react: { componentStack } } })` |

#### Camada 6: Hooks com try/catch

| # | Hook | Arquivo | Erros Capturados |
|---|------|---------|-----------------|
| F17 | useAutomations | `hooks/use-automations.ts` | CRUD, run, preview, reset - todos os catch blocks |
| F18 | useWhatsApp | `hooks/use-whatsapp.ts` | Import, disconnect, status check - catch blocks vazios! |
| F19 | useJobs | `hooks/use-jobs.ts` | Scheduling, AI enhancement - catch blocks |
| F20 | useRedirectLink | `hooks/use-redirect-link.ts` | CRUD mutations - catch blocks |
| F21 | useForgotPassword | `hooks/use-forgot-password.ts` | Password reset - catch block |
| F22 | useGroups + sub-hooks | `hooks/groups/` | All group operations - catch blocks |
| F23 | Demais hooks (10+) | `hooks/` | Cada catch block deve ter `Sentry.captureException` |

#### Camada 7: Lazy Loading / Code Splitting

| # | Ponto | Tipo de Erro | Ação Sentry |
|---|-------|-------------|-------------|
| F24 | React.lazy() chunk load failure | Network/CDN error | `Sentry.captureException` via Sentry.ErrorBoundary |

#### Camada 8: Build + Source Maps

| # | Ponto | Ação |
|---|-------|------|
| F25 | Vite build source maps | Habilitar `build.sourcemap: 'hidden'` no vite.config.ts |
| F26 | Sentry Vite plugin | Adicionar `@sentry/vite-plugin` para upload automatico de source maps |
| F27 | Release tracking | Usar git commit SHA como release identifier |

---

## Arquivos a Criar/Modificar

### Novos Arquivos

| Arquivo | Descrição |
|---------|-----------|
| `automatiza-go/internal/observability/sentry.go` | Inicialização Sentry + helpers (InitSentry, CaptureWithScope, SentryFlush, SafeGo) |
| `automatiza-go/internal/observability/sentry_interceptor.go` | ConnectRPC interceptor dedicado ao Sentry |
| `automatiza-go/internal/observability/sentry_asynq.go` | Wrapper para Asynq handlers com Sentry |
| `client/src/lib/sentry.ts` | Inicialização Sentry + config + helpers (captureHookError, setSentryUser, safeLocalStorage) |
| `client/src/lib/sentry-connect-interceptor.ts` | Interceptor ConnectRPC dedicado ao Sentry |

### Arquivos Modificados

| Arquivo | Mudança |
|---------|---------|
| **Backend - Core** | |
| `automatiza-go/cmd/server/main.go` | InitSentry + defer Flush + sentrygin + SafeGo nas goroutines + capturar defer Close errors (S15-S20, S31, S66) |
| `automatiza-go/internal/connectrpc/interceptors.go` | SentryInterceptor na chain + Recover com Sentry |
| `automatiza-go/internal/middleware/error_handler.go` | CaptureException em 5xx |
| `automatiza-go/internal/connectrpc/suspension_interceptor.go` | CaptureException em fail-open |
| **Backend - Auth (CRITICO)** | |
| `automatiza-go/internal/auth/service.go` | Capturar erros de rate limiter (S1-S4), session revocation (S4), login tracking (S5-S9) |
| `automatiza-go/internal/auth/session.go` | Capturar erros de cache session (S10-S14) |
| **Backend - Workers** | |
| `automatiza-go/internal/workers/*.go` (7 arquivos) | SentryAsynqMiddleware em todos |
| **Backend - Handlers** | |
| `automatiza-go/internal/handlers/waha_webhook_handler.go` | Captures B22-B27 |
| `automatiza-go/internal/handlers/sse_redis_bridge.go` | Capture B28 + SafeGo na goroutine (S68) |
| `automatiza-go/internal/handlers/stubs.go` | Tratar erros repo (S24-S26) + SafeGo no monitoring (S67) |
| **Backend - Services** | |
| `automatiza-go/internal/services/whatsapp/message_service.go` | Capturar historyRepo.Update errors (S33-S40) |
| `automatiza-go/internal/services/automation/automation_service.go` | Capturar history errors (S41-S43) + backpressure (S74) + cache (S49) |
| `automatiza-go/internal/services/automation/process_list.go` | Capturar backpressure (S75) |
| `automatiza-go/internal/services/monitoring/monitoring_service.go` | Capturar cache dedup (S44-S46) + dispatch log (S76) |
| `automatiza-go/internal/services/shopee/shopee_service.go` | Capturar cache (S47-S48) + parseInt (S30) |
| `automatiza-go/internal/services/loadbalancer/lb_service.go` | Capturar cache ops (S50-S55) + count errors (S80-S81) |
| `automatiza-go/internal/services/mercadolivre/ml_service.go` | Capturar duplicate check (S82) + captures B43-B44 |
| `automatiza-go/internal/services/mllinklists/service.go` | Capturar URL normalization (S58-S60) |
| `automatiza-go/internal/services/kiwify/kiwify_service.go` | Capturar regex (S61) + captures B41-B42 |
| `automatiza-go/internal/services/reconciliation/reconciliation_service.go` | Capture B40 + context check (S73) |
| `automatiza-go/internal/services/ai/ai_service.go` | Capture B45 |
| `automatiza-go/internal/services/email/email_service.go` | Capture B48 |
| `automatiza-go/internal/services/redirect/redirect_service.go` | Capturar instanceID parse (S79) |
| **Backend - Repository** | |
| `automatiza-go/internal/repository/postgres/action_history_repository.go` | Capturar json.Marshal (S57) |
| `automatiza-go/internal/repository/postgres/profile_repository.go` | Capturar password update (S77) |
| `automatiza-go/internal/repository/postgres/settings_repository.go` | Validar type assertions (S28) |
| `automatiza-go/internal/repository/postgres/coupon_repository.go` | Validar time.Parse (S29) |
| `automatiza-go/internal/repository/postgres/group_repository.go` | Capturar instanceID parse (S78) |
| `automatiza-go/internal/connectrpc/webhooks_handler.go` | Capturar webhook log create/update (S62-S65) |
| `automatiza-go/internal/config/database.go` | Capturar rows.Close() (S23) |
| `automatiza-go/internal/config/config.go` | Log warnings config (S21-S22) |
| **Backend - Context/Concurrency** | |
| `automatiza-go/internal/workers/webhook_consumer_worker.go` | ctx.Done() check (S69) |
| `automatiza-go/internal/middleware/webhook_rate_limiter.go` | ctx.Done() check (S71) |
| `automatiza-go/internal/services/cron/cron_service.go` | ctx.Done() check (S72) |
| `automatiza-go/go.mod` | Adicionar `github.com/getsentry/sentry-go` |
| **Frontend - Core** | |
| `client/src/main.tsx` | initSentry() ANTES de render |
| `client/src/app.tsx` | Sentry.ErrorBoundary wrapping routes (SF29) |
| `client/src/components/error-boundary.tsx` | Integrar com Sentry.captureException |
| `client/src/contexts/auth-context.tsx` | Sentry.setUser() + captures |
| `client/src/lib/connect-transport.ts` | SentryInterceptor na chain |
| **Frontend - Hooks (erros silenciosos)** | |
| `client/src/hooks/use-automations.ts` | Corrigir catch vazios SF3-SF4 + captures em todos os catch blocks |
| `client/src/hooks/use-whatsapp.ts` | Corrigir localStorage SF14-SF15 + promises SF16-SF17 + console-only SF6-SF9 |
| `client/src/hooks/use-session-status-sse.ts` | Breadcrumbs + captures SF5 + type assertion SF23 |
| `client/src/hooks/groups/use-groups-messaging.ts` | Corrigir catch silenciosos SF11-SF13 |
| `client/src/hooks/use-redirect-link.ts` | Mutation errors SF18-SF20 + clipboard SF22 |
| `client/src/hooks/use-ml-link-lists.ts` | Clipboard SF1 + URL parse SF25 + poll errors |
| `client/src/hooks/use-affiliate-programs.ts` | JSON parse SF26-SF27 |
| `client/src/hooks/use-history.ts` | Remover debug logs SF10 + cache invalidation SF28 |
| `client/src/hooks/use-settings-connect.ts` | Mutation error SF21 |
| `client/src/hooks/use-public-redirect.ts` | Capture em fallbacks SF2 |
| `client/src/hooks/use-jobs.ts` | Captures nos catch blocks |
| `client/src/hooks/use-forgot-password.ts` | Capture no catch block |
| `client/src/hooks/groups/use-groups.ts` | Captures nos catch blocks |
| **Frontend - Build** | |
| `client/vite.config.ts` | Source maps + sentry plugin |
| `client/package.json` | @sentry/react, @sentry/vite-plugin |
| **3rd Pass - Backend Concurrency (C1-C10)** | |
| `automatiza-go/internal/handlers/sse_handler.go` | SafeGo para hub.run() (C1) |
| `automatiza-go/internal/services/cron/cron_service.go` | SafeGo para runTask + fix ticker leak (C2, C7) |
| `automatiza-go/internal/services/watchdog/watchdog.go` | Fix map race + SafeGo (C3, C10) |
| `automatiza-go/internal/auth/tokens.go` | sync.Once para HMAC init (C4) |
| `automatiza-go/internal/repository/postgres/db.go` | statement_timeout + pool monitoring (C9, N8) |
| **3rd Pass - Backend Resilience (N1-N17)** | |
| `automatiza-go/internal/adapters/whatsapp/waha_adapter.go` | HTTP Transport + MaxBytesReader + per-op timeout + retry (C5, C6, N1, N2, N3) |
| `automatiza-go/internal/lib/cache/cache.go` | Monitoramento de tamanho do cache (N15) |
| `automatiza-go/internal/repository/postgres/webhook_events_repository.go` | Limit guard (N9) |
| **3rd Pass - Frontend Runtime (FR1-FR12)** | |
| `client/src/pages/groups.tsx` | Section ErrorBoundary (FR1) |
| `client/src/pages/dashboard.tsx` | Section ErrorBoundary (FR1) |
| `client/src/hooks/use-automations-connect.ts` | mutation onError (FR5) |
| `client/src/components/groups/group-photo-dialog.tsx` | File size validation (FR9) |
| Config i18n | missingKeyHandler (FR11) |
| **4th Pass - Backend SonarQube (SQ1-SQ68)** | |
| `automatiza-go/internal/services/ai/ai_service.go` | Capturar json.Marshal + io.ReadAll erros (SQ1-SQ2) |
| `automatiza-go/internal/services/mercadolivre/ml_service.go` | Capturar json.Marshal, io.ReadAll, LoadCredentials, cache ops + SafeGo (SQ3-SQ10, SQ42-SQ43) |
| `automatiza-go/internal/handlers/whatsapp_handler.go` | Capturar instance status, QR, backpressure (SQ11-SQ14) |
| `automatiza-go/internal/connectrpc/admin_handler.go` | Capturar 13 erros suprimidos: audit trail, email, roles, duplicate check (SQ18-SQ26, SQ40) |
| `automatiza-go/internal/connectrpc/automations_handler.go` | Capturar error metadata update (SQ15) |
| `automatiza-go/internal/connectrpc/groups_handler.go` | Capturar group delete após LeaveGroup (SQ16) |
| `automatiza-go/internal/connectrpc/loadbalancer_handler.go` | Validar server ID parse (SQ51) |
| `automatiza-go/internal/connectrpc/history_handler.go` | Capturar structpb.NewStruct (SQ39) |
| `automatiza-go/internal/services/waha/waha_service.go` | Capturar session stop (SQ17) |
| `automatiza-go/internal/services/jobs/jobs_service.go` | Capturar 8 history/status updates silenciosos (SQ27-SQ32) |
| `automatiza-go/internal/auth/service.go` | Capturar RevokeAllExcept, token reuse, magic link rate limit, time.Parse + SafeGo (SQ33-SQ37, SQ44-SQ47, SQ57-SQ60) |
| `automatiza-go/internal/auth/rate_limiter.go` | Fix type assertion insegura (SQ50) |
| `automatiza-go/internal/adapters/email/smtp_adapter.go` | SafeGo para goroutine SMTP (SQ48) |
| `automatiza-go/internal/adapters/whatsapp/waha_models.go` | Capturar JSON unmarshal de campos críticos (SQ61) |
| `automatiza-go/internal/repository/postgres/automation_repository.go` | Capturar 7 json.Marshal + 14 json.Unmarshal (SQ62-SQ64) |
| `automatiza-go/internal/repository/postgres/system_event_repository.go` | Capturar payload marshal (SQ65) |
| `automatiza-go/internal/repository/postgres/webhook_log_repository.go` | Capturar payload marshal (SQ66) |
| `automatiza-go/internal/repository/postgres/action_history_repository.go` | Capturar 5 unmarshal em read paths (SQ67) |
| `automatiza-go/internal/services/profile/password_service.go` | Capturar rate limit increment (SQ68) |
| `automatiza-go/internal/services/monitoring/monitoring_service.go` | Substituir MD5 por SHA-256 (SQ52) |
| `automatiza-go/internal/services/automation/process_list.go` | Substituir math/rand por crypto/rand (SQ53) |
| `automatiza-go/internal/services/automation/automation_service.go` | Substituir math/rand por crypto/rand (SQ54) |
| `automatiza-go/internal/workers/asynq.go` | Substituir math/rand por crypto/rand (SQ55) |
| `automatiza-go/internal/workers/message_worker.go` | Capturar dispatch log update (SQ38) |
| **4th Pass - Frontend SonarQube (FSQ1-FSQ35)** | |
| `client/src/main.tsx` | Null check no root element (FSQ1) |
| `client/src/components/error-boundary.tsx` | Implementar componentDidCatch com Sentry (FSQ14) |
| `client/src/hooks/use-coupons.ts` | Clipboard try/catch (FSQ2) |
| `client/src/components/admin/dashboard/webhook-section.tsx` | Clipboard try/catch (FSQ3) |
| `client/src/hooks/use-affiliate-programs.ts` | atob() captura (FSQ4) |
| `client/src/hooks/use-automation-logs.ts` | exportCSV try/catch + Date validation (FSQ7, FSQ34) |
| `client/src/contexts/auth-context.tsx` | checkSession non-auth errors + listSessions + signOut + CSRF breadcrumb (FSQ15-FSQ17, FSQ19, FSQ33) |
| `client/src/hooks/use-session-status-sse.ts` | beforeBreadcrumb scrub de token em URL (FSQ18) |
| `client/src/hooks/use-jobs.ts` | Unsafe cast `as Error` (FSQ10-FSQ11) |
| `client/src/hooks/use-whatsapp.ts` | setInterval leak + catch breadcrumbs + event detail null check (FSQ9, FSQ12, FSQ22-FSQ25) |
| `client/src/hooks/use-dispatch-texts.ts` | Clipboard breadcrumb (FSQ26) |
| `client/src/hooks/use-affiliate-credentials-connect.ts` | JSON parse capture (FSQ29) |
| `client/src/services/api.service.ts` | Type validation on response (FSQ13) |
| `client/src/components/admin/hooks/use-plans-tab.ts` | mutateAsync try/catch (FSQ35) |
| `client/src/lib/sentry.ts` | beforeBreadcrumb para scrub de tokens SSE em URLs (FSQ18) |
| **5th Pass - Backend WAHA (W1-W44)** | |
| `automatiza-go/internal/adapters/whatsapp/waha_adapter.go` | Capturar parseMemberOperationResult false positive (W2), HTTPS validation (W39), response body truncation (W44), participants summary logging (W43), unmarshal errors (W4) |
| `automatiza-go/internal/adapters/whatsapp/waha_models.go` | Capturar 30+ json.Unmarshal silenciosos em campos críticos (W1) |
| `automatiza-go/internal/adapters/whatsapp/mapper.go` | Substituir log.Printf por zap (W36) |
| `automatiza-go/internal/adapters/whatsapp/hmac_validator.go` | Documentar HMAC disabled design (W41) |
| `automatiza-go/internal/services/whatsapp/instance_service.go` | Capturar DB sync errors (W8), cleanup failures (W9), fix time.Sleep (W7) |
| `automatiza-go/internal/services/whatsapp/message_service.go` | Capturar history update silencioso (W20) |
| `automatiza-go/internal/services/whatsapp/groups_service.go` | Capturar DB sync failures 3 locais (W23), LeaveGroup errors (W24), mascarar phone PII (W42) |
| `automatiza-go/internal/services/redirect/redirect_service.go` | Capturar invite code fetch failure (W37) |
| `automatiza-go/internal/services/monitoring/monitoring_service.go` | Capturar dispatch log (W34), substituir math/rand (W32) |
| `automatiza-go/internal/services/automation/automation_service.go` | Capturar dispatch log (W35), substituir math/rand (W33) |
| `automatiza-go/internal/handlers/waha_webhook_handler.go` | Fix string error check → errors.As (W13), HMAC disabled Sentry alert (W40), WebhookSecret validation (W27) |
| `automatiza-go/internal/handlers/sse_handler.go` | Capturar client channel full (W28), broadcast channel full CRITICAL (W29), status notifier errors (W31) |
| `automatiza-go/internal/handlers/sse_redis_bridge.go` | context.WithTimeout em Publish (W30) |
| `automatiza-go/internal/workers/webhook_consumer_worker.go` | Refatorar isRetryableError (W15), fallback event ID (W14) |
| `automatiza-go/internal/workers/message_worker.go` | Capturar timeout-assumed-sent (W17), dispatch log (W18), fix time.Sleep (W19) |
| `automatiza-go/internal/workers/groups_sync_worker.go` | ctx.Done() check no loop (W22), participant fetch breadcrumbs (W21), fix InstanceID type (W38) |
| `automatiza-go/internal/config/config.go` | Validação de WAHA config (W26), WebhookSecret check em prod (W27) |
| `automatiza-go/cmd/server/main.go` | Nil check wahaAdapter (W25), SSE bridge failure capture (W11), backfill failure capture (W12) |
| **5th Pass - Frontend WAHA (WF1-WF40)** | |
| `client/src/hooks/use-session-status-sse.ts` | Exponential backoff (WF1), max reconnect notification (WF2), Sentry breadcrumbs (WF3), JSON parse capture (WF4), token refresh (WF5), heartbeat detection (WF6) |
| `client/src/hooks/use-whatsapp.ts` | Non-404 error captures (WF7-WF8, WF11), QR max retry (WF9), refreshQrCode capture (WF10), stale closure fix (WF12), initial status retry (WF13), import polling cleanup (WF21), window global → ref (WF22), autoSync guard (WF33) |
| `client/src/hooks/groups/use-groups-messaging.ts` | Participant fetch tracking (WF14), individual send errors (WF15), image read return (WF16), AbortController (WF17) |
| `client/src/hooks/use-redirect-link.ts` | Clipboard async/catch (WF34) |
| `client/src/components/groups/group-audio-dialog.tsx` | File size validation (WF18) |
| `client/src/components/groups/group-photo-dialog.tsx` | File type/size validation (WF19) |
| `client/src/components/groups/group-message-dialog.tsx` | isSending disabled state (WF20) |
| `client/src/components/groups/group-members-dialog.tsx` | try/catch + Sentry capture (WF39) |
| `client/src/components/groups/group-connection-status.tsx` | Loading/skeleton state (WF24) |
| `client/src/components/groups/group-description-dialog.tsx` | Submit disabled validation (WF26) |
| `client/src/components/groups/group-poll-dialog.tsx` | Validation (WF38) |
| `client/src/components/groups/schedule-multiple-groups.tsx` | File size validation + type assertion fix (WF28, WF40) |
| `client/src/components/automations/automation-cards.tsx` | Date parse try/catch (WF30) |
| `client/src/components/automations/offers-preview-dialog.tsx` | Local placeholder image (WF31) |
| `client/src/lib/whatsapp-formatter.tsx` | Fix global key counter (WF27) |
| `client/src/pages/groups.tsx` | Error state (WF25) |
| `client/src/pages/whatsapp.tsx` | WAHA-specific ErrorBoundary (WF37) |
| `client/src/pages/jobs/components/jobs-table.tsx` | Date validation (WF29) |
| **6th Pass - Backend** | |
| `automatiza-go/cmd/server/main.go` | Reordenar Sentry init antes de logger (BG1-BG2), logger.Fatal→Error+Flush (BG4), cron SafeGo (BG13) |
| `automatiza-go/internal/connectrpc/interceptors.go` | Recovery interceptor → sentry.RecoverWithContext dentro do recover() (BG7) |
| `automatiza-go/internal/observability/sentry.go` | LogAndCapture rate limiting (BG11), PII redaction parcial (BG15), interceptor fallback (BG8) |
| `automatiza-go/internal/services/shopee/shopee_service.go` | Sentry capture em TODOS error paths HTTP (BG9) |
| `automatiza-go/internal/lib/urlsafety/urlsafety.go` | Sentry Warning em fail-open (BG10) |
| **6th Pass - Frontend** | |
| `client/src/main.tsx` | initSentry() ANTES de getElementById, unhandledrejection safety net (FG1, FG2) |
| `client/src/components/error-boundary.tsx` | Consolidar componentDidCatch (FG3) |
| `client/src/hooks/use-session-status-sse.ts` | 9 console.warn → breadcrumbs (FG4), addEventListener try/catch (FG5) |
| `client/src/hooks/use-whatsapp.ts` | onStatusChange defensivo (FG6), 5 console.warn → captureHookError (FG7) |
| `client/src/hooks/use-automations.ts` | Catch vazios → captureHookError (FG8) |
| `client/src/hooks/use-redirect-link.ts` | Mutation onError com detalhes (FG9) |
| `client/src/lib/connect-transport.ts` | crypto.getRandomValues try/catch (FG13) |
| `client/src/lib/sentry.ts` (novo) | safeClipboard helper (FG12), dev mode console logger (FG15) |
| Múltiplos arquivos frontend | Varredura localStorage → safeLocalStorage (FG11), catch blocks vazios (FG10) |

---

## Environment Variables Necessárias

### Backend (.env)
```
SENTRY_DSN=https://xxx@xxx.ingest.sentry.io/xxx
SENTRY_ENVIRONMENT=production|staging|development
SENTRY_TRACES_SAMPLE_RATE=0.2
SENTRY_RELEASE=<git-sha>
```

### Frontend (.env)
```
VITE_SENTRY_DSN=https://xxx@xxx.ingest.sentry.io/xxx
VITE_SENTRY_ENVIRONMENT=production|staging|development
VITE_SENTRY_TRACES_SAMPLE_RATE=0.1
VITE_SENTRY_DEV_MODE=true (opcional - loga erros no console no formato Sentry sem enviar, para dev local sem DSN)
SENTRY_AUTH_TOKEN=<org-auth-token> (apenas no build/CI para upload de source maps)
SENTRY_ORG=<org-slug>
SENTRY_PROJECT=<project-slug>
```

---

## Shared Modules (DRY/SOLID)

> Modulos reutilizaveis que DEVEM ser criados ANTES das tarefas que os consomem. Cada modulo e criado uma vez e usado por multiplas tarefas.

### Backend Go -- Modulos Compartilhados

| Modulo | Arquivo | Descricao | Criado em | Usado por |
|--------|---------|-----------|-----------|-----------|
| `InitSentry` | `internal/observability/sentry.go` | Inicializacao Sentry com `beforeSend` PII redaction parcial | T1.1 | T1.2+ |
| `SafeGo` | `internal/observability/sentry.go` | Goroutine wrapper com panic recovery + Sentry capture | T1.1 | T1.6, T2.3, T2.8, T3.3 |
| `LogAndCapture` | `internal/observability/sentry.go` | Log estruturado (Zap) + Sentry em 1 call; rate limiting local (>10 same errors/60s = 1 evento) | T1.1 | T2.2-T2.10, T3.1-T3.7 |
| `CaptureWithTags` | `internal/observability/sentry.go` | Captura Sentry com tags customizadas | T1.1 | T2.4-T2.6, T3.1 |
| `SentryInterceptor` | `internal/observability/sentry_interceptor.go` | ConnectRPC interceptor: classifica erros por gRPC code, filtra esperados | T1.4 | Phase 2+ (todos ConnectRPC handlers) |
| `SentryAsynqMiddleware` | `internal/observability/sentry_asynq.go` | Wrapper Asynq handlers: panic recovery + error capture + tags (worker, task_type, retry_count) | T2.1 | T2.1 (wrap 7 workers) |

### Frontend React -- Modulos Compartilhados

| Modulo | Arquivo | Descricao | Criado em | Usado por |
|--------|---------|-----------|-----------|-----------|
| `initSentry` | `src/lib/sentry.ts` | Init Sentry com `beforeSend` PII redaction, `beforeBreadcrumb` token scrub | T4.1 | T4.2+ |
| `captureHookError` | `src/lib/sentry.ts` | Captura erros de hooks filtrando esperados (InvalidArgument, Unauthenticated, AbortError) | T4.1 | T5.2-T5.12 |
| `setSentryUser` | `src/lib/sentry.ts` | Set/clear user context no Sentry | T4.1 | T4.4 |
| `safeLocalStorage` | `src/lib/sentry.ts` | Wrapper localStorage com try/catch (resolve crash em private browsing) | T4.1 | T5.3, T5.11 |
| `safeClipboard` | `src/lib/sentry.ts` | Wrapper clipboard com fallback `execCommand('copy')` + Sentry capture | T4.1 | T5.6, T5.11 |
| `sentryConnectInterceptor` | `src/lib/sentry-connect-interceptor.ts` | ConnectRPC interceptor: classifica erros por gRPC code, filtra esperados | T5.1 | T5.2+ (todos hooks com ConnectRPC) |

### Grafo de Dependencias

```
Phase 1 T1.1 (sentry.go) ─────┬── Phase 1 T1.2-T1.8 (main.go + infra)
                               ├── Phase 2 T2.1-T2.10 (workers + services)
                               └── Phase 3 T3.1-T3.10 (silent errors)

Phase 1 T1.4 (SentryInterceptor) ── Phase 2+ (todos ConnectRPC handlers)
Phase 2 T2.1 (SentryAsynqMiddleware) ── Phase 2 T2.1 (wrap 7 workers)

Phase 4 T4.1 (sentry.ts) ─────┬── Phase 4 T4.2-T4.5 (boundaries + auth)
                               └── Phase 5 T5.1-T5.12 (hooks + components)

Phase 5 T5.1 (sentryConnectInterceptor) ── Phase 5 T5.2+ (hooks ConnectRPC)
```

---

## Working Phases

### Phase 1 — Backend Go: Sentry Core + Middleware + Interceptors

**Objetivo:** Sentry funcionando no backend capturando panics, 5xx, e erros ConnectRPC automaticamente.

**Depende de:** Nenhuma (primeira phase do plano)
**Desbloqueia:** Phase 2 (workers + services), Phase 3 (erros silenciosos)

**Tasks:**

#### T1.1 — Instalar SDK + Criar Modulo Core Sentry

- **Entrada -> Saida:** `go.mod` sem Sentry -> modulo `internal/observability/sentry.go` com SDK instalado e helpers prontos
- **Arquivos:** criar: [`internal/observability/sentry.go`], modificar: [`go.mod`, `go.sum`]
- **DoD:** `InitSentry` inicializa com DSN/env/release; `beforeSend` redacta PII (email, tokens, IPs); `SafeGo` faz panic recovery + Sentry capture em goroutines; `LogAndCapture` faz log estruturado (Zap) + Sentry em 1 call com rate limiting local (>10 same errors/60s = 1 evento); `CaptureWithTags` e `CaptureWithScope` disponveis como helpers
- **Teste minimo:** Unit test: `InitSentry` com DSN invalido retorna erro; `beforeSend` redacta campos PII; `LogAndCapture` rate limiting suprime apos threshold
- **Cobre:** Steps 1-2, BG11 (LogAndCapture rate limiting), BG15 (PII redaction parcial)
- **Depende de:** —

#### T1.2 — Bootstrap: Sentry Init Antes de Logger + Shutdown Safety

- **Entrada -> Saida:** `main.go` sem Sentry -> `main.go` com init Sentry ANTES de logger, shutdown seguro, WaitGroup, e startup cleanup
- **Arquivos:** modificar: [`cmd/server/main.go`]
- **DoD:** Ordem de init: config load -> extrair DSN -> `sentry.Init()` com DSN minimo -> criar logger -> continuar setup; `defer sentry.Flush(2*time.Second)` no main; `logger.Fatal` substituido por `logger.Error` + `sentry.CaptureException` + `sentry.Flush` + `os.Exit(1)` (BG4 CRITICO); `sync.WaitGroup` para tracking goroutines; cadeia de defers reestruturada para Redis/DB fechar em qualquer falha; `cronService.Stop()` com context + WaitGroup
- **Teste minimo:** Integration test: verificar que `sentry.Flush` executa antes de shutdown; verificar que falha de startup fecha Redis/DB
- **Efeito colateral / Mitigacao:** Reordenacao de init pode quebrar dependencias de logger em erros pre-init; mitigacao: fallback para `fmt.Fprintf(os.Stderr, ...)` se logger indisponivel
- **Cobre:** Steps 3, 10-12, 17, 24-25, W10, BG1-BG2, BG4, N11-N13
- **Depende de:** T1.1

#### T1.3 — Gin Middleware: sentrygin + error_handler

- **Entrada -> Saida:** Gin router sem Sentry middleware -> `sentrygin` como PRIMEIRO middleware + error_handler capturando 5xx
- **Arquivos:** modificar: [`cmd/server/main.go`, `internal/middleware/error_handler.go`]
- **DoD:** `sentrygin.New(sentrygin.Options{Repanic: true})` registrado como primeiro middleware; cobre TODAS as rotas Gin incluindo `/health`, `/webhooks/*`; error_handler captura 5xx com `hub.CaptureException(err)` e scope com request context; sem duplicacao com `RecoveryMiddleware`
- **Teste minimo:** Integration test: request que gera 5xx aparece como evento Sentry com request context; panic em rota Gin e capturado
- **Cobre:** Steps 4, 26, BG5-BG6
- **Depende de:** T1.2

#### T1.4 — ConnectRPC SentryInterceptor + Recovery Integration

- **Entrada -> Saida:** ConnectRPC sem observabilidade Sentry -> interceptor classifica erros por gRPC code + Recovery envia panics ao Sentry
- **Arquivos:** criar: [`internal/observability/sentry_interceptor.go`], modificar: [`internal/connectrpc/interceptors.go`, `internal/connectrpc/suspension_interceptor.go`]
- **DoD:** `SentryInterceptor` implementa `UnaryInterceptorFunc` + `StreamingHandlerInterceptorFunc`; classifica: Ignorar (InvalidArgument, Unauthenticated, NotFound, Canceled, AlreadyExists), Warning (PermissionDenied, ResourceExhausted, FailedPrecondition), Error (Internal, Unknown, DataLoss, Unavailable, DeadlineExceeded, Unimplemented, Aborted); tags: `procedure`, `grpc_code`, `trace_id`, `user_id`; posicao na chain: APOS Tracing, ANTES de Logging; Recovery interceptor usa `sentry.CurrentHub().RecoverWithContext(ctx, r)` DENTRO do bloco `recover()` ANTES de retornar ConnectError; suspension fail-open (B20-B21) captura com `sentry.CaptureException`
- **Teste minimo:** Unit test: erro InvalidArgument NAO gera evento; erro Internal gera evento com tags corretas; panic no handler e capturado via Recovery
- **Cobre:** Steps 5-7, 27, BG7-BG8
- **Depende de:** T1.1

#### T1.5 — DB Pool + Config + Env Validation

- **Entrada -> Saida:** DB pool sem monitoring e config sem validacao -> pool com stat monitoring, statement_timeout, env vars validadas
- **Arquivos:** modificar: [`internal/repository/postgres/db.go`, `internal/config/config.go`]
- **DoD:** Goroutine periodica verifica `pool.Stat()` e reporta ao Sentry se pool > 80% utilizado; `statement_timeout=30000` (30s) configurado em `RuntimeParams`; `LoadConfig()` valida env vars obrigatorias antes de qualquer conexao; `WebhookSecret` vazio em producao emite Sentry Error; WAHA config validada
- **Teste minimo:** Unit test: `LoadConfig` com env var obrigatoria ausente retorna erro; pool > 80% gera evento Sentry
- **Efeito colateral / Mitigacao:** `statement_timeout` pode matar queries legitimamente lentas; mitigacao: 30s e conservador, queries de relatorio devem usar timeout override
- **Cobre:** Steps 8-9, 13, 21, C9, N8, N14, W26-W27
- **Depende de:** T1.1

#### T1.6 — SafeGo em Goroutines Criticas do main.go

- **Entrada -> Saida:** Goroutines `go func()` sem panic recovery -> todas wrapeadas com `observability.SafeGo`
- **Arquivos:** modificar: [`cmd/server/main.go`]
- **DoD:** `SafeGo` aplicado em: HTTP server (`srv.ListenAndServe`), queue server (`queueServer.Start`), cron goroutines, backfill; qualquer panic nessas goroutines e capturado com Sentry + flush
- **Teste minimo:** Unit test: goroutine que faz panic dentro de `SafeGo` gera evento Sentry e nao crasha o processo
- **Cobre:** Steps 3[S66], 15, 28, SQ49, BG13
- **Depende de:** T1.1, T1.2

#### T1.7 — Startup Error Captures + WAHA Nil Check

- **Entrada -> Saida:** 6 `defer Close` silenciosos + erros de startup ignorados -> todos capturados com Sentry + WAHA nil-safe
- **Arquivos:** modificar: [`cmd/server/main.go`]
- **DoD:** 6 ocorrencias de `defer func() { _ = x.Close() }()` substituidas por versao com `LogAndCapture`; `strconv.Atoi(pgCfg.Port)` validado com Sentry capture em erro; SSE bridge subscribe failure capturado como Error; backfill failure capturado com tags `{module: "backfill", flow: "startup"}`; `wahaAdapter` nil check antes de uso em handlers (NullObject pattern ou guard)
- **Teste minimo:** Unit test: `Close` que retorna erro gera log + Sentry capture; nil wahaAdapter nao causa panic
- **Cobre:** Steps 3[S15-S20, S31], 18-20, W11-W12, W25
- **Depende de:** T1.1, T1.2

#### T1.8 — WAHA Security Validations + Rate Limiter Safety

- **Entrada -> Saida:** HMAC disabled sem alerta + HTTPS nao enforced + rate limiter com type assertion insegura -> alertas Sentry + validacoes seguras
- **Arquivos:** modificar: [`internal/handlers/waha_webhook_handler.go`, `internal/adapters/whatsapp/waha_adapter.go`, `internal/auth/rate_limiter.go`]
- **DoD:** HMAC disabled em producao emite `sentry.CaptureMessage` Error com tag `{security: "hmac-disabled"}`; construtor WAHA valida `WAHA_API_URL` comeca com `https://` em producao (Warning se nao); rate limiter `result.(int64)` substituido por assertion com ok check — se falha, `sentry.CaptureMessage` + fallback seguro (CRITICO: evita panic em runtime)
- **Teste minimo:** Unit test: HMAC disabled em env=production gera evento Sentry; type assertion com valor inesperado nao causa panic
- **Rollback:** Rate limiter assertion: se ok check falhar, usar valor default 0 (permite request) em vez de panic
- **Cobre:** Steps 16, 22-23, W39-W40, SQ50
- **Depende de:** T1.1

**Commit Checkpoint:** `feat(sentry): add sentry core integration to go backend`

---

### Phase 2 — Backend Go: Workers + Services + Integrações Externas

**Objetivo:** Capturar erros em background jobs (Asynq), chamadas a APIs externas, e webhook processing com contexto rico.

**Depende de:** Phase 1 (T1.1 core module, T1.4 SentryInterceptor)
**Desbloqueia:** Phase 3 (erros silenciosos), Phase 6 (validacao end-to-end)

**Tasks:**

#### T2.1 — Asynq Worker Middleware

- **Entrada -> Saida:** 7 workers Asynq sem observabilidade Sentry -> todos wrapeados com `SentryAsynqMiddleware` + SMTP adapter SafeGo
- **Arquivos:** criar: [`internal/observability/sentry_asynq.go`], modificar: [`cmd/server/main.go`]
- **DoD:** `SentryAsynqMiddleware(handlerName, handler)` clona hub Sentry, faz panic recovery com `hub.RecoverWithContext`, captura erros com tags (`worker`, `task_type`, `retry_count`, `queue`), max retries atingido = level Critical + tag `dlq=true`; todos 7 workers registrados: webhook_consumer, message, groups_sync, reconciliation, dispatch_log, monitoring, message_queue; SMTP adapter goroutine wrapeada com SafeGo
- **Teste minimo:** Unit test: handler que retorna erro gera evento Sentry com tags corretas; handler que faz panic e capturado; max retries gera level Critical
- **Cobre:** Steps 1-2, 30, 43, SQ48
- **Depende de:** T1.1

#### T2.2 — Webhook Handler Captures

- **Entrada -> Saida:** Webhook handler com erros parcialmente capturados -> todos error paths com Sentry + request size limit + string error refactor
- **Arquivos:** modificar: [`internal/handlers/waha_webhook_handler.go`]
- **DoD:** HMAC failure capturado com tags `security=hmac_failure`, `ip=<remote_addr>` (B22); JSON parse com tag `webhook=parse_error` (B24); idempotency check failure (B25); enqueue failure level Critical (B26); mark failed com extra `event_id` (B27); `http.MaxBytesReader` 10MB em todos endpoints webhook (N4); string error check refatorado para `errors.As` (W13); fallback event ID usa `crypto/rand` (W14)
- **Teste minimo:** Unit test: HMAC failure gera evento com tag security; payload > 10MB retorna 413; enqueue failure gera Critical
- **Efeito colateral / Mitigacao:** Request size limit pode rejeitar webhooks legitimos com media grande; mitigacao: 10MB e conservador para webhooks (media vem via URL, nao inline)
- **Cobre:** Steps 3, 17, 22, 34, 36
- **Depende de:** T1.1

#### T2.3 — SSE & Real-time Captures

- **Entrada -> Saida:** SSE bridge/hub com goroutines desprotegidas e erros silenciosos -> SafeGo em todas goroutines + captures em channel saturation + pubsub resilience
- **Arquivos:** modificar: [`internal/handlers/sse_handler.go`, `internal/handlers/sse_redis_bridge.go`, `internal/handlers/stubs.go`]
- **DoD:** SSE bridge goroutine usa `SafeGo("sse-redis-bridge", ...)` (S68); hub goroutine usa `SafeGo("sse-hub", ...)` (C1); broadcast channel full capturado como Critical (W29 — afeta TODOS users); client channel full capturado como Warning com rate-limiting (W28); pubsub close race fix: cancelar context antes de fechar pubsub (C8); pubsub reconnect com backoff exponencial (N10); publish usa `context.WithTimeout` (W30); stubs: tratar erros de `GetInstanceByUserAndName` e `GetRecent` (S24-S27); monitoring goroutine usa `SafeGo("monitoring-processor", ...)` (S67); status notifier errors capturados (W31)
- **Teste minimo:** Unit test: broadcast channel full gera Critical; pubsub reconnect apos falha com backoff; SafeGo em hub captura panic
- **Efeito colateral / Mitigacao:** Reconnect com backoff pode causar gap temporario em SSE events; mitigacao: client-side retry ja existe
- **Cobre:** Steps 4-5, 9, 12, 20, 34-35
- **Depende de:** T1.1

#### T2.4 — External API Services (ML, AI, Shopee, Email, Kiwify)

- **Entrada -> Saida:** Servicos externos com erros parcialmente capturados -> todos error paths HTTP com Sentry + performance tracing + SafeGo em goroutines
- **Arquivos:** modificar: [`internal/services/mercadolivre/ml_service.go`, `internal/services/ai/ai_service.go`, `internal/services/shopee/shopee_service.go`, `internal/services/email/email_service.go`, `internal/services/kiwify/kiwify_service.go`, `internal/services/reconciliation/reconciliation_service.go`]
- **DoD:** ML: marshal/readall/creds/cache capturados (SQ3-10), SafeGo em SaveCredentials (SQ42-43), duplicate check (S82), response limit MaxBytesReader (B43-44); AI: marshal/readall capturados (SQ1-2), circuit breaker candidate tag (B45); Shopee: TODOS error paths HTTP com tags `{external: "shopee", endpoint, status_code}` (B47, BG9); Email: capture (B48); Kiwify: captures + regex (B41-42, S61); Reconciliation: Critical level (B40); URL safety: Warning em fail-open (BG10); padrao HTTP: `sentry.StartSpan` antes, `span.Finish()` apos, tags em erro
- **Teste minimo:** Unit test por servico: chamada HTTP que falha gera evento Sentry com tags `external_service` e `http_status`; ML SaveCredentials panic e capturado
- **Cobre:** Steps 6, 8, 18-19, 21-22, 41-42
- **Depende de:** T1.1

#### T2.5 — WAHA Adapter Resilience

- **Entrada -> Saida:** WAHA adapter sem limites de conexao/tamanho/timeout/retry -> adapter resiliente com transport configurado + limites + retry + capture de false positive
- **Arquivos:** modificar: [`internal/adapters/whatsapp/waha_adapter.go`]
- **DoD:** `http.Transport{MaxIdleConns: 100, MaxConnsPerHost: 20}` configurado (C6); `http.MaxBytesReader(nil, resp.Body, 50*1024*1024)` em downloads (N1); `context.WithTimeout` por operacao: 10s status, 30s downloads (N2); retry com backoff para `ErrCodeNetworkError`: 3 retries, 100ms-3s (N3); HTTPS enforcement no construtor em producao (W39); `parseMemberOperationResult` captura body vazio retornando success (W2 — data corruption em promote/demote)
- **Teste minimo:** Unit test: response > 50MB retorna erro; retry apos network error funciona 3x com backoff; body vazio em parseMemberOperationResult gera Warning
- **Efeito colateral / Mitigacao:** MaxConnsPerHost pode limitar throughput em pico de envio; mitigacao: 20 e suficiente para uso normal, aumentar se telemetria indicar bottleneck
- **Rollback:** Se retry causa duplicacao de mensagens, desativar via env var `WAHA_RETRY_ENABLED=false`
- **Cobre:** Steps 11, 14-16, 40
- **Depende de:** T1.1

#### T2.6 — ConnectRPC Handler Captures

- **Entrada -> Saida:** ConnectRPC handlers com erros suprimidos -> todos error paths capturados com contexto
- **Arquivos:** modificar: [`internal/connectrpc/webhooks_handler.go`, `internal/connectrpc/admin_handler.go`, `internal/connectrpc/automations_handler.go`, `internal/connectrpc/groups_handler.go`, `internal/connectrpc/loadbalancer_handler.go`, `internal/connectrpc/history_handler.go`, `internal/services/waha/waha_service.go`]
- **DoD:** Webhooks: 7 ocorrencias de `webhookLogRepo.Create()`/`UpdateStatus()` capturadas (S62-S65); Admin: 13 erros suprimidos capturados — audit trail (SQ20, SQ24-SQ25), email (SQ21), roles (SQ22), duplicate check (SQ23), profile enrichment (SQ18, SQ26), webhook simulation logs (SQ19); Automations: error metadata update (SQ15); Groups: delete apos LeaveGroup (SQ16); LoadBalancer: server ID parse validado em 3 ocorrencias (SQ51); WAHA service: session stop failure (SQ17)
- **Teste minimo:** Unit test: webhookLogRepo.Create que falha gera evento Sentry; admin audit trail failure gera capture com tags
- **Cobre:** Steps 7, 24-27, 31
- **Depende de:** T1.1, T1.4

#### T2.7 — Jobs Service History/Status

- **Entrada -> Saida:** Jobs service com 8 history/status updates silenciosos -> todos capturados
- **Arquivos:** modificar: [`internal/services/jobs/jobs_service.go`]
- **DoD:** 8 captures: history target_count update (SQ27), history status updates (SQ28-SQ29), job status update (SQ30), scheduled job status (SQ31), automation next_run_at (SQ32); cada capture com tags `{module: "jobs", operation: "<specific>"}`
- **Teste minimo:** Unit test: history status update que falha gera evento Sentry com tag operation
- **Cobre:** Step 28
- **Depende de:** T1.1

#### T2.8 — Message Worker Captures

- **Entrada -> Saida:** Message worker com timeout silencioso e dispatch log ignorado -> captures com Warning/Error + time.Sleep seguro
- **Arquivos:** modificar: [`internal/workers/message_worker.go`]
- **DoD:** Timeout-assumed-sent capturado como Warning com tags `{flow: "message-send", reason: "timeout-assumed-sent"}` (W17); dispatch log creation `_, _ =` capturado com tags `{module: "message-worker"}` (W18, SQ38); `time.Sleep` substituido por `time.After` com select para ctx.Done() (W19)
- **Teste minimo:** Unit test: timeout no envio gera Warning Sentry; dispatch log failure gera capture; cancelamento via ctx.Done() interrompe sleep
- **Efeito colateral / Mitigacao:** `time.After` com select pode mudar timing de envio; mitigacao: manter mesmo delay, apenas adicionar cancelabilidade
- **Cobre:** Steps 29, 32-33
- **Depende de:** T1.1

#### T2.9 — Groups Sync Worker + Dispatch Logs + Redirect

- **Entrada -> Saida:** Groups sync sem ctx.Done() check + dispatch logs silenciosos em monitoring/automation + redirect sem capture -> todos corrigidos
- **Arquivos:** modificar: [`internal/workers/groups_sync_worker.go`, `internal/services/monitoring/monitoring_service.go`, `internal/services/automation/automation_service.go`, `internal/services/redirect/redirect_service.go`]
- **DoD:** Groups sync: `select { case <-ctx.Done(): return ctx.Err() default: }` no loop de processamento (W22); participant fetch com breadcrumbs (W21); InstanceID type validado (W38); Monitoring + Automation: `_, _ = s.dispatchLogRepo.Create()` capturado em ambos (W34-W35); Redirect: invite code failure capturado com tags `{module: "redirect", flow: "invite-code"}` (W37)
- **Teste minimo:** Unit test: ctx cancelado interrompe loop de sync; dispatch log failure gera capture; invite code failure gera capture
- **Cobre:** Steps 37-39
- **Depende de:** T1.1

#### T2.10 — Cron + Watchdog SafeGo

- **Entrada -> Saida:** Cron tasks e watchdog com goroutines desprotegidas -> SafeGo em ambos
- **Arquivos:** modificar: [`internal/services/cron/cron_service.go`, `internal/services/watchdog/watchdog.go`]
- **DoD:** Cron: `go s.runTask(task)` substituido por `observability.SafeGo("cron-task-"+task.Name, ...)` (C2); Watchdog: `go s.run(ctx)` substituido por `observability.SafeGo("watchdog", ...)` (C10); qualquer panic nessas goroutines e capturado com Sentry + flush
- **Teste minimo:** Unit test: cron task que faz panic gera evento Sentry com tag contendo task name; watchdog panic e capturado
- **Cobre:** Steps 10, 13
- **Depende de:** T1.1

**Commit Checkpoint:** `feat(sentry): add sentry to workers, handlers and external integrations`

---

### Phase 3 — Backend Go: Erros Silenciosos (Auth, Cache, History, Goroutines)

**Objetivo:** Eliminar TODOS os erros silenciosos descobertos na auditoria profunda. Zero `_ = err` sem observabilidade.

**Depende de:** Phase 1 (SafeGo, LogAndCapture, CaptureWithTags), Phase 2 (SentryAsynqMiddleware)
**Desbloqueia:** Phase 6 (validation)

**Tasks:**

#### T3.1 — Auth Security Errors (CRITICOS)

- **Entrada -> Saida:** Erros silenciosos em rate limiter, session revocation e password service -> Todos capturados com level Critical/Alto via CaptureWithTags
- **Arquivos:** criar: [], modificar: [`internal/auth/service.go`, `internal/services/profile/password_service.go`]
- **DoD:** Rate limiter increment/reset captured Critical (S1-S3); session RevokeAllForUser Critical (S4); ChangePassword RevokeAllExcept CRITICO (SQ33); magic link rate limit CRITICO (SQ35); session revocation workaround ALTO (SQ37); password service rate limit increment ALTO (SQ68). Zero `_ = err` restante nesses fluxos.
- **Teste minimo:** Test que simula rate limiter failure e verifica evento Sentry capturado com tags corretas e level Critical.
- **Efeito colateral / Mitigacao:** Pode gerar volume alto de eventos se rate limiter Redis estiver instavel; mitigado pelo rate limiting do LogAndCapture (T3.10/BG11).
- **Cobre:** S1-S4, SQ33, SQ35, SQ37, SQ68
- **Depende de:** T1.2 (CaptureWithTags)

#### T3.2 — Auth Token, Activity & Propagacao de Erros

- **Entrada -> Saida:** Erros ignorados em UpdateActivity, resetFailedAttempts, markTokenUsed, getUserByEmail, user_roles -> Warning capturado ou erro propagado corretamente
- **Arquivos:** criar: [], modificar: [`internal/auth/service.go`]
- **DoD:** UpdateActivity Warning (S5); resetFailedAttempts Warning (S6); markPasswordResetTokenUsed Warning+security tag (S7); getUserByEmail erro propagado (S8); user_roles query propagado (S9); email verification token reuse Warning (SQ34); magic link token reuse Warning (SQ36).
- **Teste minimo:** Test que verifica getUserByEmail error propagation (retorna erro em vez de nil silencioso).
- **Cobre:** S5-S9, SQ34, SQ36
- **Depende de:** T1.2 (LogAndCapture)

#### T3.3 — Auth Goroutines SafeGo + time.Parse Security

- **Entrada -> Saida:** 4 goroutines sem recover + 7 time.Parse sem validacao -> SafeGo em todas goroutines + validacao de timestamps criticos de seguranca
- **Arquivos:** criar: [], modificar: [`internal/auth/service.go`]
- **DoD:** SafeGo aplicado em password migration (SQ44), verification email (SQ45), reset email (SQ46), magic link (SQ47); LockedUntil time.Parse validado CRITICO (SQ57); Token ExpiresAt time.Parse validado ALTO 3x (SQ59); session timestamps validados (SQ58, SQ60). Nenhum goroutine sem recover no auth service.
- **Teste minimo:** Test que verifica panic recovery em goroutine SafeGo e captura Sentry do panic; test que verifica time.Parse invalido retorna erro em vez de zero time.
- **Efeito colateral / Mitigacao:** time.Parse strict pode rejeitar timestamps que antes eram aceitos como zero; mitigado por log Warning antes de rejeitar.
- **Cobre:** SQ44-SQ47, SQ57, SQ59, SQ58, SQ60
- **Depende de:** T1.3 (SafeGo)

#### T3.4 — Session Cache (fail-open)

- **Entrada -> Saida:** 5 pontos de `_ = cacheSession/redis.Del` -> LogAndCapture Warning com semantica fail-open explicita
- **Arquivos:** criar: [], modificar: [`internal/auth/session.go`]
- **DoD:** 5 cache operations capturadas via LogAndCapture Warning; fail-open semantics com tag `{cache_policy: "fail-open"}`; request nunca bloqueado por falha de cache mas sempre reportado.
- **Teste minimo:** Test que simula Redis down e verifica que session validation continua funcionando (fail-open) + evento Sentry emitido.
- **Cobre:** S10-S14, BG12
- **Depende de:** T1.2 (LogAndCapture)

#### T3.5 — History Updates Fire-and-Forget

- **Entrada -> Saida:** 12 `_ = historyRepo.Update` + 4 DB sync silenciosos em services WhatsApp -> LogAndCapture Warning mantendo fire-and-forget
- **Arquivos:** criar: [], modificar: [`internal/services/whatsapp/message_service.go`, `internal/services/automation/automation_service.go`, `internal/services/whatsapp/instance_service.go`, `internal/services/whatsapp/groups_service.go`]
- **DoD:** 12 historyRepo.Update errors capturados (S32-S43); instance logout DB sync capturado (W8); instance cleanup failure com tag `{flow: "create-cleanup"}` (W9); message service history (W20); groups service DB sync 3 locations (W23). Fluxo principal nunca interrompido.
- **Teste minimo:** Test que simula historyRepo.Update failure e verifica que dispatch continua + evento Sentry emitido com tags module/operation/history_id.
- **Efeito colateral / Mitigacao:** Alto volume de eventos se DB estiver degradado; mitigado por rate limiting do LogAndCapture (T3.10/BG11).
- **Cobre:** S32-S43, W8, W9, W20, W23
- **Depende de:** T1.2 (LogAndCapture)

#### T3.6 — Cache Operations Silenciosas

- **Entrada -> Saida:** 13 `_ = cache.Set/SetJSON/Delete` em 6 services -> LogAndCapture com nivel diferenciado (Warning para dedup, Info para read cache)
- **Arquivos:** criar: [], modificar: [`internal/services/monitoring/monitoring_service.go`, `internal/services/shopee/shopee_service.go`, `internal/services/automation/automation_service.go`, `internal/services/loadbalancer/loadbalancer_service.go`, `internal/handlers/sse_handler.go`]
- **DoD:** 13 cache ops capturadas; dedup cache (S44, S46, S49) = Warning (risco de duplicatas); read cache (S47-S48, S50-S55) = Info (degradacao de performance); tag `{cache_policy: "fail-open"}` em todos.
- **Teste minimo:** Test que verifica nivel correto (Warning vs Info) para dedup cache failure vs read cache failure.
- **Cobre:** S44-S56, BG12
- **Depende de:** T1.2 (LogAndCapture)

#### T3.7 — JSON/Parsing + Repository Errors

- **Entrada -> Saida:** json.Marshal/Unmarshal silenciosos em 8+ arquivos + repository errors nao propagados -> Erros propagados ou capturados com contexto
- **Arquivos:** criar: [], modificar: [`internal/repository/postgres/action_history_repository.go`, `internal/services/mllinklists/service.go`, `internal/services/kiwify/kiwify_service.go`, `internal/repository/postgres/profile_repository.go`, `internal/services/loadbalancer/loadbalancer_service.go`, `internal/repository/postgres/settings_repository.go`, `internal/repository/postgres/coupon_repository.go`, `internal/adapters/whatsapp/waha_models.go`, `internal/repository/postgres/automation_repository.go`, `internal/repository/postgres/system_event_repository.go`, `internal/repository/postgres/webhook_log_repository.go`, `internal/connectrpc/history_handler.go`, `internal/connectrpc/admin_handler.go`, `internal/lib/urlsafety/urlsafety.go`]
- **DoD:** json.Marshal/Unmarshal errors capturados ou propagados; password update propagado CRITICO (S77); instanceID parse com fallback 0 (S78-S79); CountByServer com fallback 0 (S80-S81); type assertions validadas (S28); time.Parse com retorno de erro (S29); WAHA models 30+ unmarshal capturados (SQ61); automation repo 21 marshal/unmarshal (SQ62-64); system_event/webhook_log/action_history repos (SQ65-67); structpb.NewStruct (SQ39-40); urlsafety cache (SQ41).
- **Teste minimo:** Test que verifica password update error propagation (S77) e que structpb.NewStruct failure eh capturado.
- **Cobre:** S57-S61, S77-S82, S28-S29, SQ61, SQ62-SQ67, SQ39-SQ41
- **Depende de:** T1.2 (LogAndCapture, CaptureWithTags)

#### T3.8 — Concurrency Fixes (Race Conditions)

- **Entrada -> Saida:** 4 race conditions identificadas (map access, HMAC init, adapter cache, ticker leak) -> Corrigidos com sync primitives e context awareness
- **Arquivos:** criar: [], modificar: [`internal/services/watchdog/watchdog.go`, `internal/auth/tokens.go`, `internal/adapters/whatsapp/waha_adapter.go`, `internal/services/cron/cron_service.go`]
- **DoD:** Watchdog map access sob lock reconnectMu (C3); HMAC init via sync.Once com Sentry capture em falha (C4); adapter statusCache sob statusCacheMu (C5); tickers cron com stop context-aware e monitoramento de goroutine count (C7).
- **Teste minimo:** Test com `-race` flag que verifica acesso concorrente ao watchdog map e adapter cache sem data race.
- **Efeito colateral / Mitigacao:** sync.Once no HMAC pode mascarar init failure repetida; mitigado por Sentry capture no primeiro failure + panic se critical.
- **Rollback:** Se lock contention causar latencia, reverter para atomic operations onde possivel.
- **Cobre:** C3, C4, C5, C7
- **Depende de:** T1.1 (observability package)

#### T3.9 — Context Cancellation + Infrastructure

- **Entrada -> Saida:** 5 context cancellation gaps + transaction audit + unbounded queries/cache + deadlock detection -> Context respected em todos loops/workers + guards de seguranca
- **Arquivos:** criar: [], modificar: [`internal/workers/webhook_consumer_worker.go`, `internal/handlers/sse_redis_bridge.go`, `internal/middleware/webhook_rate_limiter.go`, `internal/services/cron/cron_service.go`, `internal/services/reconciliation/reconciliation_service.go`, `internal/repository/postgres/db.go`, `internal/repository/postgres/webhook_events_repository.go`, `internal/lib/cache/cache.go`]
- **DoD:** 5 context cancellation checks implementados (S69-73); tx commit com defer rollback auditado e tag Sentry (N7); GetPendingEvents limit guard max 1000 (N9); cache size monitoring > 50k entries (N15); deadlock PG 40P01 detection (N17); webhook body size errors.As (W13); Redis bridge WithTimeout 5s (W30).
- **Teste minimo:** Test que verifica context cancellation interrompe webhook_consumer loop; test que verifica limit guard em GetPendingEvents.
- **Cobre:** S69-S73, N7, N9, N15, N17, W13, W30
- **Depende de:** T1.2 (LogAndCapture)

#### T3.10 — Crypto Fixes + PII Masking + Config Warnings + LogAndCapture Rate Limiting

- **Entrada -> Saida:** math/rand inseguro + MD5 + PII em logs + config warnings + risco de event flood -> crypto/rand + SHA-256 + PII masking + config observabilidade + rate limiting interno
- **Arquivos:** criar: [], modificar: [`internal/services/monitoring/monitoring_service.go`, `internal/services/whatsapp/process_list_service.go`, `internal/services/automation/automation_service.go`, `internal/workers/asynq_workers.go`, `internal/services/watchdog/watchdog.go`, `internal/adapters/whatsapp/mapper.go`, `internal/services/whatsapp/groups_service.go`, `internal/config/config.go`, `internal/observability/sentry.go`]
- **DoD:** math/rand substituido por crypto/rand em 5 services (SQ52-56); MD5 substituido por SHA-256 (SQ52); phone PII masking `+55 11 ****-****` (W42); mapper log.Printf substituido por zap (W36); config warnings para viper/rows.Close (S21-23); LogAndCapture rate limiting: mesmo erro >10x em 60s captura apenas 1 evento com `extra: {suppressed_count: N}` (BG11); PII redaction parcial em sentry.go preservando error types/stack frames (BG15).
- **Teste minimo:** Test que verifica crypto/rand output entropy; test que verifica PII masking preserva country code + DDD; test que verifica rate limiting suprime eventos repetidos.
- **Efeito colateral / Mitigacao:** Rate limiting pode suprimir eventos legitimos distintos com mesma mensagem; mitigado por key incluir file:line alem de err.Error().
- **Cobre:** S21-S23, SQ52-SQ56, W42, W36, BG11, BG15
- **Depende de:** T1.1 (observability package base)

**Commit Checkpoint:** `fix(observability): eliminate all silent errors in go backend`

---

### Phase 4 — Frontend React: Sentry Core + Error Boundaries

**Objetivo:** Sentry capturando erros JS unhandled, render errors, e fornecendo user context.

**Depende de:** Nenhuma (frontend independente do backend)
**Desbloqueia:** Phase 5 (hooks + components)

**Tasks:**

#### T4.1 — Instalar SDK + Criar Modulo Core Sentry Frontend

- **Entrada -> Saida:** Projeto sem Sentry frontend -> `client/src/lib/sentry.ts` com initSentry(), helpers de captura, PII scrubbing e dev mode; SDK instalado; main.tsx com init ordering correto
- **Arquivos:** criar: [`client/src/lib/sentry.ts`], modificar: [`client/package.json`, `client/src/main.tsx`]
- **DoD:** `@sentry/react` + `@sentry/vite-plugin` instalados; `initSentry()` exportado com Sentry.init() configurado (dsn, environment, release, browserTracing, replay com maskAllText/blockAllMedia, tracesSampleRate, beforeSend com PII redaction, denyUrls, ignoreErrors); `beforeBreadcrumb` scrub de tokens SSE em URLs (FSQ18); dev mode console logger quando `VITE_SENTRY_DEV_MODE=true` sem DSN (FG15); `captureHookError`, `setSentryUser`, `safeLocalStorage`, `safeClipboard` helpers exportados; `initSentry()` chamado ANTES de `getElementById('root')` (FG1); `window.addEventListener('unhandledrejection')` safety net ANTES de Sentry.init (FG2); root null check com `Sentry.captureException` se ausente (FSQ1); guard para DSN ausente em producao.
- **Teste minimo:** Test que verifica initSentry() nao lanca com DSN vazio; test que verifica beforeSend remove token de query params; test que verifica safeLocalStorage retorna null em fallback.
- **Cobre:** Steps 1-3, 10, 13-15, 17 (originais); FSQ1, FSQ18, FG1, FG2, FG15
- **Depende de:** --

#### T4.2 — Error Boundaries

- **Entrada -> Saida:** ErrorBoundary existente sem reporte + rotas sem protecao Sentry -> ErrorBoundary reportando via Sentry + Sentry.ErrorBoundary em rotas e secoes criticas
- **Arquivos:** criar: [], modificar: [`client/src/app.tsx`, `client/src/components/error-boundary.tsx`, `client/src/pages/groups.tsx`, `client/src/pages/jobs/jobs-page.tsx`]
- **DoD:** `Sentry.ErrorBoundary` wrapping routes com SentryFallback reutilizando design do error-boundary.tsx existente (SF29); `componentDidCatch` consolidado (Steps 5+11/FSQ14/FG3) com `Sentry.captureException(error, { contexts: { react: { componentStack } } })`; section ErrorBoundaries em paginas criticas (tabela de grupos, lista de jobs) (FR1); `<Suspense>` mantido dentro do ErrorBoundary.
- **Teste minimo:** Test que renderiza componente que lanca erro e verifica que Sentry.captureException eh chamado com componentStack.
- **Cobre:** Steps 4-5, 7, 11, 16 (originais); SF29, FSQ14, FG3, FR1
- **Depende de:** T4.1 (sentry.ts deve existir)

#### T4.3 — App-level QueryClient + ChunkLoadError

- **Entrada -> Saida:** QueryClient sem error handling global + lazy imports sem retry -> onError global capturando via Sentry + ChunkLoadError com retry automatico
- **Arquivos:** criar: [], modificar: [`client/src/app.tsx`]
- **DoD:** `defaultOptions.queries.onError` e `defaultOptions.mutations.onError` callbacks chamando `Sentry.captureException` (FR4); `React.lazy()` com catch que captura Sentry antes de `window.location.reload()` para ChunkLoadError (FR7); ChunkLoadError removido de ignoreErrors pois agora tem retry explicito.
- **Teste minimo:** Test que verifica onError do QueryClient chama captureException; test que verifica ChunkLoadError trigger reload apos captura.
- **Cobre:** Steps 8-9 (originais); FR4, FR7
- **Depende de:** T4.1 (sentry.ts deve existir)

#### T4.4 — Auth Context Sentry Integration

- **Entrada -> Saida:** Auth context sem user tracking no Sentry + catch blocks vazios -> setUser em login/logout/session restore + erros capturados em signOut/listSessions/checkSession
- **Arquivos:** criar: [], modificar: [`client/src/contexts/auth-context.tsx`]
- **DoD:** `Sentry.setUser({ id, email })` + `Sentry.setTag('user_role', role)` no login success e session restore; `Sentry.setUser(null)` no logout; signOut catch com `captureException` level warning tag `auth_logout` (FSQ15); listSessions catch com `captureException` para erros de servidor (FSQ16); checkSession erros nao-Unauthenticated capturados como Warning (FSQ17).
- **Teste minimo:** Test que verifica setUser chamado com dados corretos apos login; test que verifica signOut error eh capturado em vez de silenciado.
- **Cobre:** Steps 6, 12 (originais); FSQ15, FSQ16, FSQ17
- **Depende de:** T4.1 (setSentryUser helper)

#### T4.5 — PII Scrubbing Config

- **Entrada -> Saida:** (Cross-cutting, implementado em T4.1) -> Validacao e documentacao de que PII scrubbing cobre todos os vetores
- **Arquivos:** criar: [], modificar: [] (parte de `client/src/lib/sentry.ts` criado em T4.1)
- **DoD:** `beforeSend` strips query params com token/key/secret; redacts form data de breadcrumbs; strips PII de error messages (emails, phones); `beforeBreadcrumb` scrubs SSE token URLs; nenhum token/credential visivel em eventos Sentry capturados por T4.2-T4.4.
- **Teste minimo:** Test end-to-end que dispara erro com URL contendo token e verifica que evento Sentry nao contem o token.
- **Cobre:** Step 13 (cross-cutting com T4.1); FSQ18
- **Depende de:** T4.1 (beforeSend/beforeBreadcrumb config)

**Commit Checkpoint:** `feat(sentry): add sentry core to react frontend`

---

### Phase 5 — Frontend React: ConnectRPC + SSE + Hooks + Erros Silenciosos

**Objetivo:** Capturar erros de rede, SSE, e operacoes de hooks com contexto rico. Corrigir TODOS os erros silenciosos do frontend.

**Depende de:** Phase 4 (sentry.ts, captureHookError, safeLocalStorage, safeClipboard)
**Desbloqueia:** Phase 6 (validacao end-to-end)

**Tasks:**

#### T5.1 — ConnectRPC Sentry Interceptor Frontend

- **Entrada -> Saida:** Transport ConnectRPC sem observabilidade Sentry -> interceptor classifica erros por gRPC code e envia ao Sentry com tags
- **Arquivos:** criar: [`client/src/lib/sentry-connect-interceptor.ts`], modificar: [`client/src/lib/connect-transport.ts`]
- **DoD:** `sentryInterceptor` registrado como ULTIMO na chain (apos errorNormalization); ordem final: csrf -> tracing -> retry -> errorNormalization -> sentry; classifica: Ignorar (InvalidArgument, Unauthenticated, NotFound, AbortError), Warning (ResourceExhausted, PermissionDenied), Error (Internal, Unknown, Unavailable, DataLoss, DeadlineExceeded, Network errors); tags: `rpc.method`, `rpc.service`, `rpc.code`, `trace_id` (extraido do header x-trace-id)
- **Teste minimo:** Unit test: erro InvalidArgument NAO gera evento Sentry; erro Internal gera evento com tags corretas; AbortError ignorado
- **Cobre:** Steps 1-2
- **Depende de:** T4.1 (sentry.ts core)

#### T5.2 — SSE Resilience + Sentry Coverage

- **Entrada -> Saida:** SSE com 9 console.warn sem observabilidade + sem backoff + sem heartbeat -> SSE resiliente com Sentry breadcrumbs, backoff exponencial, e protecao completa
- **Arquivos:** modificar: [`client/src/hooks/use-session-status-sse.ts`]
- **DoD:** Exponential backoff 3s->6s->12s->24s->30s cap (WF1); max reconnect notification ao user + `Sentry.captureMessage` (WF2); `Sentry.addBreadcrumb` no onerror + capture apos 3 falhas consecutivas (WF3); JSON parse error capturado em event.data (WF4); 401 detectado para acionar refresh token antes de reconnect (WF5); heartbeat timeout para detectar conexao zumbi (WF6); 9 `console.warn('[SSE]...')` substituidos por `Sentry.addBreadcrumb({ category: 'sse', message, level: 'warning' })` + manter console para dev (FG4); CADA addEventListener callback body wrappado em try/catch completo cobrindo `onStatusChangeRef.current?.()` (FG5); `window.dispatchEvent(new CustomEvent(...))` wrappado em try/catch (FR2); event listeners rastreados com refs + removeEventListener antes de re-adicionar em connect() (FR3); validacao runtime para `as SSEEvent` / `as SessionStatusData` verificando campos obrigatorios (SF23)
- **Teste minimo:** Integration test: SSE disconnect gera breadcrumbs; 6+ reconnects mantem listeners estaveis; JSON parse invalido gera Sentry capture; callback que lanca erro e capturado sem matar SSE
- **Efeito colateral / Mitigacao:** Backoff pode atrasar reconexao em cenarios de flap rapido; mitigacao: cap de 30s e conservador + reset do backoff apos sucesso
- **Cobre:** Steps 3, 10-11, 29, 42-43 (SF5, SF23, FR2, FR3, WF1-WF6, FG4, FG5)
- **Depende de:** T4.1 (sentry.ts core)

#### T5.3 — useWhatsApp: All Fixes

- **Entrada -> Saida:** useWhatsApp com localStorage direto, promises nao observadas, console-only warnings, e leaks -> hook seguro com safeLocalStorage, Sentry em todos error paths, e cleanup correto
- **Arquivos:** modificar: [`client/src/hooks/use-whatsapp.ts`]
- **DoD:** `localStorage.getItem/setItem` substituido por `safeLocalStorage` (SF14-SF15); `handleImportGroups(true)` wrappado em try/catch no useEffect (SF16); async setInterval callback wrappado em try/catch (SF17); 5 `console.warn('[WhatsApp]...')` substituidos por `captureHookError` ou `Sentry.addBreadcrumb` (SF6-SF9, FG7); `onStatusChange` corpo wrappado em try/catch com `captureHookError` (FG6); non-404 errors capturados em handleConnect, refreshQrCode delete, disconnectInstance (WF7-WF8, WF11); max retry counter 5 para QR refresh loops + `Sentry.captureWarning` (WF9); refreshQrCode falha -> `Sentry.captureException` + toast (WF10); retry antes de limpar state em initial status check (WF13); import polling interval armazenado em ref + cleanup no useEffect return (WF21); `window.__importFallbackTimeout` substituido por useRef (WF22); setInterval ref armazenado + cleanup em unmount (FSQ12); `customEvent.detail?.data` null check defensivo + breadcrumb (FSQ9); `queryClient.cancelQueries` antes de chamadas manuais de refetch (FR6)
- **Teste minimo:** Unit test: safeLocalStorage nao crasha em private browsing; QR loop 6+ vezes atinge max retry; unmount durante import limpa interval; network error em initial status faz retry
- **Rollback:** safeLocalStorage fallback in-memory garante que app funciona mesmo se localStorage indisponivel
- **Cobre:** Steps 4, 21, 23, 30, 44-45, 47 (SF6-SF9, SF14-SF17, WF7-WF13, WF21-WF22, FSQ9, FSQ12, FR6, FG6, FG7)
- **Depende de:** T4.2 (captureHookError, safeLocalStorage)

#### T5.4 — useAutomations: Catch Block Fixes

- **Entrada -> Saida:** useAutomations com ~10 catch blocks vazios ou sem Sentry -> todos catch blocks com captureHookError + toast
- **Arquivos:** modificar: [`client/src/hooks/use-automations.ts`]
- **DoD:** toggleStatus catch -> `captureHookError(error, 'useAutomations', 'toggleStatus')` + toast (SF3/FG8); handleDelete catch -> `captureHookError(error, 'useAutomations', 'delete')` + toast (SF4); todos ~10 catch blocks restantes com `captureHookError` mantendo toast existente; padrao: `catch (error: unknown) { captureHookError(error, 'useAutomations', 'operationName'); toast(...) }`
- **Teste minimo:** Unit test: toggleStatus error gera Sentry event com hook name e operation; catch vazio retorna 0 resultados em grep
- **Cobre:** Steps 5, 46 (SF3-SF4, FG8)
- **Depende de:** T4.2 (captureHookError)

#### T5.5 — useGroupsMessaging: Error Handling + AbortController

- **Entrada -> Saida:** useGroupsMessaging com catch silenciosos e sem abort -> error tracking por grupo + AbortController para bulk send
- **Arquivos:** modificar: [`client/src/hooks/groups/use-groups-messaging.ts`]
- **DoD:** Participant fetch tracking com error count; se ALL fail -> `Sentry.captureMessage` (SF11/WF14); FileReader catch captura `error` object + `return` apos file read error toast para evitar envio sem imagem (SF12/WF16); loop error capturado com `captureHookError` + `{ extra: { groupId, groupName } }` (SF13/WF15); AbortController implementado para bulk send + cleanup no unmount (WF17)
- **Teste minimo:** Unit test: bulk send cancelado no unmount via AbortController; erro individual inclui groupId no Sentry event; file read error impede envio
- **Cobre:** Steps 6, 31 (SF11-SF13, WF14-WF17)
- **Depende de:** T4.2 (captureHookError)

#### T5.6 — useRedirectLink + safeClipboard

- **Entrada -> Saida:** Mutations sem onError detalhado + clipboard nao protegida em 3+ locais -> mutation errors capturados + safeClipboard helper com fallback
- **Arquivos:** criar: [safeClipboard helper em `client/src/lib/sentry.ts` ou `client/src/lib/utils.ts`], modificar: [`client/src/hooks/use-redirect-link.ts`, `client/src/hooks/use-coupons.ts`, `client/src/components/admin/dashboard/webhook-section.tsx`]
- **DoD:** Mutation onError em useRedirectLink acessa `error` param + `captureHookError` (SF18-SF20/FG9); safeClipboard helper criado: tenta `navigator.clipboard.writeText(text)`, fallback `document.execCommand('copy')`, se ambos falham `Sentry.captureMessage('clipboard unavailable')` + toast "Copiar manualmente"; `navigator.clipboard.writeText` substituido por safeClipboard em use-redirect-link (SF22/WF34), use-coupons (FSQ2), webhook-section (FSQ3); clipboard async/await com catch rejection
- **Teste minimo:** Unit test: safeClipboard em contexto HTTP usa fallback; falha de ambos metodos gera Sentry message; mutation error inclui hook name
- **Cobre:** Steps 7, 18, 37, 47, 49 (SF18-SF22, WF34, FSQ2-FSQ3, FG9, FG12)
- **Depende de:** T4.2 (captureHookError)

#### T5.7 — Remaining Hooks: Sentry Coverage

- **Entrada -> Saida:** 10+ hooks com catch blocks silenciosos ou sem Sentry -> todos com captureHookError ou Sentry.addBreadcrumb
- **Arquivos:** modificar: [`client/src/hooks/use-ml-link-lists.ts`, `client/src/hooks/use-affiliate-programs.ts`, `client/src/hooks/use-history.ts`, `client/src/hooks/use-settings-connect.ts`, `client/src/hooks/use-public-redirect.ts`, `client/src/hooks/use-jobs.ts`, `client/src/hooks/use-forgot-password.ts`, `client/src/hooks/groups/use-groups.ts`, `client/src/hooks/use-dispatch-texts.ts`, `client/src/hooks/use-affiliate-credentials-connect.ts`]
- **DoD:** use-ml-link-lists: clipboard catch + URL parse -> `captureHookError` (SF1, SF25); use-affiliate-programs: JSON.parse catches -> `captureHookError` + atob() guard com Sentry capture (SF26-SF27, FSQ4); use-history: debug logs removidos + invalidateQueries error capturado (SF10, SF28); use-settings-connect: onError -> `captureHookError` (SF21); use-public-redirect: DOM catch -> `Sentry.captureMessage` level Info (SF2); use-jobs: todos catch blocks + unsafe error cast corrigido com `instanceof Error` pattern (FSQ10-FSQ11); use-forgot-password: catch block -> `captureHookError`; use-groups: todos catch blocks -> `captureHookError`; use-dispatch-texts: catch blocks com breadcrumbs (FSQ26); use-affiliate-credentials-connect: credential JSON parse -> `Sentry.captureMessage('credential_json_parse_failed', 'warning')` (FSQ29); 10 catch blocks `// ignore` restantes com `Sentry.addBreadcrumb` ou `captureHookError` (FSQ22-FSQ31)
- **Teste minimo:** Unit test: JSON parse invalido gera Sentry warning; atob() com input invalido nao crasha; catch `// ignore` blocks retornam 0 em grep
- **Cobre:** Steps 8, 19, 24-25, 27, 47 (SF1, SF2, SF10, SF21, SF25-SF28, FSQ4, FSQ22-FSQ31, FSQ29)
- **Depende de:** T4.2 (captureHookError)

#### T5.8 — Group Component Fixes

- **Entrada -> Saida:** Componentes de grupo sem validacao de upload, type assertions inseguras, e catch silenciosos -> validacoes client-side + Sentry em error paths
- **Arquivos:** modificar: [`client/src/components/groups/group-audio-dialog.tsx`, `client/src/components/groups/group-photo-dialog.tsx`, `client/src/components/groups/group-message-dialog.tsx`, `client/src/components/groups/group-members-dialog.tsx`, `client/src/components/groups/group-description-dialog.tsx`, `client/src/components/groups/group-poll-dialog.tsx`, `client/src/components/groups/schedule-multiple-groups.tsx`]
- **DoD:** Audio size validation max 16MB com error toast (WF18); photo type + size validation max 5MB com error toast (WF19/FR9); message dialog com `isSending` disabled state (WF20); members handleConfirmedAction wrappado em try/catch + `Sentry.captureException` (WF39); schedule file size validado antes de base64 encode + captura tentativas oversized (WF40); schedule type assertion `error as { message?: string }` substituido por `instanceof Error` pattern + `Sentry.captureException` (WF28); description/poll validacao defensiva (WF26, WF38)
- **Teste minimo:** Unit test: arquivo >16MB rejeitado no audio dialog; arquivo >5MB rejeitado no photo dialog; type assertion invalida nao crasha schedule
- **Cobre:** Steps 14, 32, 36, 40-41 (FR9, WF18-WF20, WF26, WF28, WF38-WF40)
- **Depende de:** T4.2 (captureHookError)

#### T5.9 — Page-level Fixes + Formatter

- **Entrada -> Saida:** Paginas sem error states + formatter com global mutable key -> error states implementados + key generation segura
- **Arquivos:** modificar: [`client/src/pages/groups.tsx`, `client/src/pages/whatsapp.tsx`, `client/src/lib/whatsapp-formatter.tsx`, `client/src/components/groups/group-connection-status.tsx`]
- **DoD:** Groups page com error state quando load de grupos falha (WF25); WAHA-especifico ErrorBoundary com acoes de recovery (Reconnect/Clear session) substituindo generico (WF37); `let globalKey = 0` mutable global substituido por key generation segura (useId ou stable index-based) - ALTO (WF27); GroupConnectionStatus com loading/skeleton quando status === 'checking' (WF24)
- **Teste minimo:** Unit test: groups page mostra error state em falha de load; formatter key e deterministica sem side effects; ErrorBoundary WAHA oferece recovery actions
- **Efeito colateral / Mitigacao:** Mudanca do globalKey pode afetar reconciliacao React existente; mitigacao: key generation estavel baseada em index garante mesma ordem
- **Cobre:** Steps 34-35, 39 (WF24-WF25, WF27, WF37)
- **Depende de:** T4.3 (ErrorBoundary)

#### T5.10 — App-wide Patterns (Double Submit, i18n, Navigation, AbortSignal)

- **Entrada -> Saida:** Forms sem protecao contra double submit, i18n sem deteccao de keys faltantes, navegacao sem guard durante mutations, ConnectRPC sem AbortSignal -> protecoes globais implementadas
- **Arquivos:** modificar: [multiplos page/form files, config i18n, multiplos hooks com ConnectRPC direto]
- **DoD:** `isPending` do useMutation desabilita botoes submit em todos forms (FR10); `missingKeyHandler` na config i18n chama `Sentry.captureMessage` level Warning em producao (FR11); `useBlocker` do React Router adicionado quando mutations estiverem pendentes (FR12); AbortController implementado em todas chamadas ConnectRPC diretas conforme CLAUDE.md rules (WF35)
- **Teste minimo:** Unit test: double-click submit gera mutation unica; chave i18n inexistente gera warning Sentry; navegacao durante mutation pendente e bloqueada
- **Cobre:** Steps 15-17, 38 (FR10-FR12, WF35)
- **Depende de:** T5.1 (interceptor), T4.1 (sentry.ts)

#### T5.11 — Browser API Guards

- **Entrada -> Saida:** APIs de browser (CSV export, error cast, API service, crypto) sem protecao -> try/catch + fallbacks + Sentry em todos browser APIs
- **Arquivos:** modificar: [`client/src/hooks/use-automation-logs.ts`, `client/src/hooks/use-jobs.ts`, `client/src/services/api.service.ts`, `client/src/lib/connect-transport.ts`, `client/src/components/admin/hooks/use-plans-tab.ts`]
- **DoD:** `exportCSV()` inteira wrappada em try/catch + `format(new Date(log.created_at))` com validacao de Date (FSQ7, FSQ34); `(error as Error).message` substituido por `error instanceof Error ? error.message : String(error)` em use-jobs (FSQ10-FSQ11); legacy API service resposta `any` com validacao de tipo runtime (FSQ13); `mutateAsync` em use-plans-tab wrappado em try/catch ou substituido por `.mutate()` (FSQ35); `crypto.getRandomValues(array)` wrappado em try/catch com fallback `Math.random()` + `Sentry.captureMessage('crypto.getRandomValues unavailable', 'warning')` (FG13); varredura completa `localStorage.` -> ZERO uso direto, todos substituidos por `safeLocalStorage` (FG11 CRITICO)
- **Teste minimo:** Unit test: exportCSV com date invalida nao crasha; crypto fallback funciona; localStorage grep retorna 0 resultados (excluindo safeLocalStorage/node_modules/gen)
- **Cobre:** Steps 20, 22, 26-27, 48, 50 (FSQ7, FSQ10-FSQ11, FSQ13, FSQ34, FSQ35, FG11, FG13)
- **Depende de:** T4.2 (safeLocalStorage)

#### T5.12 — WAHA Frontend Zero-to-Full Coverage

- **Entrada -> Saida:** 33 arquivos WAHA frontend com ZERO Sentry coverage -> cobertura completa de error handlers
- **Arquivos:** modificar: [todos 33 arquivos WAHA frontend identificados na auditoria - cross-cutting]
- **DoD:** `Sentry.captureException` implementado em TODOS os error handlers dos 33 arquivos WAHA frontend; este task e o resultado agregado de T5.2-T5.9 aplicado especificamente a fluxos WAHA; verificacao: `grep -rn 'catch' --include='*.ts' --include='*.tsx' src/hooks/use-whatsapp.ts src/hooks/groups/ src/components/groups/` retorna ZERO catch blocks sem Sentry
- **Teste minimo:** Grep audit: zero catch blocks sem Sentry/captureHookError/breadcrumb em arquivos WAHA
- **Cobre:** Step 28 (WF36 CRITICO)
- **Depende de:** T5.2, T5.3, T5.5, T5.8, T5.9

**Commit Checkpoint:** `feat(sentry): add sentry to connectrpc, sse, hooks and fix all silent errors`

---

### Phase 6 — Validacao end-to-end + Source Maps + Alertas

**Objetivo:** Source maps funcionando, erros aparecendo no dashboard com stack traces legiveis, alertas configurados. Verificar que ZERO erros silenciosos restam.

**Depende de:** Phase 1-5 (todas as fases anteriores)
**Desbloqueia:** Nenhuma (fase final)

**Tasks:**

#### T6.1 — Source Maps + Build Config

- **Entrada -> Saida:** Build sem source maps no Sentry + console.error/warn redundantes -> source maps hidden com upload automatico + cleanup de console redundante
- **Arquivos:** modificar: [`client/vite.config.ts`, `.gitignore`]
- **DoD:** `build.sourcemap: 'hidden'` configurado (gera source maps mas nao referencia no bundle); `@sentry/vite-plugin` adicionado com `org/project/authToken` via env vars + `filesToDeleteAfterUpload: ['./dist/**/*.map']`; `.sentryclirc` adicionado ao `.gitignore`; chaves i18n para SentryFallback adicionadas se necessario; `console.error/console.warn` redundantes com Sentry removidos (manter apenas os guardados com `import.meta.env.DEV`); deteccao de ChunkLoadError no ErrorBoundary com prompt para reload (FR8)
- **Teste minimo:** Build de producao gera source maps hidden; stack traces no Sentry mostram codigo original; `.map` files deletados apos upload; ChunkLoadError exibe prompt de reload
- **Cobre:** Steps 1-2, 8-10 (FR8)
- **Depende de:** T4.3 (ErrorBoundary), T5.1 (interceptor)

#### T6.2 — Backend Audit (grep varredura)

- **Entrada -> Saida:** Codebase backend potencialmente com erros silenciosos restantes -> auditoria completa confirmando ZERO gaps
- **Arquivos:** Nenhum (audit only)
- **DoD:** Todas as seguintes varreduras retornam 0 violacoes (ou justificativa documentada): `_ =` sem observabilidade (excluindo _test.go); `go func` sem SafeGo/recover (excluindo _test.go); `if err != nil` sem log OU return; `math/rand` em services (substituido por crypto/rand); `md5.` para hashing; `time.Parse` com `_ :=`; `json.Marshal/Unmarshal` com `_ =`; type assertions `.(int64)/.(string)` sem `, ok`; `logger.Fatal` restantes (substituidos por logger.Error + sentry.Flush + os.Exit) (BG4); `log.Printf/log.Println/fmt.Printf` em adapters (W36); `phone/Phone` em logs nao mascarados (W42-W43); `http.Client{}` sem Transport; `io.ReadAll` sem MaxBytesReader; queries DB sem context+timeout; `sentry.Init()` ocorre ANTES de qualquer operacao fallible (BG1-BG2); `sentrygin.New()` cobre TODAS rotas Gin (BG5); Recovery interceptor chama `RecoverWithContext` DENTRO do recover() (BG7); `LogAndCapture` rate limiting local funciona em Redis down (BG11); PII redaction parcial verificada em eventos Sentry reais (BG15); TODOS pontos W1-W44 com captura Sentry no codigo WAHA
- **Teste minimo:** Cada grep command retorna 0 resultados (ou excepcoes justificadas e documentadas)
- **Cobre:** Steps 3, 11, 13, 16, 19
- **Depende de:** T1.1-T3.12 (todas tasks backend)

#### T6.3 — Frontend Audit (grep varredura)

- **Entrada -> Saida:** Codebase frontend potencialmente com erros silenciosos restantes -> auditoria completa confirmando ZERO gaps
- **Arquivos:** Nenhum (audit only)
- **DoD:** Todas as seguintes varreduras retornam 0 violacoes: `catch {` sem Sentry/captureHookError/breadcrumb; `console.warn.*SSE` sem breadcrumb (todos 9 locais com Sentry); `localStorage.` direto (excluindo safeLocalStorage/node_modules/gen) -> 0; `navigator.clipboard` direto (excluindo safeClipboard) -> 0; `atob/btoa` sem try/catch -> 0; `as Error` cast unsafe em catch blocks -> 0; `mutateAsync` sem try/catch -> 0; `catch {}` empty -> 0; `initSentry()` executa ANTES de `getElementById('root')` (FG1); `unhandledrejection` handler existe (FG2); `componentDidCatch` em error-boundary.tsx com Sentry (FG3); CADA addEventListener SSE com try/catch completo (FG5); `crypto.getRandomValues` com try/catch em connect-transport.ts (FG13); TODOS pontos WF1-WF40 com captura nos arquivos WAHA frontend; ZERO catch blocks sem Sentry em use-whatsapp.ts/groups hooks/group components
- **Teste minimo:** Cada grep command retorna 0 resultados
- **Cobre:** Steps 4, 13, 19
- **Depende de:** T4.1-T5.12 (todas tasks frontend)

#### T6.4 — Validation Tests (Backend + Frontend)

- **Entrada -> Saida:** Integracao Sentry nao validada end-to-end -> suite completa de testes confirmando captura, filtragem, PII redaction, e resiliencia
- **Arquivos:** Nenhum (testing only)
- **DoD:** **Backend tests:** Sentry init + "Sentry initialized" log message; panic intencional -> evento Sentry; 5xx -> captura com request context; ConnectRPC Internal -> captura com procedure/traceId; InvalidArgument NAO no Sentry; PII (email, tokens) redacted; Redis down -> Warning; rate limiter failure -> Critical; pool exhaustion (25 queries) -> evento Sentry; Redis disconnect -> reconnect Pub/Sub + breadcrumb; POST 50MB -> resposta 413 (nao OOM); Redis Eval tipo inesperado -> rate limiter nao crasha (SQ50); goroutine panic (SaveCredentials SQ42, email auth SQ45-SQ47) -> Sentry capture; LockedUntil formato invalido -> conta bloqueada (SQ57); ExpiresAt formato invalido -> token rejeitado (SQ59); json.Marshal fail em automation_repository -> Sentry capture (SQ62); admin reset -> email + audit trail (SQ20-SQ21); WAHA timeout -> "timeout-assumed-sent" (W17); HMAC disabled -> Critical (W40); broadcast cheio -> Critical (W29); `logger.Fatal` -> zero restantes; backend sem SENTRY_DSN -> funciona com warning "Sentry disabled" (BG3); falha ANTES do logger -> evento Sentry chega (BG1-BG2); panic ConnectRPC -> Recovery interceptor captura (BG7); Redis down 60s -> ~1 evento agregado (BG11); phone number -> redacao parcial `+55 11 ****-****` (BG15). **Frontend tests:** throw em componente -> ErrorBoundary + Sentry; ConnectRPC error -> stack trace; login -> user context; source maps -> codigo original; AbortError NAO no Sentry; private browsing -> funciona (FG11); SSE disconnect -> breadcrumbs + max reconnect; clipboard HTTP -> fallback (FG12); `#root` ausente -> Sentry capture (FG1); unhandled rejection sem DSN -> console.error (FG2); render error -> componentDidCatch + componentStack (FG3); SSE 9 breadcrumbs (FG4); double-click -> mutation unica; i18n missing key -> warning; SSE 6+ reconnects -> listeners estaveis; QR loop 6+ -> max retry (WF9); network error initial status -> retry (WF13); navigate during bulk send -> abort (WF17); unmount during import -> cleanup (WF21); phone numbers NAO em events; ChunkLoadError -> retry + prompt; version mismatch -> reload prompt; ErrorBoundary reporta via componentDidCatch (FSQ14); checkSession non-auth errors (FSQ17); SSE URLs sem tokens em breadcrumbs (FSQ18); exportCSV dates invalidas nao crasha (FSQ34); DB failure apos WAHA logout -> state mismatch capturado (W8)
- **Teste minimo:** Todos os cenarios acima executados e passando; screenshot/log do Sentry dashboard confirmando eventos
- **Cobre:** Steps 5-6, 12, 14, 17, 20
- **Depende de:** T6.1 (source maps), T6.2 (backend audit), T6.3 (frontend audit)

#### T6.5 — Sentry Dashboard Alerts

- **Entrada -> Saida:** Sentry dashboard sem alertas configurados -> alertas P0-P3 configurados por prioridade com thresholds
- **Arquivos:** Nenhum (Sentry dashboard config only)
- **DoD:** **P0 (Critical - Notificacao imediata):** DLQ entries (tag:dlq=true); queue enqueue failures; panic recoveries; rate limiter failures (S1-S3); session revocation failures (S4); rate limiter type assertion panic (SQ50); password migration goroutine crash (SQ44); HMAC disabled em producao (W40); broadcast channel saturation (W29); `logger.Fatal` before Sentry flush (BG4); ErrorBoundary triggered but no Sentry event (FG3). **P1 (High - Alerta em 5min):** External API failures (WAHA, ML, Shopee, AI); HMAC failures (security events); database connection issues; password update failures (S77); history updates >10/min (S32-S43); session revocation em ChangePassword (SQ33); magic link rate limit bypass (SQ35); LockedUntil/ExpiresAt time.Parse failure (SQ57, SQ59); admin email not sent (SQ21); jobs stuck running (SQ30); password reset rate limit bypass (SQ68); WAHA API key sem HTTPS (W39); timeout-assumed-sent >5/min (W17); instance DB sync failure (W8, W23); SSE disconnect >5min (FG4-FG6); event flood >100/min (BG11). **P2 (Medium - Digest diario):** Rate limit exceeded; SSE max reconnects; email failures; cache ops (S44-S56); frontend hook errors; admin audit trail losses (SQ20, SQ24-SQ25); WAHA JSON unmarshal (SQ61); automation JSON marshal (SQ62); SSE client drops >50/hour (W28); QR refresh max retries (WF9); dispatch log silencioso (W34-W35); localStorage failures private mode (FG11); clipboard unavailable >5% (FG12). **P3 (Low - Weekly digest):** Warnings, degraded mode; config warnings (S21-S22); clipboard/localStorage failures; phone PII em logs (W42); redirect invite code (W37); over-redaction (BG15)
- **Teste minimo:** Cada alerta P0 dispara notificacao imediata em teste; P1 dispara em 5min; P2 aparece no digest diario; screenshot de cada regra configurada
- **Cobre:** Steps 7, 15, 18, 21
- **Depende de:** T6.4 (validation tests confirmam que eventos chegam)

**Commit Checkpoint:** `feat(sentry): source maps, validation, alerts and final silent error audit`

---

## Configuração Sentry - beforeSend (PII Redaction)

### Backend Go
```go
BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
    // Redact email patterns in exception messages
    // Redact authorization/cookie headers in request context
    // Redact request body fields: password, token, secret, key, credential
    // Redact IP addresses (keep only /24 subnet for debugging)
    // Scrub WhatsApp phone numbers from messages/extras
    return event
}
```

### Frontend React
```typescript
beforeSend(event) {
    // Strip query params with token/key/secret
    // Redact user input from breadcrumbs (form data)
    // Remove localStorage/sessionStorage data from extras
    // Strip PII from error messages (email regex, phone regex)
    // Scrub CSRF tokens from request headers
    return event;
}
```

---

## Risk Assessment

| Risk | Probabilidade | Impacto | Mitigação |
|------|--------------|---------|-----------|
| Sentry SDK aumenta bundle size frontend | Baixa | Baixo | Tree-shaking + lazy load (~30KB gzipped) |
| Sentry offline/down em prod | Baixa | Nenhum | SDK tem failsafe silencioso, app continua |
| PII vazando para Sentry | Media | Alto | beforeSend com redaction agressiva + review em Phase 6 |
| Source maps expostos publicamente | Baixa | Medio | Usar `sourcemap: 'hidden'` + deletar maps após upload |
| Overhead de performance no backend | Baixa | Baixo | SampleRate configurável, async transport |
| Muitos eventos/custos Sentry | Media | Medio | Filtros de erros esperados + sampling rate adequado |
| Corrigir erros silenciosos quebra fluxo existente | Baixa | Medio | Padrão: log+capture mas NAO alterar return path (fire-and-forget mantido, só adiciona observabilidade) |
| localStorage crash em private browsing (SF14-SF15) | Alta | Alto | safeLocalStorage wrapper com fallback in-memory |
| **3rd Pass - Concurrent map panic (C3, C5)** | Media | Alto | Auditar todos map access sob locks; SafeGo com recover |
| **3rd Pass - OOM via request/response sem limite (N1, N4, N6)** | Media | Critico | MaxBytesReader em todos endpoints webhook e chamadas HTTP externas |
| **3rd Pass - QueryClient sem onError global (FR4)** | Alta | Alto | Adicionar defaultOptions onError como safety net |
| **3rd Pass - ChunkLoadError tela branca (FR7)** | Alta | Alto | Retry logic em React.lazy() + ErrorBoundary + prompt reload |
| **3rd Pass - DB pool exhaustion silenciosa (C9)** | Baixa | Alto | Monitoramento periódico de pool.Stat() via Sentry |
| **3rd Pass - Pub/Sub Redis não reconecta (N10)** | Media | Alto | Reconnect com backoff exponencial |
| **4th Pass - Rate limiter type assertion panic (SQ50)** | Media | Critico | Panic no rate_limiter crasha todo rate limiting; adicionar ok check |
| **4th Pass - Password migration goroutine sem recover (SQ44)** | Baixa | Critico | Panic silencioso impede migração bcrypt→argon2id; SafeGo wrapper |
| **4th Pass - ErrorBoundary sem componentDidCatch (FSQ14)** | Alta | Critico | Erros React capturados pelo boundary nunca reportados a nenhum sistema |
| **4th Pass - Admin handler 13 erros suprimidos (SQ18-SQ26)** | Media | Alto | Trilhas de auditoria admin perdidas; operações críticas sem log |
| **4th Pass - Jobs service 8 status updates perdidos (SQ27-SQ32)** | Alta | Alto | Jobs ficam presos em "running" sem indicação de falha |
| **4th Pass - Auth goroutines sem recover (SQ44-SQ47)** | Baixa | Alto | Emails de verificação/reset/magic link podem nunca ser enviados |
| **4th Pass - LockedUntil time.Parse zero time (SQ57)** | Baixa | Critico | Conta nunca fica bloqueada; brute-force bypass |
| **4th Pass - Token expiry time.Parse (SQ59)** | Baixa | Alto | Tokens podem expirar imediatamente ou nunca |
| **4th Pass - WAHA models 30+ unmarshal suprimidos (SQ61)** | Media | Alto | Dados de grupo parciais se API WAHA muda formato |
| **4th Pass - Automation repo 21 marshal/unmarshal suprimidos (SQ62-SQ64)** | Baixa | Alto | Automações criadas com campos array NULL; leitura retorna zero values |
| **4th Pass - MD5 para dedup keys (SQ52)** | Baixa | Medio | Risco de colisão; mensagens duplicadas processadas |
| **4th Pass - math/rand em 5 services (SQ52-SQ56)** | Baixa | Medio | Não é CSPRNG; previsibilidade em delays/jitter |
| **4th Pass - SSE token em URL query string (FSQ18)** | Alta | Alto | Token aparece em browser history, server logs, breadcrumbs Sentry |
| **4th Pass - checkSession engole non-auth errors (FSQ17)** | Media | Alto | Backend down durante init = zero dados diagnósticos |
| **4th Pass - setInterval leak em handleImportGroups (FSQ12)** | Media | Alto | Timer continua após unmount; state updates em componente desmontado |
| **5th Pass - WF36 zero Sentry no frontend WAHA** | Alta | Critico | Time CEGO para falhas de WhatsApp em produção: SSE, QR, mensagens, sync |
| **5th Pass - W40 HMAC disabled sem alerta** | Alta | Critico | Webhook endpoint aceita payload de QUALQUER fonte em produção; injection de eventos fake |
| **5th Pass - W39 WAHA API key sem HTTPS** | Media | Alto | API key interceptável se WAHA em host remoto sem TLS |
| **5th Pass - W29 broadcast channel saturation** | Media | Critico | Todos os users perdem SSE events quando buffer=256 enche |
| **5th Pass - WF9 QR code loop infinito** | Alta | Alto | Delete+create a cada 30s sem limite; pode sobrecarregar WAHA server |
| **5th Pass - WF13 initial status clears instance** | Media | Alto | Erro de rede transient "desconecta" user de sessão WAHA válida |
| **5th Pass - WF21 import polling memory leak** | Media | Alto | setInterval continua após unmount; state updates em componente desmontado |
| **5th Pass - WF17 bulk send sem cancellation** | Media | Alto | User navega e async loop continua enviando mensagens em background |
| **5th Pass - W17 timeout-assumed-sent silencioso** | Media | Alto | Zero visibilidade sobre mensagens com entrega incerta |
| **5th Pass - W42 phone numbers em logs (PII)** | Alta | Alto | Violação LGPD/GDPR; phone numbers logados em Info level |
| **5th Pass - W2 parseMemberOperationResult false positive** | Baixa | Alto | Promote/demote reporta sucesso quando WAHA retorna body vazio |
| **6th Pass - BG4 logger.Fatal bypasses defers** | Media | Critico | `os.Exit(1)` de `logger.Fatal` impede `sentry.Flush` — erros fatais do server NUNCA chegam ao Sentry |
| **6th Pass - FG1 crash antes de Sentry init** | Baixa | Critico | Se `#root` ausente ou init falha antes de `Sentry.init()`, erro completamente invisível |
| **6th Pass - FG3 ErrorBoundary sem componentDidCatch** | Alta | Critico | Render errors React capturados para fallback UI mas NUNCA reportados ao Sentry |
| **6th Pass - FG11 localStorage crash private mode** | Alta | Alto | ~15% de usuários em modo privado podem ter app quebrada sem nenhum diagnóstico |
| **6th Pass - BG7 Recovery interceptor não repassa ao Sentry** | Media | Alto | Panics em handlers ConnectRPC capturados pelo Recovery mas nunca chegam ao SentryInterceptor |
| **6th Pass - BG11 Sentry event flood sem Redis** | Baixa | Alto | Redis down → cada request gera evento Sentry → milhares de eventos → custos e noise |
| **6th Pass - BG15 PII over-redaction** | Media | Medio | Redação muito agressiva torna erros ilegíveis; phone/email context perdido |
| **6th Pass - FG5 addEventListener callback exceptions** | Media | Alto | Exceção em onStatusChange mata SSE inteiro silenciosamente para todos os users |
| **6th Pass - FG4 SSE 9 console-only logs** | Alta | Alto | Falhas de SSE/reconexão completamente invisíveis no Sentry; troubleshooting cego |

## Rollback Plan

### Se Sentry causar problemas:
1. **Frontend:** Remover `initSentry()` do main.tsx → 0 impacto (SDK não inicializa)
2. **Backend:** Setar `SENTRY_DSN=""` → SDK não inicializa, app roda normal
3. **Completo:** Reverter commits de cada phase (independentes entre si)
4. **Sem impacto em dados:** Sentry é read-only na aplicação, não modifica estado
5. **NOVO:** Phase 3 (erros silenciosos backend) é revertível independentemente - apenas adiciona logging, não muda fluxo
6. **NOVO (3rd pass):** Mudanças de concorrência (SafeGo, race fixes) e resiliência (MaxBytesReader, timeouts) são defensivas - reverter volta ao comportamento original sem quebrar funcionalidade
7. **NOVO (5th pass WAHA):** Correções de PII masking (W42-W44) devem ser mantidas mesmo em rollback Sentry; QR max retry (WF9) e import cleanup (WF21) são bug fixes independentes de Sentry
8. **NOVO (6th pass):** Mudanças de bootstrap ordering (BG1-BG2), logger.Fatal→Error (BG4), safeLocalStorage (FG11), safeClipboard (FG12), e addEventListener protection (FG5) são melhorias defensivas que beneficiam a app independentemente do Sentry. LogAndCapture rate limiting (BG11) e PII redaction parcial (BG15) são Sentry-specific e revertíveis

### Ordem de rollback (se necessário):
- Phase 6 → Phase 5 → Phase 4 → Phase 3 → Phase 2 → Phase 1 (cada fase é independente e revertível)

---

## Dependências de Pacotes

### Backend (go.mod)
```
github.com/getsentry/sentry-go v0.31.1
github.com/getsentry/sentry-go/gin v0.31.1
```

### Frontend (package.json)
```json
{
  "@sentry/react": "^9.x",
  "@sentry/vite-plugin": "^3.x"
}
```

---

## Estatísticas da Auditoria

| Métrica | Valor |
|---------|-------|
| Arquivos Go analisados | 298+ |
| Erros silenciosos backend (1st-2nd pass) | 96 |
| Erros silenciosos frontend (1st-2nd pass) | 32+ |
| Total de pontos de captura (original) | B1-B53 + F1-F27 = 80 |
| Total de erros silenciosos (1st-2nd pass) | S1-S82 + SF1-SF29 = 111 |
| **3rd pass: Concorrência** | **C1-C10 = 10** |
| **3rd pass: Resiliência de rede** | **N1-N17 = 17** |
| **3rd pass: Runtime frontend** | **FR1-FR12 = 12** |
| **4th pass: Backend static analysis** | **SQ1-SQ68 = 68** |
| **4th pass: Frontend static analysis** | **FSQ1-FSQ35 = 35** |
| **5th pass: Backend WAHA** | **W1-W44 = 44** |
| **5th pass: Frontend WAHA** | **WF1-WF40 = 40** |
| **6th pass: Backend coverage final** | **BG1-BG15 = 15** |
| **6th pass: Frontend coverage final** | **FG1-FG15 = 15** |
| **Total geral de pontos Sentry** | **447** |
| Arquivos a criar | 5 |
| Arquivos a modificar | 125+ |
| Categorias de segurança crítica | 12 (rate limit, session revocation, password update, token reuse, magic link rate limit, password reset rate limit, LockedUntil bypass, token expiry bypass, HMAC disabled, WAHA API key plaintext, webhook injection, SSE token leak) |
| CRITICOs (3rd pass) | 3 (N4 request size, FR4 QueryClient onError, FR7 ChunkLoadError) |
| CRITICOs (4th pass) | 3 (SQ44 password migration goroutine, SQ50 rate limiter type assertion, FSQ14 ErrorBoundary sem componentDidCatch) |
| CRITICOs (5th pass) | 1 (WF36 zero Sentry em todo frontend WAHA) |
| CRITICOs (6th pass) | 4 (BG4 logger.Fatal bypasses Sentry flush, FG1 crash antes de Sentry init, FG3 ErrorBoundary steps duplicados, FG11 localStorage crash private mode) |
| ALTOs (4th pass) | 32 |
| ALTOs (5th pass) | 14 (W2, W10, W17, W29, W39, W40, WF2, WF5, WF9, WF13, WF15, WF17, WF21, WF27) |
| ALTOs (6th pass) | 8 (BG7 Recovery interceptor, BG13 Cron Phase dependency, FG2 unhandled rejection safety net, FG4-FG7 SSE/WhatsApp silent, FG8 automations catch) |
| MEDIOs (4th pass) | 42 |
| MEDIOs (5th pass) | 38 |
| MEDIOs (6th pass) | 14 (BG1-BG2 bootstrap, BG5-BG6 Gin routes, BG9-BG10 external clients, BG11 rate limiting, BG14 worker list, BG15 PII, FG9-FG10 catch, FG12-FG14 browser APIs) |
| BAIXOs (4th pass) | 25 |
| BAIXOs (5th pass) | 31 |
| BAIXOs (6th pass) | 4 (BG3 env validation, BG8 interceptor edge, BG12 cache semantics, FG15 dev mode) |
| **Cobertura estimada após implementação** | **~99% de todos os error paths** |

---

## Checklist de Conclusão

### Phase 1 - Core Backend
- [ ] Sentry inicializado no backend (main.go)
- [ ] sentrygin middleware registrado
- [ ] ConnectRPC SentryInterceptor capturando erros por gRPC code
- [ ] Panic recovery integrado com Sentry (Gin + ConnectRPC)
- [ ] `SafeGo` helper criado e usado na goroutine do server (S66)
- [ ] `LogAndCapture` helper criado
- [ ] defer Close() errors capturados (S15-S20)
- [ ] Port parsing validado (S31)
- [ ] DB pool monitoring via pool.Stat() (C9)
- [ ] statement_timeout configurado no pool (N8)
- [ ] Shutdown WaitGroup para goroutines (N11)
- [ ] Startup cleanup: Redis/DB close em falha (N12)
- [ ] cronService.Stop() aguardado (N13)
- [ ] Env vars obrigatórias validadas (N14)
- [ ] Rate limit middleware notado para Sentry logging (N16)
- [ ] Queue server goroutine com SafeGo (SQ49)
- [ ] Rate limiter type assertion com ok check (SQ50) - CRITICO
- [ ] Sentry Init no main.go (W10) - ALTO
- [ ] SSE bridge subscribe failure capturado (W11)
- [ ] Backfill startup failure capturado (W12)
- [ ] WAHA adapter nil check em boundaries (W25)
- [ ] WebhookSecret validation em prod (W27)
- [ ] HMAC disabled alerta Sentry Critical (W40)
- [ ] WAHA HTTPS enforcement em prod (W39)
- [ ] **6th pass:** Sentry Init ANTES do logger — config load → DSN → sentry.Init() → logger (BG1-BG2)
- [ ] **6th pass:** logger.Fatal substituído por logger.Error + sentry.Flush + os.Exit (BG4) - CRITICO
- [ ] **6th pass:** sentrygin.New() cobre rotas Gin puras /health /webhooks (BG5-BG6)
- [ ] **6th pass:** Recovery interceptor chama sentry.RecoverWithContext DENTRO do recover() (BG7)
- [ ] **6th pass:** Cron SafeGo movido para Phase 1 (BG13)

### Phase 2 - Workers + Services
- [ ] Workers Asynq wrappados com SentryAsynqMiddleware
- [ ] Webhook handler com captures de segurança (B22-B27)
- [ ] SSE bridge com SafeGo (S68)
- [ ] stubs.go com error handling + SafeGo (S24-S27, S67)
- [ ] Services externos capturando timeouts/failures
- [ ] Webhook log create/update capturados (S62-S65)
- [ ] SSE Hub com SafeGo (C1)
- [ ] Cron tasks com SafeGo (C2)
- [ ] WAHA HTTP client com Transport configurado (C6)
- [ ] Pub/Sub close race resolvido (C8)
- [ ] Watchdog com SafeGo (C10)
- [ ] MaxBytesReader no WAHA adapter (N1)
- [ ] Per-operation timeout no WAHA (N2)
- [ ] Retry transiente no WAHA (N3)
- [ ] Request size limit em webhooks (N4)
- [ ] AI timeout pattern logado (N5)
- [ ] ML response limit (N6)
- [ ] Pub/Sub reconnect com backoff (N10)
- [ ] AI service json.Marshal + io.ReadAll capturados (SQ1-SQ2)
- [ ] ML service completo: marshal, readall, creds, cache, SafeGo (SQ3-SQ10, SQ42-SQ43)
- [ ] WhatsApp handler: instance status, QR, backpressure (SQ11-SQ14)
- [ ] Automations handler: error metadata (SQ15)
- [ ] Groups handler: delete após LeaveGroup (SQ16)
- [ ] WAHA service: session stop (SQ17)
- [ ] Admin handler: 13 erros suprimidos (SQ18-SQ26, SQ40)
- [ ] Jobs service: 8 history/status updates (SQ27-SQ32)
- [ ] Message worker: dispatch log update (SQ38)
- [ ] SMTP adapter: SafeGo (SQ48)
- [ ] LoadBalancer handler: server ID parse (SQ51)
- [ ] Message worker timeout-assumed-sent capturado (W17) - ALTO
- [ ] Message worker dispatch log capturado (W18)
- [ ] SSE broadcast channel full → Critical alert (W29) - ALTO
- [ ] SSE client channel full → Warning capturado (W28)
- [ ] Webhook consumer isRetryableError refatorado (W15)
- [ ] Groups sync ctx.Done() check no loop (W22)
- [ ] Redirect invite code failure capturado (W37)
- [ ] Dispatch log silencioso capturado em monitoring + automation (W34-W35)
- [ ] parseMemberOperationResult false positive capturado (W2) - ALTO
- [ ] **6th pass:** Shopee service TODOS error paths com Sentry (BG9)
- [ ] **6th pass:** URL safety service fail-open com Sentry Warning (BG10)
- [ ] **6th pass:** Lista explícita 7 workers verificada com SentryAsynqMiddleware (BG14)

### Phase 3 - Erros Silenciosos Backend
- [ ] Auth rate limiter errors capturados (S1-S3) - CRITICO
- [ ] Session revocation error capturado (S4) - CRITICO
- [ ] Auth session cache errors capturados (S10-S14)
- [ ] History update errors capturados (S32-S43, 12 pontos)
- [ ] Cache operation errors capturados (S44-S56, 13 pontos)
- [ ] JSON/parsing errors tratados (S57-S61)
- [ ] Repository errors tratados (S77-S82)
- [ ] Backpressure errors capturados (S74-S76)
- [ ] Context cancellation respeitado (S69-S73)
- [ ] Config warnings logados (S21-S23)
- [ ] Type assertions validadas (S28-S31)
- [ ] beforeSend com redaction de PII (backend)
- [ ] Watchdog map race corrigido (C3)
- [ ] HMAC init com sync.Once (C4)
- [ ] Adapter cache race verificado (C5)
- [ ] Ticker leak corrigido (C7)
- [ ] Transaction commit auditado (N7)
- [ ] GetPendingEvents com limit guard (N9)
- [ ] Cache size monitorado (N15)
- [ ] Deadlock detection no repository layer (N17)
- [ ] Auth ChangePassword RevokeAllExcept capturado (SQ33) - CRITICO segurança
- [ ] Magic link rate limit Increment capturado (SQ35) - CRITICO segurança
- [ ] Session revocation workaround capturado (SQ37)
- [ ] Email verification token reuse capturado (SQ34)
- [ ] Magic link token reuse capturado (SQ36)
- [ ] Auth goroutines com SafeGo: password migration (SQ44), verification email (SQ45), reset email (SQ46), magic link email (SQ47)
- [ ] LockedUntil time.Parse validado (SQ57) - CRITICO: bypass de bloqueio de conta
- [ ] Token ExpiresAt time.Parse validado (SQ59) - ALTO: expiração indefinida
- [ ] Session timestamps time.Parse (SQ58, SQ60)
- [ ] math/rand substituído por crypto/rand em 5 services (SQ52-SQ56)
- [ ] MD5 substituído por SHA-256 para dedup (SQ52)
- [ ] WAHA models JSON unmarshal capturado em campos críticos (SQ61)
- [ ] Automation repository 7 marshal + 14 unmarshal capturados (SQ62-SQ64)
- [ ] System event repository payload marshal (SQ65)
- [ ] Webhook log repository payload marshal (SQ66)
- [ ] Action history repository 5 unmarshal (SQ67)
- [ ] History/Admin handler structpb.NewStruct (SQ39-SQ40)
- [ ] URL safety cache.SetJSON (SQ41)
- [ ] Password service rate limit Increment capturado (SQ68) - ALTO
- [ ] WAHA models 30+ json.Unmarshal capturados (W1)
- [ ] Instance service logout DB sync capturado (W8)
- [ ] Instance service cleanup failure capturado (W9)
- [ ] Message service history update capturado (W20)
- [ ] Groups service DB sync 3 locais capturados (W23)
- [ ] Phone numbers mascarados em logs (W42) - PII
- [ ] Mapper stdlib log.Printf → zap (W36)
- [ ] Webhook body size string match → errors.As (W13)
- [ ] Redis bridge context.Background() → WithTimeout (W30)
- [ ] **6th pass:** LogAndCapture rate limiting implementado (>10 same errors/60s → 1 evento) (BG11)
- [ ] **6th pass:** Cache failures definidos como fail-open + tag cache_policy (BG12)
- [ ] **6th pass:** PII redaction parcial: phone +55 11 ****-****, email b***@gmail.com (BG15)

### Phase 4 - Core Frontend
- [ ] Sentry inicializado no frontend (main.tsx)
- [ ] ErrorBoundary com Sentry no root da app (SF29)
- [ ] Auth context setando user no Sentry
- [ ] safeLocalStorage helper criado
- [ ] captureHookError helper criado
- [ ] beforeSend com redaction de PII (frontend)
- [ ] Section ErrorBoundaries em páginas críticas (FR1)
- [ ] QueryClient com onError global (FR4)
- [ ] ChunkLoadError retry logic em React.lazy() (FR7)
- [ ] Root element null check com Sentry capture (FSQ1)
- [ ] ErrorBoundary componentDidCatch implementado com Sentry (FSQ14) - CRITICO
- [ ] Auth context: checkSession non-auth errors (FSQ17), listSessions (FSQ16), signOut (FSQ15)
- [ ] SSE token scrubbing via beforeBreadcrumb (FSQ18)
- [ ] **6th pass:** initSentry() ANTES de getElementById('root') (FG1) - CRITICO
- [ ] **6th pass:** window.addEventListener('unhandledrejection') safety net ANTES de Sentry.init (FG2)
- [ ] **6th pass:** componentDidCatch steps consolidados (Step 5 + Step 11 = 1 único fix) (FG3)
- [ ] **6th pass:** Dev mode Sentry console logger (VITE_SENTRY_DEV_MODE) (FG15)

### Phase 5 - Hooks + Erros Silenciosos Frontend
- [ ] ConnectRPC SentryInterceptor filtrando erros esperados
- [ ] SSE com breadcrumbs e captures (SF5, SF23)
- [ ] useWhatsApp: localStorage fix (SF14-SF15)
- [ ] useWhatsApp: promises observadas (SF16-SF17)
- [ ] useWhatsApp: console-only → Sentry (SF6-SF9)
- [ ] useAutomations: catch vazios corrigidos (SF3-SF4)
- [ ] useGroupsMessaging: catch silenciosos corrigidos (SF11-SF13)
- [ ] useRedirectLink: mutation errors + clipboard (SF18-SF22)
- [ ] Demais hooks com captureHookError em todos os catch blocks
- [ ] Debug logs removidos (SF10)
- [ ] dispatchEvent wrappado em try/catch (FR2)
- [ ] Event listeners com cleanup correto (FR3)
- [ ] useMutation sem onError corrigidos (FR5)
- [ ] Refetch race com cancelQueries (FR6)
- [ ] File upload com validação de tamanho (FR9)
- [ ] Double submit prevenido com isPending (FR10)
- [ ] i18n missingKeyHandler configurado (FR11)
- [ ] Navigation guard com useBlocker (FR12)
- [ ] Clipboard protegida: use-coupons (FSQ2), webhook-section (FSQ3)
- [ ] atob() guard em affiliate-programs (FSQ4)
- [ ] exportCSV try/catch + Date validation (FSQ7, FSQ34)
- [ ] SSE event detail null check (FSQ9)
- [ ] Unsafe error cast corrigido em use-jobs (FSQ10-FSQ11)
- [ ] setInterval leak corrigido em handleImportGroups (FSQ12)
- [ ] 10 catch blocks restantes com breadcrumbs (FSQ22-FSQ31)
- [ ] Credential JSON parse capture (FSQ29)
- [ ] Legacy API service type validation (FSQ13)
- [ ] mutateAsync wrappado em try/catch (FSQ35)
- [ ] **WAHA Frontend - Zero Sentry → Cobertura completa (WF36)** - CRITICO
- [ ] SSE exponential backoff implementado (WF1)
- [ ] SSE max reconnect notification + Sentry (WF2)
- [ ] SSE breadcrumbs + consecutive failure capture (WF3)
- [ ] SSE JSON parse capture (WF4)
- [ ] SSE token refresh awareness (WF5)
- [ ] SSE heartbeat/keepalive detection (WF6)
- [ ] handleConnect/refreshQrCode non-404 captures (WF7-WF8)
- [ ] QR refresh max retry counter implementado (WF9) - ALTO
- [ ] refreshQrCode failure → Sentry + toast (WF10)
- [ ] disconnectInstance non-404 captures (WF11)
- [ ] Initial status check retry antes de clear (WF13) - ALTO
- [ ] loadMentionParticipants failure tracking (WF14)
- [ ] Individual send errors capturados com groupId (WF15) - ALTO
- [ ] Image read error → return (WF16)
- [ ] Bulk send AbortController implementado (WF17) - ALTO
- [ ] Audio upload size validation (WF18)
- [ ] Photo upload type/size validation (WF19)
- [ ] Message dialog isSending disabled (WF20)
- [ ] Import polling interval cleanup no unmount (WF21) - ALTO
- [ ] window global → ref (WF22)
- [ ] GroupConnectionStatus loading state (WF24)
- [ ] Groups page error state (WF25)
- [ ] WhatsApp formatter global key fix (WF27) - ALTO
- [ ] Schedule groups type assertion fix + Sentry (WF28)
- [ ] Clipboard async/catch em redirect link (WF34)
- [ ] AbortSignal em ConnectRPC diretas (WF35)
- [ ] WAHA ErrorBoundary com recovery actions (WF37)
- [ ] GroupMembers handleConfirmedAction try/catch (WF39)
- [ ] Schedule file size validation (WF40)
- [ ] **6th pass:** SSE 9 console-only → Sentry breadcrumbs em use-session-status-sse.ts (FG4)
- [ ] **6th pass:** addEventListener callbacks protegidos com try/catch completo (FG5)
- [ ] **6th pass:** useWhatsApp onStatusChange defensivo com try/catch (FG6)
- [ ] **6th pass:** useWhatsApp 5× console.warn → captureHookError (FG7)
- [ ] **6th pass:** useAutomations catch vazios → captureHookError + toast (FG8)
- [ ] **6th pass:** Mutation onError com detalhes em todos hooks (FG9)
- [ ] **6th pass:** Varredura completa localStorage → ZERO uso direto (FG11) - CRITICO
- [ ] **6th pass:** safeClipboard helper criado e usado em todos locais (FG12)
- [ ] **6th pass:** crypto.getRandomValues com try/catch no transport (FG13)
- [ ] **6th pass:** Catch blocks vazios varredura → 0 resultados (FG10)

### Phase 6 - Validação
- [ ] Source maps habilitados (hidden) + upload via Sentry Vite plugin
- [ ] Audit final: zero `_ = err` sem justificativa no backend
- [ ] Audit final: zero catch blocks sem Sentry no frontend
- [ ] Audit final: zero localStorage sem safeLocalStorage
- [ ] Validação: erros aparecem com stack traces legíveis
- [ ] Validação: PII não vaza
- [ ] Validação: erros esperados filtrados
- [ ] Validação: private browsing funciona (SF14-SF15)
- [ ] Alertas P0-P3 configurados por prioridade
- [ ] Version mismatch detection com prompt reload (FR8)
- [ ] Audit: todas goroutines com SafeGo ou recover
- [ ] Audit: todos http.Client com Transport configurado
- [ ] Audit: todos io.ReadAll com MaxBytesReader
- [ ] Audit: todas queries DB com context + timeout
- [ ] Teste: DB pool exhaustion → evento Sentry
- [ ] Teste: Redis disconnect → Pub/Sub reconnect + breadcrumb
- [ ] Teste: POST 50MB em webhook → resposta 413
- [ ] Teste: pool.Stat() reporting no Sentry
- [ ] Teste: ChunkLoadError tratado graciosamente
- [ ] Teste: React Query falha sem onError local → handler global
- [ ] Teste: Deploy nova versão → prompt reload em tab antiga
- [ ] Teste: Double-click submit → mutation única
- [ ] Teste: Chave i18n inexistente → warning Sentry
- [ ] Teste: SSE reconnect 6+ vezes → listeners estáveis
- [ ] Audit: zero `_ = err` sem observabilidade no backend (SonarQube)
- [ ] Audit: zero math/rand em services (substituído por crypto/rand)
- [ ] Audit: zero MD5 para hashing/dedup
- [ ] Audit: zero time.Parse com erro suprimido
- [ ] Audit: zero json.Marshal/Unmarshal com erro suprimido
- [ ] Audit: zero type assertions sem ok check
- [ ] Audit: zero catch blocks sem Sentry no frontend
- [ ] Audit: zero navigator.clipboard sem try/catch
- [ ] Audit: zero atob/btoa sem try/catch
- [ ] Audit: zero `as Error` cast unsafe em catch blocks
- [ ] Audit: zero mutateAsync sem try/catch
- [ ] Teste: Rate limiter Redis retorna tipo inesperado → sem panic (SQ50)
- [ ] Teste: SaveCredentials goroutine panic → Sentry capture (SQ42)
- [ ] Teste: Email goroutine panic → Sentry capture (SQ45-SQ47)
- [ ] Teste: LockedUntil formato inválido → conta bloqueada corretamente (SQ57)
- [ ] Teste: Token ExpiresAt formato inválido → token rejeitado (SQ59)
- [ ] Teste: Automation JSON marshal falha → Sentry capture (SQ62)
- [ ] Teste: Admin password reset → email enviado E audit trail registrado (SQ20-SQ21)
- [ ] Teste: Jobs completados → status correto no DB (SQ27-SQ32)
- [ ] Teste: ErrorBoundary → componentDidCatch reporta ao Sentry (FSQ14)
- [ ] Teste: checkSession com backend down → Sentry warning (FSQ17)
- [ ] Teste: SSE URLs em breadcrumbs sem tokens (FSQ18)
- [ ] Teste: exportCSV com dates inválidas → sem crash (FSQ34)
- [ ] Alertas P0 adicionais: SQ50, SQ44
- [ ] Alertas P1 adicionais: SQ33, SQ35, SQ57, SQ59, SQ21, SQ30, SQ68
- [ ] **5th Pass WAHA - Backend:**
- [ ] Audit: todos pontos W1-W44 com captura Sentry implementada
- [ ] Audit: zero phone numbers em logs não mascarados (W42-W43)
- [ ] Audit: zero stdlib log calls em adapters WAHA (W36)
- [ ] Teste: WAHA timeout em envio → Sentry "timeout-assumed-sent" (W17)
- [ ] Teste: HMAC disabled em prod → alerta Critical (W40)
- [ ] Teste: Broadcast channel cheio → alerta Critical (W29)
- [ ] Teste: DB failure após WAHA logout → Sentry state mismatch (W8)
- [ ] Alertas P0 adicionais: W40 (HMAC), W29 (broadcast)
- [ ] Alertas P1 adicionais: W39, W17, W8, W23
- [ ] **5th Pass WAHA - Frontend:**
- [ ] Audit: todos pontos WF1-WF40 com captura Sentry implementada
- [ ] Audit: zero catch blocks WAHA sem Sentry
- [ ] Teste: SSE max reconnects → user notification + Sentry (WF2)
- [ ] Teste: QR refresh loop 6+ → max retry atingido (WF9)
- [ ] Teste: Network error em initial status → retry antes de clear (WF13)
- [ ] Teste: Navegar durante bulk send → AbortController cancela (WF17)
- [ ] Teste: Unmount durante import → setInterval cleanup (WF21)
- [ ] Teste: Phone numbers não aparecem em Sentry events (PII redaction)
- [ ] **6th Pass - Cobertura Final Backend:**
- [ ] Audit: zero `logger.Fatal` no codebase (BG4)
- [ ] Audit: sentry.Init() antes de qualquer operação que pode falhar (BG1-BG2)
- [ ] Audit: sentrygin.New() cobre TODAS rotas Gin (BG5)
- [ ] Audit: Recovery interceptor chama sentry.RecoverWithContext dentro do recover() (BG7)
- [ ] Audit: LogAndCapture rate limiting funciona (BG11)
- [ ] Audit: PII redaction parcial (phone/email) em eventos reais (BG15)
- [ ] Teste: Backend sem SENTRY_DSN → app funciona com warning (BG3)
- [ ] Teste: Falha pré-logger → evento Sentry chega (BG1-BG2)
- [ ] Teste: Panic em handler ConnectRPC → Recovery + Sentry (BG7)
- [ ] Teste: Redis down 60s → ~1 evento aggregado, não milhares (BG11)
- [ ] Teste: Erro com phone → redação parcial +55 11 ****-**** (BG15)
- [ ] Alertas P0 adicionais: BG4 (logger.Fatal), FG3 (ErrorBoundary)
- [ ] Alertas P1 adicionais: BG11 (event flood), FG4-FG6 (SSE disconnect)
- [ ] **6th Pass - Cobertura Final Frontend:**
- [ ] Audit: initSentry() ANTES de getElementById('root') (FG1)
- [ ] Audit: unhandledrejection safety net handler existe (FG2)
- [ ] Audit: componentDidCatch com Sentry.captureException (FG3)
- [ ] Audit: 9 SSE console.warn têm Sentry breadcrumb (FG4)
- [ ] Audit: addEventListener callbacks com try/catch completo (FG5)
- [ ] Audit: zero localStorage direto (FG11)
- [ ] Audit: zero navigator.clipboard direto (FG12)
- [ ] Audit: crypto.getRandomValues com try/catch (FG13)
- [ ] Audit: zero catch blocks vazios (FG10)
- [ ] Teste: #root ausente → Sentry capture "Root element not found" (FG1)
- [ ] Teste: Unhandled rejection sem DSN → console.error safety net (FG2)
- [ ] Teste: Render error → ErrorBoundary + Sentry com componentStack (FG3)
- [ ] Teste: SSE disconnect → 9 breadcrumbs no Sentry (FG4)
- [ ] Teste: Private browsing → app funciona sem crash (FG11)
- [ ] Teste: Clipboard HTTP context → fallback funciona (FG12)
- [ ] Alertas P2 adicionais: FG11 (localStorage), FG12 (clipboard)
- [ ] Alertas P3 adicionais: BG15 (over-redaction)
