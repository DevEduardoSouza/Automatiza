⏺ Não basta só a VPS_SSH_KEY. Para o deploy funcionar no próximo push, você precisa ter tudo isso configurado antes:                                                                                                              
                                                                                             
  1. SSH Key (par de chaves)                                                                                                                                                                                                      
                                                                                                                                                                                                                                  
  - Gerar: ssh-keygen -t ed25519 -f deploy_key -N ""                                                                                                                                                                              
  - Chave privada → GitHub Secret VPS_SSH_KEY                                                                                                                                                                                     
  - Chave pública → na VPS em ~/.ssh/authorized_keys

  2. Todos os GitHub Secrets
  ┌───────────────────────────────┬────────────┐
  │            Secret             │   Status   │
  ├───────────────────────────────┼────────────┤
  │ VPS_HOST                      │ configurar │
  ├───────────────────────────────┼────────────┤
  │ VPS_USER                      │ configurar │
  ├───────────────────────────────┼────────────┤
  │ VPS_SSH_KEY                   │ configurar │
  ├───────────────────────────────┼────────────┤
  │ DB_PASSWORD                   │ configurar │
  ├───────────────────────────────┼────────────┤
  │ JWT_SECRET                    │ configurar │
  ├───────────────────────────────┼────────────┤
  │ TOKEN_HMAC_KEY                │ configurar │
  ├───────────────────────────────┼────────────┤
  │ ML_CREDENTIALS_ENCRYPTION_KEY │ configurar │
  ├───────────────────────────────┼────────────┤
  │ WAHA_API_KEY                  │ configurar │
  ├───────────────────────────────┼────────────┤
  │ KIWIFY_WEBHOOK_TOKEN          │ configurar │
  ├───────────────────────────────┼────────────┤
  │ SMTP_USERNAME                 │ configurar │
  ├───────────────────────────────┼────────────┤
  │ SMTP_PASSWORD                 │ configurar │
  ├───────────────────────────────┼────────────┤
  │ SMTP_FROM_EMAIL               │ configurar │
  ├───────────────────────────────┼────────────┤
  │ AI_API_KEY                    │ configurar │
  └───────────────────────────────┴────────────┘
  3. Todas as GitHub Variables
  ┌────────────────────┬────────────────────────────────────────────┐
  │      Variable      │                   Valor                    │
  ├────────────────────┼────────────────────────────────────────────┤
  │ DOMAIN             │ astecwms.com.br                            │
  ├────────────────────┼────────────────────────────────────────────┤
  │ DB_USER            │ automatiza                                 │
  ├────────────────────┼────────────────────────────────────────────┤
  │ DB_NAME            │ automatiza                                 │
  ├────────────────────┼────────────────────────────────────────────┤
  │ DB_SSLMODE         │ disable                                    │
  ├────────────────────┼────────────────────────────────────────────┤
  │ DB_MAX_CONNS       │ 25                                         │
  ├────────────────────┼────────────────────────────────────────────┤
  │ PORT               │ 3000                                       │
  ├────────────────────┼────────────────────────────────────────────┤
  │ NODE_ENV           │ production                                 │
  ├────────────────────┼────────────────────────────────────────────┤
  │ LOG_LEVEL          │ info                                       │
  ├────────────────────┼────────────────────────────────────────────┤
  │ CORS_ORIGIN        │ https://astecwms.com.br                    │
  ├────────────────────┼────────────────────────────────────────────┤
  │ PUBLIC_BASE_URL    │ https://astecwms.com.br                    │
  ├────────────────────┼────────────────────────────────────────────┤
  │ BACKEND_PUBLIC_URL │ https://astecwms.com.br                    │
  ├────────────────────┼────────────────────────────────────────────┤
  │ COOKIE_DOMAIN      │ astecwms.com.br                            │
  ├────────────────────┼────────────────────────────────────────────┤
  │ COOKIE_SECURE      │ true                                       │
  ├────────────────────┼────────────────────────────────────────────┤
  │ TZ                 │ America/Sao_Paulo                          │
  ├────────────────────┼────────────────────────────────────────────┤
  │ SMTP_HOST          │ smtp.gmail.com                             │
  ├────────────────────┼────────────────────────────────────────────┤
  │ SMTP_PORT          │ 587                                        │
  ├────────────────────┼────────────────────────────────────────────┤
  │ SMTP_FROM_NAME     │ Automatiza                                 │
  ├────────────────────┼────────────────────────────────────────────┤
  │ AI_API_URL         │ https://api.openai.com/v1/chat/completions │
  ├────────────────────┼────────────────────────────────────────────┤
  │ AI_MODEL           │ gpt-4o-mini                                │
  └────────────────────┴────────────────────────────────────────────┘
  4. Na VPS (antes do push)

  # Diretório precisa existir
  mkdir -p /opt/automatiza/docker

  # Parar serviços antigos
  systemctl stop automatiza-server automatiza-worker automatiza-frontend

  5. Postgres/Redis/WAHA já rodando

  Os containers de infra (postgres, redis, waha) precisam estar rodando na VPS. O compose usa volumes nomeados (docker_postgres_data) para preservar os dados existentes.

  ---
  Resumindo: SSH key + 13 secrets + 19 variables + diretório criado + systemd parado = próximo push faz deploy automático.