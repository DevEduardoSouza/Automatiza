Execute *context.check* - *context.listToFill* - *context.fillSingle* do MCP ai-context para atualizar o contexto de forma inteligente

plan **E** using ai-context MCP

start the workflow

--------------------------------------------------------------------------------------------------------------------------------------------

Credenciais de Teste:

┌──────────────────────┬──────────────┬─────────────┬───────┐
│        Email         │   Senha      │    Roles    │ Plano │
├──────────────────────┼──────────────┼─────────────┼───────┤
│ test@example.com     │ semSenha123! │ admin, user │ pro   │
├──────────────────────┼──────────────┼─────────────┼───────┤
│ readonly@example.com │ Test123456   │ user        │ free  │
└──────────────────────┴──────────────┴─────────────┴───────┘

--------------------------------------------------------------------------------------------------------------------------------------------

# CRIAR PLANO:

Crie um plano de implementação para: leia as docs do waha plus com engine gows em */Users/brunovilefort/Desktop/automatiza/waha-openapi.json* e depois veja o antigo fluxo da rota /whatsapp no frontend *client* e antigo backend *server*, então crie um plano de migração para o novo fluxo *React + Node → React + Go (gRPC/Proto) sem perda de lógica* e trocando a evolution api pela waha plus com engine gows, alem disso troque o supabase por postgres. Preciso de um plano extremamente completo e que garanta as mesmas funcionalidades anterioes de *client ( react ) e server ( node)* na nova stack com golang, postgres e waha plus com engine gows.


--------------------------------------------------------------------------------------------------------------------------------------------

# MELHORAR PLANO

Aplique o checklist que vou fornecer no plano *migration/prompts/paridade/01-auth-migration.md*:

## REFINAMENTO DE PLANOS:
- [ ] Defina a estrutura organizacional de pastas (frontend e backend) indicando onde ficam: pages, components, hooks, services/api, schemas/validators, routes/controllers/use-cases, db/migrations, shared/utils
- [ ] Em cada tarefa, liste explicitamente os arquivos a criar, a modificar e a remover (paths completos)
- [ ] Para cada tarefa, descreva rapidamente os efeitos colaterais prováveis e inclua 1 mitigação objetiva (quando aplicável)
- [ ] Identifique tarefas com código reaproveitável e crie/ajuste tarefas para extrair shared modules evitando duplicação (DRY) e separando responsabilidades (SOLID)
- [ ] Adicione em cada tarefa: Entrada → Saída (o que ela usa e o que ela entrega) em 1 linha, para evitar tarefas vagas
- [ ] Adicione em cada tarefa Critério de aceite (DoD) curto e objetivo (ex.: “página X com paridade; testes Y passando; endpoint Z respondendo com schema W”)
- [ ] Marque dependências entre tarefas (ex.: “bloqueia/depende de …”) para o plano ter ordem natural sem precisar de datas
- [ ] Quando uma tarefa mexer em comportamento antigo, inclua plano de rollback simples (ex.: feature flag, fallback, reversão de migration)
- [ ] Inclua teste mínimo por tarefa (unit ou integração ou E2E) — 1 linha, só o essencial
- [ ] Remova prazos e qualquer referência temporal para as tarefas (datas, semanas, “hoje/amanhã”)


--------------------------------------------------------------------------------------------------------------------------------------------


# CHECKLIST DE VALIDAÇÃO FINAL ( GRATUITO ):
1) - Verificação de possíveis pontos de falha em todos os arquivos relacionados a página.
2) - SonarQube com quality gate agressivo em todos os arquivos relacionados a página.
3) - Análise profunda e correção de todos os possíveis erros silenciosos em todos os arquivos relacionados a página.
4) - Listar todos possíveis testes unitários, testes de integração e testes e2e que podem ser criados para cobrir todos os arquivos relacionados a página.
5) - Análise profunda de performance e otimização de todos os arquivos relacionados a página.
6) - Análise profunda de segurança de todos os arquivos relacionados a página.
7) - Padronização de erros (catálogo de erros). Error codes/categorias por camada (Repo/Service/Handler/Client).
8) - Go API: Recovery de panic (interceptor/middleware) + interceptors de erro (normalização para RPC codes) + logging estruturado sem duplicar logs em cascata.
9) - Postgres/pgx: mapeamento de erros por SQLSTATE (unique violation, FK violation, not found) + erros tipados de domínio + mensagens seguras (sem vazar detalhes do DB).
10) - Idempotência + dedupe + invariantes no banco (unique constraints, checks, FKs) para impedir estado inválido/duplicado.
11) - Timeouts e limites por camada (PG/Redis/WAHA/RPC), paginação real e limites de payload.
12) - Segurança: threat model rápido da página/fluxo + hardening (authz, rate limit, validação, redaction, headers, CORS).


# CHECKLIST DE VALIDAÇÃO FINAL ( PAGO ):
1) - React: ErrorBoundary global + integração com monitoramento (captura de erro, fallback UI, retry) e padronização de “errorId/traceId” exibido/logado.
2) - DoD (Definition of Done) com evidências obrigatórias no PR (quality gate, testes, dashboards, alertas, traces, logs).
3) - Correlação ponta a ponta (traceId/requestId) do React → ConnectRPC → Handler → Service → Repo/Redis/WAHA.
4) - Observabilidade de anomalias sem erro (duplicidade, gap/silêncio, backlog) com métricas e alertas dedicados.
5) - Implementação do Datadog para monitorar por completo o fluxo completo de todos os arquivos relacionados a página.


--------------------------------------------------------------------------------------------------------------------------------------------


# CRIAR PLANO DE VERIFICAÇAO DE PARIDADE:
Baseado nos arquivos: *migration/plan/01-pages.md* - *migration/plan/02-migration-plan.md* - *migration/plan/whatsapp.md* - *migration/whatsapp-docs/mapeamento-todo.md* - *migration/whatsapp-docs/waha-openapi.json*, preciso que crie um plano chamado 01-auth-migration.md dentro da pasta *migration/prompts/paridade* checando todos os pontos necessários para garantir 100% que o sistema *antigo-projeto* foi migrado para o novo sistema *novo-projeto*, sem alterar ou perder nenhuma funcionalidade, mesmo com as novas tecnologias. Seja extremamente detalhista e se necessário investigue novamente os dois projetos para tirar dúvidas durante a criação do plano.


--------------------------------------------------------------------------------------------------------------------------------------------


Seguindo o mesmo padrão do plano *migration/prompts/paridade/01-auth-migration.md* crie um novo plano ( arquivo na pasta *migration/prompts/paridade* ) para a página */profile* chamado *05-profile-migration.md*, se necessário, use como referencia os arquivos: *migration/plan/01-pages.md* - *migration/plan/02-migration-plan.md* - *migration/plan/whatsapp.md* e faça novas explorações para deixar o plano extremamente completo e seguindo o mesmo padrão do plano *migration/prompts/paridade/01-auth-migration.md*. IMPORTANTE: Não e para realizar a implementação do plano.


--------------------------------------------------------------------------------------------------------------------------------------------


Baseado no plano migration/prompts/paridade/09-offer-automations-migration.md, crie um novo plano em migration/prompts/checklist chamado:
migration/prompts/checklist/09-offer-automations-checklist.md

OBJETIVO
Verificar e implementar (ou ajustar) TODOS os arquivos citados/criados/modificados no plano migration/prompts/paridade/09-offer-automations-migration.md, incluindo dependências diretas necessárias, garantindo o padrão de qualidade abaixo e mantendo paridade funcional do fluxo de autenticação.

ESCOPO OBRIGATÓRIO
- Liste no topo do novo plano todos os paths (arquivos/pastas) mencionados no migration/prompts/paridade/09-offer-automations-migration.md.
- Considere também arquivos de suporte diretamente necessários (config, interceptors, middlewares, schemas, utils) para que o auth funcione sem erro silencioso.

CHECKLIST (GRATUITO) — deve ser aplicado em todos os arquivos do escopo
1) Verificação de possíveis pontos de falha em todos os arquivos relacionados à autenticação.
2) SonarQube com quality gate agressivo em todos os arquivos do escopo.
3) Análise profunda e correção de todos os possíveis erros silenciosos no escopo.
4) Listar todos os testes possíveis (unitários, integração, e2e) para cobrir o escopo.
5) Análise profunda de performance e otimização do escopo.
6) Análise profunda de segurança do escopo.
7) Padronização de erros (catálogo). Error codes/categorias por camada (Repo/Service/Handler/Client).
8) Go API: recovery de panic (interceptor/middleware) + interceptors de erro (normalização RPC codes) + logging estruturado sem duplicar logs em cascata.
9) Postgres/pgx: mapear erros por SQLSTATE (unique, FK, not found, deadlock/timeout) → erros tipados do domínio + mensagens seguras (sem vazar detalhes).
10) Idempotência + dedupe + invariantes no banco (unique constraints, checks, FKs) para impedir estado inválido/duplicado.
11) Timeouts e limites por camada (PG/Redis/WAHA/RPC), paginação real e limites de payload.
12) Segurança: threat model rápido do fluxo de auth + hardening (authz, rate limit, validação, redaction, headers, CORS).

EXIGÊNCIAS PARA O NOVO PLANO (FORMATO FIXO)
A) Arquivos do escopo (lista completa de paths)
B) Pontos de falha por camada (Front / ConnectRPC / Handler / Service / Repo / DB)
C) Correções necessárias (por arquivo) e como eliminar erros silenciosos
D) Catálogo de erros + mapeamento (incluindo SQLSTATE → domínio → RPC code)
E) Timeouts/limites/retry policy (onde pode e onde não pode)
F) Segurança: threat model e mitigação (por endpoint/fluxo)
G) Performance: queries críticas, índices e otimizações necessárias
H) Testes: lista completa (unit, integração, e2e) com cenários de falha
I) DoD final (critério de aceite) com evidências

SONARQUBE (OBRIGATÓRIO)
- Rodar SonarQube após as alterações e corrigir até o quality gate agressivo passar:
  1) novo-projeto/client
  2) novo-projeto/automatiza-go
- Defina no plano o que significa “quality gate agressivo” (ex.: 0 vulnerabilities, 0 new bugs/blockers, sem new debt) e aplique isso.
- O ciclo deve ser: rodar → corrigir → rodar até PASSAR, em ambos.

REGRAS
- Proibido “catch e ignora”; toda falha deve virar erro tipado ou sinal rastreável.
- Não duplicar logs de erro em cascata (um ponto único por request).
- Manter paridade funcional do auth; não simplificar regras.


--------------------------------------------------------------------------------------------------------------------------------------------

Next steps: Execute tasks in the order defined (P0 → P1 → P2 → P3), run SonarQube after each phase, and correct until quality gates pass.

--------------------------------------------------------------------------------------------------------------------------------------------

 ---                                                                                  
  Recomendações de Otimização                                                          
                                                                                       
  # docker-compose.yml ou .env de produção                                             
                                                                                       
  # PostgreSQL                                                                         
  DB_MAX_CONNS=100                                                                     
                                                                                       
  # Workers                                                                            
  WORKER_CONCURRENCY=25                                                                
                                                                                       
  # Redis                                                                              
  # Aumentar para 1GB no docker-compose                                                
  maxmemory: 1gb                                                                       
                                                                                       
  # Backpressure (ajustar para maior throughput)                                       
  PRODUCER_USER_MAX_ENQUEUED_10MIN=1000                                                
  WHATSAPP_SEND_LIMIT_PER_MINUTE=60                                                    
                                                                                       
  --- 


--------------------------------------------------------------------------------------------------------------------------------------------


#8D2D53


O select de *Selecionar Grupos* do modal *Agendar Nova Ação* da página */jobs* não está buscando os grupos dinâmicamente

Baseado na paginação da tabela de *Grupos* da página */groups* preciso que implemente uma mesma paginação server side com o mesmo visual na tabela *Histórico Completo* da página */history*
















No card *Total de Membros* na pagina dashboard, coloque o seguinte icone: <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="#141B34" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
    <path d="M12.06 21.5124C11.5445 21.8375 11.2868 22 11 22C10.7132 22 10.4554 21.8374 9.94 21.5124L8.02913 20.3073C7.54415 20.0014 7.30166 19.8485 7.03253 19.8397C6.74172 19.8301 6.49493 19.9768 5.97087 20.3073C5.38395 20.6774 4.21687 21.6971 3.46195 21.2108C3 20.9133 3 20.1575 3 18.6458V8.00002C3 5.17158 3 3.75736 3.82699 2.87868C4.65399 2 5.98501 2 8.64706 2H13.3529C16.015 2 17.346 2 18.173 2.87868C19 3.75736 19 5.17158 19 8.00002V12" />
    <path d="M11 11H7" />
    <path d="M17 14V22M21 18L13 18" />
    <path d="M15 7L7 7" />
</svg>







Sem quebrar nenhum breakpoint do tailwind, apenas adicionando novos. Ajuste a pagina */dashboard* para ficar extremamente responsiva, principalmente em dispositivos moveis e em telas grandes

Melhore ainda mais a responsividade da pagina */dashboard*, ainda está quebrando em alguns tamanhos de tela, como muito pegueno e tamanhos medios


  Credenciais:
  - Email: bruno.clara@yahoo.com
  - Senha: semSenha123!
  - Role: admin
  - Plano: pro



<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="#141B34" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
    <path d="M8.00002 8C8.00002 7.42459 8.00002 7.17765 8.04465 6.92457C8.21993 5.93047 8.89355 5.09255 9.83302 4.70001C10.0723 4.60003 10.3559 4.53526 10.9232 4.40573L13.6508 3.78286C17.0405 3.00882 18.7353 2.6218 19.8677 3.51317C21 4.40454 21 6.1257 21 9.56803L21 14.432C21 17.8743 21 19.5955 19.8676 20.4868C18.7353 21.3782 17.0405 20.9912 13.6508 20.2171L10.9232 19.5943C10.3559 19.4647 10.0723 19.4 9.833 19.3C8.89353 18.9074 8.21991 18.0695 8.04462 17.0754C8 16.8224 8 16.5754 8 16" />
    <path d="M13 9C13 9 16 11.2095 16 12C16 12.7906 13 15 13 15M15.5 12H3" />
</svg>