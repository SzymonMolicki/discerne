# Discerne

Discerne is a browser-based daily language identification quiz. The backend is written in Go and the future frontend will be a TypeScript application.

## Run Backend

Start PostgreSQL:

```bash
docker compose up -d postgres
```

From `backend/`:

```bash
go run ./cmd/api
```

Health check:

```bash
curl http://localhost:8080/api/v1/health
```

Tests:

```bash
go test ./...
```

Validate seed data:

```bash
go run ./cmd/data-validator
```

Generate a deterministic quiz preview from seed data:

```bash
go run ./cmd/quiz-preview --seed 1 --locale en-US
```

Run database migrations:

```bash
source ../.env
goose -dir ./migrations postgres "$DATABASE_URL" up
```

Import validated seed data into PostgreSQL:

```bash
go run ./cmd/seed-import
```

Stop local services:

```bash
docker compose down
```
