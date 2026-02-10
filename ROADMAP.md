# ROADMAP — Debug VPS + Diagnóstico Completo

> Gerado em: 2026-02-04 ~03:40 (BRT)
> VPS: `76.13.173.13` | Hostname: `srv1307111`
> Domínio: `astecwms.com.br`

---

## 1. Informações da VPS

### Acesso
- **IP:** `76.13.173.13`
- **User:** root
- **OS:** Debian/Ubuntu (apt-based)
- **Hostname:** srv1307111
- **Domínio:** astecwms.com.br (HTTPS via Let's Encrypt + Nginx)

### Setup de Deploy
O projeto roda como **3 serviços systemd** (sem PM2, sem Docker para a aplicação):

| Serviço | Binário | Porta |
|---------|---------|-------|
| `automatiza-server.service` | `/opt/automatiza/backend/server` | 3000 |
| `automatiza-worker.service` | `/opt/automatiza/backend/server --worker` | — |
| `automatiza-frontend.service` | `npx serve -s dist -l 8000` | 8000 |

**Infraestrutura em Docker:**

| Container | Imagem | Status | Porta |
|-----------|--------|--------|-------|
| `automatiza-postgres` | postgres:16 | Up 7h (healthy) | 127.0.0.1:5432 |
| `automatiza-redis` | redis:7-alpine | Up 7h (healthy) | 127.0.0.1:6379 |
| `automatiza-waha` | devlikeapro/waha-plus:gows | Up 4h (**unhealthy**) | 127.0.0.1:3001 |

**Nginx** faz reverse proxy:
- `/` → Frontend (porta 8000)
- `/api/` → Backend (porta 3000)
- `/connect/` → Backend ConnectRPC (porta 3000)

### Estrutura de Arquivos
```
/opt/automatiza/
├── .env                # Config geral
├── env.conf            # EnvironmentFile para systemd
├── backend/
│   ├── server          # Binário Go compilado (55MB, atualizado 05:32)
│   ├── server.bak      # Backup anterior (39MB)
│   ├── .env            # Config do backend
│   └── migrations/
├── frontend/
│   └── dist/           # Build estático do React
├── docker/
│   └── docker-compose.yml
├── nginx/
├── scripts/
└── uploads/
```

---

## 2. Estado da Infraestrutura

### PostgreSQL — OK
```
/var/run/postgresql:5432 - accepting connections
```
- Healthy, sem problemas.
- **Erro recorrente:** `column "session_name" does not exist` na tabela `reconciliation_checkpoints` — a migração dessa tabela está com colunas inconsistentes com o código.

### Redis — OK
```
PONG
```
- Healthy, background saves funcionando normalmente a cada 5 minutos.

### WAHA — UNHEALTHY (mas funcional)
- Health check retorna 401 (requer API key), por isso o Docker marca como "unhealthy".
- O container funciona normalmente — sessões ativas, mensagens sendo enviadas.
- **WAHA API Key:** `34558b70cde04acba978037d02bddf11` (header `X-Api-Key`)
- **Versão:** 2026.2.1, Engine: GOWS, Tier: PLUS

### WAHA Webhooks — PROBLEMA
Os logs WAHA mostram erros recorrentes de webhook:
```
WARN: Error sending POST request: 'connect ETIMEDOUT 172.17.0.1:3000'. Retrying 14/15...
```
O webhook está configurado como `http://host.docker.internal:3000/api/v1/webhooks/waha`, mas as sessões individuais têm webhook `https://astecwms.com.br/api/v1/webhooks/waha`. O Docker `host.docker.internal` resolve para `172.17.0.1` que dá timeout — provavelmente o backend não está ouvindo nessa interface.

---

## 3. Estado do WhatsApp/WAHA

### Sessões Ativas (2)

| Sessão | Usuário | Telefone | Status | Presence |
|--------|---------|----------|--------|----------|
| `session_01kgkhmeg6t7cjkx3h7kyy2meb` | Eduardo Desenvolvedor | 557488359787 | WORKING | online |
| `session_01kgkkpkxp4ghbdqspgcqzargx` | Bruno Vilefort | 553197252407 | WORKING | offline |

Ambas as sessões têm webhook configurado para `https://astecwms.com.br/api/v1/webhooks/waha`.

---

## 4. Estado do Banco de Dados

### Automações (3 ativas)

| ID | Nome | Tipo | Status | Intervalo | Último Run | Próximo Run |
|----|------|------|--------|-----------|------------|-------------|
| `7ffb0544...` | Teste 01 | search | active | 10min | 06:30 | 06:40 |
| `399c3d5f...` | opaa | search | active | 10min | 06:29 | 06:39 |
| `a458be12...` | teste | search | active | 10min | 06:24 | 06:34 |

### Action History (últimas 10)
Todas com status **completed**, `success_count: 10`, `error_count: 0`.
Automações rodando normalmente a cada ~5-10 min.

### Dispatch Logs (últimos 15)
Todos com status **sent**, sem erros. URLs Shopee reais sendo enviadas.
- Automação "Teste 01" → grupo `120363423386100604@g.us`
- Automação "teste" → grupo `120363405850275497@g.us`

### Automation Runs (últimos 10)
Todos **completed**. `offers_found: 0`, `messages_sent: 0`, `messages_failed: 0`.
> Nota: `messages_sent: 0` na tabela automation_runs, mas `success_count: 10` no action_history — sugere que o contador de messages na run não está sendo atualizado corretamente.

### Instâncias WhatsApp (2)

| ID | User | Instance ID | Status |
|----|------|-------------|--------|
| 2 | `8a2cef5d...` | `session_01kgkhmeg6t7cjkx3h7kyy2meb` | connected |
| 3 | `2faf0ab8...` | `session_01kgkkpkxp4ghbdqspgcqzargx` | connected |

### Credenciais de Afiliado (Shopee)
Ambos os usuários têm credenciais Shopee ativas com `auto_generate: true`, mas **sem affiliate_id nem tracking_id** configurados.

### Grupos (19 no total)
Apenas 1 grupo favorito: **tech 02** (`120363405850275497@g.us`, instance 2).
Os dispatches vão para os grupos configurados na automação, não necessariamente favoritos.

---

## 5. Estado das Filas (Redis/Asynq)

### Filas
- `default` — fila principal
- `webhooks` — fila de webhooks

### Contadores

| Métrica | Valor |
|---------|-------|
| Processed (total) | 519 |
| Failed (total) | **200** |
| Archived (dead letter) | **50** |
| Retry | 0 |
| Pending | 0 |

### Análise dos Jobs Archived (Dead Letter)
Os 50 jobs archived são todos `whatsapp:send_message` com o mesmo erro:
```
WAHA error 500: Cannot use 'in' operator to search for 'url' in undefined
```

**Causa raiz:** O backend Go envia o payload com `"mediaUrl": "https://..."` mas o WAHA GoWS engine espera `"file": {"url": "https://..."}`.

**Stack trace do WAHA:**
```
TypeError: Cannot use 'in' operator to search for 'url' in undefined
    at WhatsappSessionGoWSPlus.fileToMedia (session.gows.plus.js:46:19)
    at WhatsappSessionGoWSPlus.sendMedia (session.gows.plus.js:122:34)
    at WhatsappSessionGoWSPlus.sendImage (session.gows.plus.js:188:27)
```

Os jobs archived são de uma sessão antiga (`session_01kgk9904te68bhz4kx50dxsm0`) que não existe mais.

---

## 6. Testes Manuais WAHA (resultados reais)

### Teste 1: Envio de Texto Simples
```bash
curl -X POST -H 'X-Api-Key: ...' http://localhost:3001/api/sendText \
  -d '{"session": "session_01kgkkpkxp4ghbdqspgcqzargx",
       "chatId": "120363423386100604@g.us",
       "text": "[TESTE DEBUG] Mensagem de texto simples"}'
```
**Resultado: SUCESSO** — Mensagem entregue com ID confirmado.

### Teste 2: Envio de Imagem com `mediaUrl` (formato antigo)
```bash
curl -X POST -H 'X-Api-Key: ...' http://localhost:3001/api/sendImage \
  -d '{"session": "...", "chatId": "...",
       "caption": "...",
       "mediaUrl": "https://cf.shopee.com.br/file/sg-11134201-7rdwy-mciokd5ue9g431"}'
```
**Resultado: ERRO 500** — `Cannot use 'in' operator to search for 'url' in undefined`

### Teste 3: Envio de Imagem com `file.url` (formato correto GoWS)
```bash
curl -X POST -H 'X-Api-Key: ...' http://localhost:3001/api/sendImage \
  -d '{"session": "...", "chatId": "...",
       "caption": "...",
       "file": {"url": "https://cf.shopee.com.br/file/sg-11134201-7rdwy-mciokd5ue9g431"}}'
```
**Resultado: SUCESSO** — Imagem enviada e entregue no grupo com thumbnail.

### Conclusão dos Testes
| Método | Formato Payload | Resultado |
|--------|----------------|-----------|
| sendText | `text` | OK |
| sendImage | `mediaUrl` (campo flat) | **FALHA** (WAHA GoWS bug) |
| sendImage | `file: {url: ...}` (nested) | **OK** |

A URL Shopee é acessível normalmente da VPS (HTTP 200, 141KB JPEG).

---

## 7. Fixes Já Implementados no Código Local

### Fix 1: Logger + Error Tracking no AutomationsHandler
- Adicionado structured logging com zap no handler de automações
- Error tracking melhorado para diagnóstico

### Fix 2: Fallback Imagem → Texto no `message_worker.go`
- Quando o envio de imagem falha, tenta enviar como texto puro
- Log de warning com a imageURL e o erro original
- Localização: `automatiza-go/internal/workers/message_worker.go:131`

### Fix 3: Timeout 30s → 60s no `asynq.go`
- Aumentado timeout de processamento de jobs na fila

---

## 8. Bug Raiz Descoberto: `mediaUrl` vs `file.url`

### O Problema
O arquivo `automatiza-go/internal/services/waha/waha_service.go:323-328` envia:
```go
payload := models.WahaSendImageRequest{
    Session:  instanceName,
    ChatID:   groupID,
    Caption:  text,
    MediaURL: imageURL,  // Serializa para "mediaUrl"
}
```

Que gera o JSON:
```json
{"session": "...", "chatId": "...", "caption": "...", "mediaUrl": "https://..."}
```

Mas o **WAHA GoWS engine** (versão 2026.2.1) espera:
```json
{"session": "...", "chatId": "...", "caption": "...", "file": {"url": "https://..."}}
```

### O Fix Necessário
Alterar `waha_service.go:SendMessageWithImage` para usar `File` ao invés de `MediaURL`:

```go
payload := models.WahaSendImageRequest{
    Session: instanceName,
    ChatID:  groupID,
    Caption: text,
    File:    &models.WahaFile{URL: imageURL},
}
```

O struct `WahaFile` já existe com o campo `URL` correto (`json:"url"`).

---

## 9. Outros Problemas Encontrados

### 9.1 Erro `reconciliation_checkpoints` — coluna `session_name` não existe
O worker loga a cada minuto:
```
ERROR: column "session_name" does not exist (SQLSTATE 42703)
```
Na query:
```sql
SELECT session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
FROM reconciliation_checkpoints WHERE session_name = $1
```
**Ação:** Verificar a migração da tabela e o nome correto da coluna.

### 9.2 WAHA Health Check retorna 401
O Docker health check faz `curl -f http://localhost:3000/api/health` sem API key.
**Ação:** Adicionar header no health check ou configurar WAHA para não exigir auth no `/api/health`.

### 9.3 WAHA Webhook Timeout (Docker → Backend)
O `docker-compose.yml` configura `WHATSAPP_HOOK_URL: http://host.docker.internal:3000`, mas o backend escuta em `127.0.0.1:3000`. O `host.docker.internal` pode não resolver corretamente.
**Ação:** As sessões já têm webhook para `https://astecwms.com.br/...` que funciona via Nginx, então os webhooks por sessão compensam. Mas o fallback global deveria ser corrigido.

### 9.4 Contadores `automation_runs.messages_sent` sempre 0
Enquanto `action_history.success_count` mostra 10, o `automation_runs.messages_sent` está sempre 0.
**Ação:** Verificar se o fluxo de dispatch está atualizando o run record.

---

## 10. Próximos Passos

### Prioridade 1 — Fix Crítico (Bloqueia envio de imagens)
- [ ] **Corrigir `waha_service.go:SendMessageWithImage`** para usar `File: &WahaFile{URL: imageURL}` ao invés de `MediaURL: imageURL`
- [ ] Compilar novo binário e fazer deploy na VPS
- [ ] Testar envio de imagem Shopee via automação real

### Prioridade 2 — Fix de Migração
- [ ] Verificar/criar migração para `reconciliation_checkpoints` com a coluna correta
- [ ] Eliminar erro recorrente no worker

### Prioridade 3 — Health Check WAHA
- [ ] Adicionar `WAHA_API_KEY` no health check do docker-compose, ou usar endpoint sem auth
- [ ] Container deve reportar "healthy"

### Prioridade 4 — Contadores de Run
- [ ] Investigar por que `automation_runs.messages_sent` está sempre 0
- [ ] Garantir que o tracking está correto

### Deploy na VPS
```bash
# 1. Compilar para Linux no Mac
GOOS=linux GOARCH=amd64 go build -o server ./cmd/server/main.go

# 2. Upload
scp server root@76.13.173.13:/opt/automatiza/backend/server.new

# 3. Na VPS: swap binário
ssh root@76.13.173.13
cp /opt/automatiza/backend/server /opt/automatiza/backend/server.bak2
mv /opt/automatiza/backend/server.new /opt/automatiza/backend/server
chmod +x /opt/automatiza/backend/server
chown automatiza:automatiza /opt/automatiza/backend/server

# 4. Restart
systemctl restart automatiza-server automatiza-worker

# 5. Verificar logs
journalctl -u automatiza-server -f
journalctl -u automatiza-worker -f
```
