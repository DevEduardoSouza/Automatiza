# SonarQube Setup - Automatiza

Quality gates agressivos para a stack **React 19 + Golang/Gin + WAHA Plus (GOWS) + PostgreSQL + gRPC/Protobuf**.

## Quick Start

```bash
# 1. Iniciar SonarQube
make sonar-up

# 2. Aguardar ~2-3 minutos e acessar:
#    http://localhost:9000/sonarqube
#    Login: admin / admin (trocar na primeira vez)

# 3. Configurar quality gates e projetos
make sonar-setup

# 4. Copiar o token gerado e exportar:
export SONAR_TOKEN=<token-gerado>

# 5. Rodar análise
make sonar-analyze
```

## Quality Gates Configurados

### Código Novo (foco principal)

| Métrica | Threshold | Descrição |
|---------|-----------|-----------|
| Coverage | >= 80% | Cobertura de testes em código novo |
| Duplicated Lines | <= 3% | Código duplicado |
| Maintainability | A | Rating de manutenibilidade |
| Reliability | A | Rating de confiabilidade (bugs) |
| Security | A | Rating de segurança (vulnerabilidades) |
| Security Hotspots | 100% reviewed | Hotspots de segurança revisados |

### Código Total

| Métrica | Threshold | Descrição |
|---------|-----------|-----------|
| Coverage | >= 70% | Cobertura total de testes |
| Duplicated Lines | <= 5% | Código duplicado total |
| Maintainability | A | Technical Debt Ratio < 5% |
| Reliability | A | 0 bugs |
| Security | A | 0 vulnerabilidades |

### Tolerância Zero

| Métrica | Threshold |
|---------|-----------|
| Blocker Issues | 0 |
| Critical Issues (código novo) | 0 |
| Major Issues | <= 5 |

## Comandos Disponíveis

```bash
# SonarQube Server
make sonar-up          # Iniciar servidor
make sonar-down        # Parar servidor
make sonar-logs        # Ver logs
make sonar-clean       # Remover containers e volumes

# Setup
make sonar-setup       # Configurar quality gates (rodar 1x após início)

# Análise
make sonar-analyze           # Analisar todos os projetos
make sonar-analyze-client    # Analisar apenas React
make sonar-analyze-go        # Analisar apenas Go
```

## Requisitos

### sonar-scanner CLI

```bash
# macOS
brew install sonar-scanner

# Linux
curl -o sonar-scanner.zip https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/sonar-scanner-cli-5.0.1.3006-linux.zip
unzip sonar-scanner.zip
export PATH=$PATH:$(pwd)/sonar-scanner-5.0.1.3006-linux/bin
```

### golangci-lint (para Go)

```bash
# macOS
brew install golangci-lint

# Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

## Projetos Configurados

| Projeto | Key | Stack |
|---------|-----|-------|
| React Frontend | `automatiza-client` | React 19, TypeScript, Vite |
| Go Backend | `automatiza-go` | Go 1.24, Gin, Connect-RPC |

## Integração CI/CD

### GitHub Actions

```yaml
name: SonarQube Analysis

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  sonarqube:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: client/package-lock.json

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Install dependencies
        run: |
          cd client && npm ci
          cd ../automatiza-go && go mod download

      - name: Run tests with coverage
        run: |
          cd client && npm run test:coverage
          cd ../automatiza-go && go test -coverprofile=coverage.out -json ./... > test-report.json

      - name: SonarQube Scan
        uses: sonarsource/sonarqube-scan-action@master
        env:
          SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}
          SONAR_HOST_URL: ${{ secrets.SONAR_HOST_URL }}
```

## Troubleshooting

### SonarQube não inicia

```bash
# Verificar se Elasticsearch precisa de mais memória
sudo sysctl -w vm.max_map_count=262144

# Ou adicionar ao docker-compose:
# ulimits:
#   memlock:
#     soft: -1
#     hard: -1
```

### Erro de autenticação

1. Verificar se o token foi gerado corretamente
2. Verificar se `SONAR_TOKEN` está exportado
3. Regenerar token em: `http://localhost:9000/sonarqube/account/security`

### Análise não encontra arquivos

1. Verificar `sonar.sources` no `sonar-project.properties`
2. Verificar se não está excluindo arquivos importantes
3. Rodar com debug: `sonar-scanner -X`

## Recursos

- [SonarQube Documentation](https://docs.sonarsource.com/sonarqube/latest/)
- [Quality Gates](https://docs.sonarsource.com/sonarqube/latest/user-guide/quality-gates/)
- [Go Plugin](https://docs.sonarsource.com/sonarqube/latest/analyzing-source-code/languages/go/)
- [JavaScript/TypeScript Plugin](https://docs.sonarsource.com/sonarqube/latest/analyzing-source-code/languages/javascript-typescript/overview/)
