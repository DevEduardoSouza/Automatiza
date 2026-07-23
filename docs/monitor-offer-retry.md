# Melhoria 9 — monitor com conteúdo diverso e ofertas

Estado desta entrega e contrato do ledger. Plano-fonte:
`.claude/plans/papel-atue-como-arquiteto-vectorized-crown.md`.

## O que já está no working tree

| § do plano | Entrega | Estado |
|---|---|---|
| 12.1 | `internal/lib/commerceurl` (parser + SafeClient + classificador) | ✅ com testes |
| 12.1 | `text.IsShopeeLink/IsMLLink/IsOtherStoreLink` delegando ao helper | ✅ |
| 12.1 | `text.RenderMixedCampaignNotice` + `SelectParagraphsContaining` + `ReplaceLinksLongestFirst` | ✅ |
| 12.2 | ML: fetch endurecido, `RemoteError` tipado, fim do passthrough de short de terceiro | ✅ com testes |
| 12.3 | Shopee: `ExpandURL` endurecida, `ExtractProductIDs` por path, `stripAffiliateAttribution` sem fallback cru, `shouldRelayOwnShortLink` exige `FromMe` | ✅ com testes |
| 12.4 | Migração **079** (jobs/items/deliveries) + `models.MonitorOffer*` | ✅ SQL e domínio |
| 12.5 | `internal/lib/usergate` | ✅ com testes |
| 12.6 | `WahaAdapter.DownloadMediaByURL` (`safeWAHAMediaPath`) | ✅ já corrigido por outra frente |

## O que NÃO está implementado

`internal/repository/postgres/monitor_offer_retry_repository.go`, o planner
`internal/services/monitoroffer`, `internal/services/relaymedia`, a religação de
`offer_generate`/`monitoring`, o worker/cron de retry e o
`cmd/replay-monitor-offer`.

Motivo, e ele é físico: `internal/repository/postgres` **não compila** no
working tree por WIP de outra frente (`job_repository.go` — assinatura de
`Create` divergente da interface e `sort` sem import). Enquanto isso não fechar,
qualquer repositório novo escrito nesse pacote é código não verificável.

## Numeração da migração

O plano reservava a 077. Quando a implementação começou, `077_instagram_story_hardening`
já estava commitada e havia `077_instance_failover` + `078_monitor_dispatch_cadence`
no working tree de outra frente. Esta melhoria usa a **079**.

## Contrato que o ledger implementa

**Não é exactly-once físico.** A WAHA só oferece `id` pré-gerado em
`MessageTextRequest`; `MessageImageRequest` e `sendLinkCustomPreview` não têm
chave de idempotência equivalente. O contrato implementável é:

- **at-most-once automático por destino** — `sending` que perdeu o desfecho vira
  `ambiguous` e **não** é reenviado;
- **dedup lógica** — `monitor_offer_deliveries` com PK `(job, output_key, destino)`;
- **ambiguidade visível** — `exhausted`, `blocked` e `ambiguous` são a DLQ de
  domínio, consultável por SQL.

`dispatch_logs` continua sendo o histórico por destino e **não** é o ledger:
é volumosa, o status é por destino e `GetRecentByCanonicalKey` só olha
`sent/queued/pending`.

### Identidade

- Job: `(user_id, automation_id, mode, source_group_id, message_id)`.
  `message_id` vazio é payload inválido — registra `missing_message_id` e não envia.
- Item: `(job_id, ordinal)`, ordinal = posição da primeira ocorrência no corpo.
- Fusão de duplicata: unique parcial `(job_id, canonical_key) WHERE canonical_key IS NOT NULL`.
  Ao publicar a key, o menor ordinal vence; depois que a key pertence a uma saída
  preparada/enviada, ela **nunca** troca de dono.
- Delivery: `(job_id, output_key, destination_group_id)` — a autoridade de
  idempotência. Nunca chamar a WAHA apoiado só em claim Redis fail-open.

### Payload cifrado

AES-GCM v1 via `internal/lib/crypto`, chave `ML_CREDENTIALS_ENCRYPTION_KEY`
(sem secret novo). Conteúdo: `{version, body_after_story, from_me,
message_timestamp, ordered_urls}`, teto de 16 KiB antes de cifrar.

Estourar **não trunca**: as saídas já seguras vão, e o que dependeria de retry
vira `blocked/payload_too_large`. Um payload truncado reclassificaria links
diferentes dos que a mensagem tinha.

Nunca entram: sender, push name, telefone, MediaURL, base64, título/descrição de
preview, token. TTL do ciphertext: 6h. Metadata terminal: 30 dias.

## Gates

| Env | Formato | Default | Controla |
|---|---|---|---|
| `MONITOR_MIRROR_CONTENT` | `off\|all\|<csv UUID>` | `off` | espelho de texto/mídia sem commerce no modo monitoring |
| `MONITOR_PRODUCT_RETRY` | `off\|all\|<csv UUID>` | `off` | ledger, task e recovery nos dois modos |

Token fora de `off`/`all`/UUID é **erro de boot**, nunca `all`
(`usergate.MustParse`).

**As correções de segurança não são gated.** Gate OFF significa "sem espelho
comum novo e sem retry novo" — host exato, guard anti-SSRF e proibição de
repassar URL comercial crua valem sempre. O critério "gate OFF = byte-a-byte" do
plano antigo não se aplica.

## Replay manual (ainda a implementar)

```bash
go run ./cmd/replay-monitor-offer --item <uuid> --user <uuid>
```

- aceita `exhausted` transient, `blocked/permanent_config` e `ambiguous`;
- `permanent_content`, `security_block` e `unsupported_commerce` são **não
  replayáveis** (reprocessar não muda o desfecho, e no caso de `security_block`
  reabriria a superfície fechada);
- `ambiguous` exige `--confirm-ambiguous` — há risco real de duplicata;
- exige payload não expirado, tenant exato e automação ativa;
- zera `attempts` e lease, preserva `total_attempts`, incrementa `replay_count`
  e `enqueue_generation`;
- não aceita `sent`, `succeeded` nem `superseded`;
- logs mostram item ID, user ID e reason code — **nunca** payload.

## Rollback operacional

Desligar os dois gates. As tabelas ficam inertes; **não** derrubá-las — é nelas
que fica o rastro das entregas ambíguas, que é o que se consulta depois de um
incidente.
