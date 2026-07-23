package repository

import "errors"

// ErrUnknownTrackingTenant é devolvido quando o nome da sessão WAHA não resolve
// para NENHUMA instância em `instances`.
//
// Isto NÃO é transitório na prática: a fonte dominante são sessões ÓRFÃS —
// vivas na WAHA, sem linha no banco (backlog conhecido do runbook). A WAHA
// continua entregando group.v2.participants por elas indefinidamente, e
// retentar produz 4 execuções por evento que nunca podem dar certo: em
// 23/07/2026 isso despejou 359 eventos/hora no DLQ e queimou slots do worker
// durante um backlog de webhook.
//
// A corrida legítima que o retry cobria (webhook chegando antes do commit da
// linha de uma instância recém-pareada) tem rede própria: o cron
// reconcile_group_memberships refaz a projeção de hora em hora.
//
// Vive em arquivo próprio, e não em interfaces.go, para não disputar o mesmo
// arquivo com as frentes que estão escrevendo lá.
var ErrUnknownTrackingTenant = errors.New("tracking tenant unavailable")
