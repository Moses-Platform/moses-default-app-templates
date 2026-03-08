# Fullstack Simple

Minimal Go + React fullstack template for Moses. One API endpoint, one page, no database.

## Architecture

```
Frontend (nginx:8080)  →  Backend (Go:8080)
  React SPA                /health
  /api/* proxy             /api/v1/status
                           /api/openapi.json
```

## Running Locally

```bash
# Backend
cd backend && go run cmd/server/main.go

# Frontend (separate terminal)
cd frontend && npm install && npm run dev
# Open http://localhost:3000
```

## Extending

- Add backend endpoints in `backend/internal/handler/`
- Add frontend pages in `frontend/src/`
- Update `backend/api/openapi.json` for MCP tool generation
- Add database by following the fullstack-showcase pattern
