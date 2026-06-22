# Análise de Capacidade, Sharding do WAHA e Risco de Banimento

> Snapshot coletado ao vivo da VPS de produção em **2026-06-22 ~20:40 UTC**
> (VPS Hetzner + Postgres + WAHA). Números são um retrato do momento — re-medir antes de decisões grandes.

---

## 1. Carga real hoje

| Métrica | Valor |
|---|---|
| Usuários cadastrados (`profiles`) | 382 |
| Usuários com automação ativa | 123 |
| Automações ativas | 287 (search 165, monitoring 83, list 22, offer_generate 17) |
| Instâncias/chips cadastrados | 187 (109 conectados) |
| **Sessões WhatsApp ativas no WAHA** | **110 — todas WORKING** (104 números distintos) |
| Mensagens **enviadas** /24h | 80.486 (≈ 56/min médio) |
| Pico de envio | ≈ 424 msg/min (~7 msg/s) |
| Eventos de dispatch /24h | 422.392 — **81% `skipped` (dedup)**, 19% envio real |
| Filas asynq | praticamente vazias → sistema acompanha a carga |
| Falhas /24h | 69 `failed` (0,08%) |

**Leitura:** sem acúmulo de fila, 0,08% de falha. O sistema está confortável no volume atual.

---

## 2. Infra (a "caixa") e folga

Servidor dedicado Hetzner — **i5-13500 (14 cores / 20 threads), 62 GB RAM, 436 GB disco, RAID md2**.

| Recurso | Capacidade | Uso atual | Folga |
|---|---|---|---|
| CPU | 20 threads | load avg 1.1 / 2.6 / 3.8 → ~6–19% | ~85% ocioso |
| RAM | 62 GB | 5,3 GB | enorme |
| Disco | 436 GB | 219 GB (53%) | 195 GB |
| Redis | maxmem 4 GB | 70 MB | ok (pós-fix OOM) |
| Postgres | — | 1,3 GB | trivial |

**Consumo por container — só um pesa:**

| Container | CPU | RAM |
|---|---|---|
| **WAHA** | **65% de 1 core** | **4,8 GB** ← único relevante |
| worker | 9% | 50 MB |
| server | 3% | 86 MB |
| postgres / redis / frontend | <2% | <1,4 GB |

**Conclusão:** CPU/RAM/disco **não são o gargalo**. A caixa aguentaria 5–8× o volume atual.

---

## 3. Onde quebra primeiro (o teto real não é a caixa)

1. **WAHA single-container** 🔴 *gargalo #1* — um container Node atende as 110 sessões; já em 65% de 1 core e 4,8 GB. Não escala horizontalmente no setup atual. ~44 MB RAM/sessão, mas o event-loop/CPU satura antes da RAM.
2. **Scheduler serial** 🟠 *gargalo #2 (latência)* — as 165 automações `search` rodam num loop serial; uma chamada lenta (Shopee/ML fora) trava todas. Mitigado por `SCHEDULER_CONCURRENCY` (#211) e pacer `SESSION_PACE_MS=1200`; raiz (contexto por automação) ainda não refeita.
3. **Anti-ban do WhatsApp por chip** 🟡 *limite de produto* — hoje ~0,5 msg/chip/min médio, ~4/min no pico: muita margem. Escalar = mais chips → recai no WAHA.

### Teto estimado por camada

| Camada | Limita em | Teto seguro |
|---|---|---|
| CPU/RAM/disco da caixa | recursos | ~5–8× hoje (não é a restrição) |
| **WAHA single container** | chips/sessões | **~350–500 chips (~3–4× hoje)** ← estoura primeiro |
| Scheduler serial | latência sob falha externa | degrada antes do teto de chips |
| Postgres/Redis | — | muito além |

**Em usuários:** ~110 chips ≈ 123 usuários ativos (≈1 chip/usuário). Teto ~350–500 chips ≈ **~350–450 usuários ativos** sem re-arquitetar. Acima disso exige sharding do WAHA + scheduler concorrente, e **só com load test real** — os números são extrapolação, não medição sob estresse.

---

## 4. Sharding do WAHA — viabilidade

**Não é flag-flip, mas não é do zero.** Existe um scaffold de load balancer ~40% pronto, porém **desconectado do caminho de envio**.

### Já existe ✅
- Tabela `user_servers` + `user_server_repository`
- `loadbalancer.Service` (`GetServerForUser`, `AssignUserToServer`, `MigrateUser`, `GetLeastLoadedServer`)
- Endpoints admin RPC + stats na observabilidade
- Config de servidores via settings (`waha_servers` JSON)
- Webhook já global (`WHATSAPP_HOOK_URL: http://server:3000/...`) → qualquer nº de shards posta de volta sem mudança 👍

### Falta ⛔
1. **Send path ignora o LB.** `cmd/server/main.go:241-265` constrói **um** `wahaAdapter` com **uma URL fixa** no startup. `GetServerForUser` só é chamado pelos endpoints admin/observability — nunca pelo adapter / `monitoring_service` / `message_processor`. Hoje todo envio vai pro WAHA único, **ignorando `user_servers`**. Para ligar: trocar o adapter singleton por um **pool keyed por URL de shard** e resolver `userID → shard` em **todo call-site** (criar sessão, enviar texto/imagem, status, logout).
2. 🔴 **Sessões não são portáveis.** WAHA GOWS guarda sessões em volume **local** (`/app/.sessions`, sqlite/arquivos) — sem store compartilhado. `MigrateUser` só troca um ponteiro no banco; **não move o estado de auth do WhatsApp**. Migrar chip já pareado = **re-parear (QR)** ou copiar arquivos na unha. Os 110 chips atuais ficam **presos ao WAHA atual**. Sharding limpo só para chips **novos**.
3. **Métrica de balanceamento errada.** O LB conta **usuários/servidor** (`maxUsersPerServer=100`); a carga real é **chips + volume**. Precisa balancear por sessões.
4. **Scaffold meio-abandonado.** `HealthCheck` tem `// TODO: Actually ping the server`, tipos de model inconsistentes, `maxUsersPerServer` hardcoded.

### Esforço

| Aspecto | Avaliação |
|---|---|
| Subir waha-2/waha-3 na mesma box | 🟢 Trivial (RAM sobra) |
| Webhooks multi-shard | 🟢 Já resolvido |
| Wirear LB no envio (pool + call-sites) | 🟠 Médio (refactor real) |
| Portar sessões existentes | 🔴 Doloroso (re-pareamento) |
| Faxina do scaffold + métrica por sessão | 🟠 Médio |

**Veredito:** projeto de tamanho médio. Fundação economiza ~40%; o grosso é ligar no hot path + sessões pinadas.

### Atalho recomendado (menor dor)
1. **Shard vertical na mesma box** — sobe `waha-2` com volume próprio.
2. **Pin:** clientes velhos no `waha-1`, novos no `waha-2` via `AssignUserToServer` na criação. Zero re-pareamento.
3. Ligar pool de adapters resolvendo por `user_servers`.
4. Trocar métrica para sessões/servidor.
5. Só depois separar shards em máquinas distintas.

Dobra o teto (~350→~700 chips) sem tocar nas sessões existentes e sem big-bang.

---

## 5. Risco de banimento — IPs residenciais / proxies

### Achado crítico 🔴

**104 números brasileiros de WhatsApp conectam-se todos por 1 único IP de datacenter:**

- Egress: `35.59.180.157` / `2a01:4f9:3100:4646::2` — **Hetzner (AS24940), Helsinki/Finlândia**
- `WHATSAPP_PROXY`: **não configurado** — nenhum proxy, saída direta
- Engine GOWS, 110 sessões, **mesmo IP datacenter pra todas**

Isso acumula **três sinais de anti-spam** que o WhatsApp pondera:

1. **ASN de datacenter** (Hetzner) — não é IP residencial nem móvel. WhatsApp trata faixas de cloud/hosting como suspeitas; tráfego "normal" de usuário vem de operadora móvel/banda larga residencial.
2. **Mismatch geográfico** — números `+55` (Brasil) saindo de IP na **Finlândia**. Conta brasileira "logada" permanentemente da Europa é incoerente com uso humano.
3. **Densidade de números por IP** — 104 contas num só IP. Um IP residencial hospeda 1–3 WhatsApps; 100+ é assinatura clara de operação em massa.

> ⚠️ Nenhum desses três é, sozinho, garantia de ban. Mas combinados elevam o *score de risco* — e o gatilho real costuma ser **comportamento** (volume/velocidade/denúncias). O IP é **amplificador**, não causa raiz.

### O que pesa MAIS que o IP

O WhatsApp bane majoritariamente por **comportamento do número**, não por IP:
- Velocidade/volume de envio por chip (hoje ~4 msg/min pico — **conservador, bom**)
- Proporção de denúncias ("Reportar"/"Bloquear") dos destinatários
- Mensagens idênticas em massa, links repetidos
- Chip novo "frio" disparando alto volume sem aquecimento

Ou seja: **mesmo com IP residencial perfeito, um chip que spamma é banido.** E o sistema já tem mitigações comportamentais (dedup, pacer por chip, anti-ban por chip no fan-out). Isso é o que mais protege.

### Decisão: precisamos de IPs residenciais?

**Não imediatamente — mas a topologia atual é uma dívida de risco que escala mal.** Recomendação por fase:

| Fase | Chips | Recomendação de rede |
|---|---|---|
| **Agora** (≤110) | atual | Manter, mas **mitigar o mismatch geográfico**: mover egress do WhatsApp para **IP brasileiro** (VPS BR ou proxy BR) é o ganho #1 com menor custo. Foco principal segue em comportamento (pacing/aquecimento). |
| **Crescimento** (110–350) | ~3× | Introduzir **proxies por shard** — cada shard WAHA sai por um IP distinto (idealmente BR). Reduz densidade de números/IP e isola o risco: se um IP "queima", só os chips daquele shard são afetados, não todos. |
| **Escala** (350+) | grande | **Proxies residenciais/móveis BR por grupo de chips** (1 IP para poucos números), preferencialmente **IP fixo por chip** (sticky session) para não "teleportar" a conta entre IPs — trocar de IP a cada conexão também é sinal ruim. |

### Pontos de implementação (quando for a hora)
- **GOWS suporta proxy por sessão** — dá pra atribuir IP diferente por chip/shard sem trocar engine. Configurável via `WHATSAPP_PROXY` global ou por sessão na API.
- **Sticky é essencial**: o mesmo chip deve sair sempre pelo mesmo IP. Proxy rotativo (IP muda a cada request) é **pior** que datacenter fixo — parece sequestro de conta.
- **Geografia importa**: IP **brasileiro** > IP residencial genérico estrangeiro. Um número BR num IP residencial alemão ainda tem mismatch.
- **Aquecimento de chip** (ramp-up de volume nos primeiros dias) protege mais que qualquer proxy. Vale como feature de produto.

### Nota: bans das plataformas de afiliado (Shopee/ML)
Risco **diferente** e menor: scraping/links de afiliado. Já mitigado com APIs de afiliado oficiais (meli.la, generateShortLink Shopee). O egress datacenter pode levar a rate-limit/captcha em scraping, mas não a "ban de conta" como no WhatsApp. Não exige IP residencial — exige respeitar rate limits e usar as APIs oficiais (caminho já adotado).

---

## 6. Veredito executivo

1. **Aguenta hoje com folga** — caixa ~85% ociosa, filas zeradas, 0,08% de falha.
2. **Cresce até ~3–4× (≈350–500 chips / ≈400 usuários ativos)** sem tocar arquitetura — o **WAHA single-container é o teto**, não o servidor.
3. **Passar disso** exige sharding do WAHA (fundação ~40% pronta, mas desligada) + scheduler concorrente + **load test real**.
4. **Banimento:** o maior risco não é capacidade — é **104 números BR saindo de 1 IP datacenter na Finlândia**. Prioridade barata = **mover egress para IP brasileiro**; em escala = **proxies sticky por shard/chip**. Mas **comportamento (pacing + aquecimento) protege mais que IP**.
5. **Antes de qualquer crescimento, a prioridade real é resiliência, não capacidade:** backup automatizado do Postgres (hoje inexistente) e reduzir o ponto único de falha (tudo numa caixa só).
