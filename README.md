# SmartFM LMS — Go Backend

Binary builds, database migrations, code generation.

## Quick Start

```bash
cp .env.example .env
make dev
```

## Commands

| Command | Description |
|---|---|
| `make dev` | Run with hot-reload (requires `air`) |
| `make build` | Compile production binary |
| `make run` | Build + run |
| `make migrate-up` | Run database migrations |
| `make migrate-down` | Roll back last migration |
| `make sqlc` | Regenerate type-safe SQL |
| `make test` | Run all tests |
| `make lint` | Run golangci-lint |

## Project Structure

```
cmd/api/          → Application entry point
internal/
  config/         → Environment + config loading
  middleware/     → JWT auth, CORS, rate-limit
  domain/         → Domain models (shared across services)
  auth/           → Auth handlers + JWT logic
  user/           → User/Employee service
  course/         → Course + Module CRUD
  module/         → Module management
  assessment/     → Quiz + scoring
  analytics/      → ClickHouse analytics
  leaderboard/    → Redis sorted-set leaderboard
  content/        → Content library
  storage/        → MinIO client
  database/       → PostgreSQL connection
pkg/
  response/       → Standardized API response helpers
db/
  migrations/     → SQL migration files
  queries/        → sqlc query definitions
```
