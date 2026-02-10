---
status: filled
slug: getting-started
category: getting-started
---

# How do I set up and run this project?

## Prerequisites

### Required Tools
- **Node.js** 20+ (LTS recommended)
- **Go** 1.24+
- **Docker** & Docker Compose (for PostgreSQL, Redis)
- **Buf CLI** (for proto generation)
- **Playwright** (for E2E tests)

### Optional Tools
- **Make** (for automation commands)
- **VS Code** with Go and TypeScript extensions

## Installation

### 1. Clone the Repository
```bash
git clone <repository-url>
cd novo-projeto
```

### 2. Start Infrastructure (PostgreSQL + Redis)
```bash
cd automatiza-go
docker-compose up -d
```

### 3. Install Frontend Dependencies
```bash
cd client
npm install
npx playwright install  # For E2E tests
```

### 4. Install Backend Dependencies
```bash
cd automatiza-go
go mod download
```

### 5. Generate Proto Code (if needed)
```bash
cd client
npx buf generate ../proto
```

## Running the Project

### Development Mode

**Terminal 1 - Backend:**
```bash
cd automatiza-go
go run ./cmd/server/main.go
# Server runs on http://localhost:3000
```

**Terminal 2 - Frontend:**
```bash
cd client
npm run dev
# Dev server runs on http://localhost:5173
```

**Terminal 3 - Background Workers (optional):**
```bash
cd automatiza-go
go run ./cmd/server/main.go -worker
```

### Using Makefile
```bash
make dev      # Start frontend dev server
make server   # Start backend server
make worker   # Start background workers
make proto    # Regenerate proto files
```

## Environment Configuration

### Frontend (.env)
```env
VITE_API_URL=http://localhost:3000
```

### Backend (automatiza-go/.env)
```env
DATABASE_URL=postgres://user:password@localhost:5432/automatiza
REDIS_URL=redis://localhost:6379
WAHA_API_URL=<waha-instance-url>
JWT_SECRET=<secret>
```

## Verification

After setup, verify everything works:

1. **Backend health:** `curl http://localhost:3000/health`
2. **Frontend:** Open `http://localhost:5173` in browser
3. **Database:** `docker-compose ps` should show healthy containers

## Common Issues

| Issue | Solution |
|-------|----------|
| Port 3000 in use | Change backend port or kill existing process |
| Proto generation fails | Ensure `buf` CLI is installed |
| Database connection fails | Check Docker containers are running |
| CORS errors | Verify Vite proxy config in `vite.config.ts` |
