# Revisao das Telas

## /groups

- [x] Responsividade quebrada nos botoes da secao "Acoes em Massa"
- [ ] Deve exibir apenas os grupos em que o usuario e ADMIN

## /jobs

- [ ] Erro de UI/UX: ao clicar na bolinha, a acao nao funciona; so dispara ao clicar no nome
- [ ] Erro ao criar agendamento (400 Bad Request)

**Detalhes do erro:**

```
POST /connect/jobs.v1.JobsService/CreateJob
Status: 400 Bad Request
```

Payload enviado:

```json
{
  "actionType": "JOB_ACTION_TYPE_SEND_MESSAGE",
  "scheduledFor": "2026-02-04T18:37:00Z",
  "payload": {
    "groups": ["05f41a2b-dcad-4778-b115-1dfa2491bf50"],
    "message": "Oferta imperdivel! Confira esse produto incrivel com desconto especial. Corra antes que acabe!"
  }
}
```

Response:

```json
{
  "code": "invalid_argument",
  "message": "failed to create job: ERROR: null value in column \"type\" of relation \"jobs\" violates not-null constraint (SQLSTATE 23502)"
}
```

> Causa provavel: o campo `type` nao esta sendo enviado ou mapeado corretamente no backend.

## /offer-automations (Divulgacoes de IA)

- [ ] Nao funcionou no teste (pode ser que o deploy ainda nao tenha sido feito)

## /dispatch-texts

- [ ] Adicionar template de exemplo como inspiracao no campo "Texto"

Modelo sugerido:

```
NADA MELHOR QUE COMPRAR E ECONOMIZAR

> CORRE QUE VAI ESGOTAR

{nome_produto}

De: R$ {preco_original}
Por: R$ {preco}

Compre aqui: {link_afiliado}

{aviso_preco}
```

