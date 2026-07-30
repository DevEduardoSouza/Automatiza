# Automatiza — Conector Mercado Livre

Extensão de navegador que conecta a conta do Mercado Livre do cliente ao
Automatiza, para o sistema gerar os links de afiliado dele automaticamente.

## Por que ela precisa existir

O cookie de sessão do Mercado Livre é **HttpOnly**: nenhum código de página
consegue lê-lo — nem bookmarklet, nem script no console. Até aqui o cliente
tinha de abrir o DevTools, achar o cookie e colar no Automatiza. Pior: ele
expirava em silêncio. As credenciais que existiam em produção ficaram entre
**111 e 144 dias mortas**, gerando **zero** links, sem que ninguém percebesse.

A extensão é o único caminho técnico para ler esse cookie automaticamente.

## Como instalar (enquanto não está na Chrome Web Store)

1. Abra `chrome://extensions` no Chrome (ou `edge://extensions` no Edge).
2. Ligue o **Modo do desenvolvedor** (canto superior direito).
3. Clique em **Carregar sem compactação** e escolha esta pasta (`extension-ml/`).
4. Faça login em [mercadolivre.com.br](https://www.mercadolivre.com.br).
5. Clique no ícone da extensão → **Copiar código de conexão**.
6. Cole em **Programas de Afiliados → Mercado Livre → Token de conexao** e salve.

O backend valida a sessão na hora de salvar: se o cookie já estiver expirado,
o cadastro é recusado com a mensagem certa, em vez de aceitar e falhar depois.

## O que ela faz — e o que deliberadamente não faz

**Faz:**

- Lê os cookies de `www.mercadolivre.com.br` e monta o código de conexão.
- Descobre a etiqueta de afiliado sozinha (`/affiliate-program/api/v2/stripe/user/tags`),
  eliminando o campo manual. Com mais de uma etiqueta, ela **não escolhe** —
  quem escolhe é o cliente, no painel.
- Responde ao painel do Automatiza via `externally_connectable`.

**Não faz:**

- Não tem servidor próprio e não envia nada para lugar nenhum. O único destino
  de rede é o próprio Mercado Livre.
- Não usa content script: não lê nem altera o conteúdo de página alguma.
- Não navega, não clica e não abre link de afiliado. Ela gera **links**, nunca
  cliques — o que a mantém fora da hipótese vedada pela cláusula 1.9 dos Termos
  do Programa de Afiliados.
- Não exibe nem registra o valor do cookie em log algum.

## Permissões e por que cada uma

| Permissão                            | Motivo                                                    |
| ------------------------------------ | --------------------------------------------------------- |
| `cookies`                            | ler o cookie de sessão (HttpOnly, inacessível de outro jeito) |
| `host_permissions` só do mercadolivre.com.br | limita tecnicamente o alcance da extensão a um único site |

Não há `tabs`, `scripting`, `webRequest` nem `<all_urls>` — de propósito.

## Arquitetura (quem chama quem)

É o **painel** que chama a extensão, nunca o contrário:

```
Painel Automatiza  --sendMessage-->  Extensão  --cookies-->  Mercado Livre
       ^                                 |
       +--------- token MLCRED_V1 -------+
```

O painel então segue o fluxo normal dele (mesma sessão, mesmo CSRF, mesmo RPC).
Consequência: **zero** configuração de CORS, **zero** exceção de CSRF e **zero**
mudança no backend.

## Publicar na Chrome Web Store

Ao submeter, a justificativa de permissão precisa ser explícita: a extensão usa
`cookies` **apenas** no domínio do Mercado Livre, para que o próprio usuário
conecte a própria conta de afiliado a um serviço que ele contratou. Vale anexar
este README — a política de "single purpose" é atendida porque a extensão faz
uma coisa só.
