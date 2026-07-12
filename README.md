# Discerne

Discerne is a browser-based daily language identification quiz. The backend is written in Go and the future frontend will be a TypeScript application.

## Current State

The repository currently contains:

- configuration loading;
- `APP_TIMEZONE=Europe/Warsaw` by default;
- `APP_NAME=Discerne` by default;
- a minimal API server;
- `GET /api/v1/health`;
- seed metadata for 8 enabled languages;
- 5 seed texts for each enabled language;
- configurable distractor weight calculation;
- weighted distractor selection;
- random source for deterministic quiz logic;
- first in-memory quiz generator shape;
- quiz preview command using seed data.

## Run Backend

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
