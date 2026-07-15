# Discerne

Discerne is a browser-based daily language identification quiz. The player sees a short text in an unknown language and chooses one of five answers. Each quiz has five questions and the quiz for a given day is the same for every player.

The project does not require user accounts. Progress is tied to an anonymous identifier stored in an HTTP cookie and scoring is calculated on the server.

## How The Quiz Works

For each day, the generator selects five different languages and one approved text for each of them. Four distractors are selected for every correct answer. They are not sampled uniformly: languages that share the same family, group, subgroup, continent, or script get a higher weight, while unrelated languages still keep a non-zero chance of appearing.

Texts and language metadata are stored as seed data in `backend/data`.

The interface is localized for:

- `pl-PL`
- `en-US`
- `es-ES`

Language names shown as answers are localized as well.

## Repository Layout

- `backend/` - Go HTTP API, PostgreSQL migrations, quiz generation logic, seed data validator and seed importer.
- `backend/data/` - versioned language data: families, groups, scripts, localized names and quiz texts.
- `frontend/` - React/Vite TypeScript application with three supported locales.
- `docker-compose.yml` - local PostgreSQL, backend, frontend and backend tool services.

The backend exposes a REST API under `/api/v1`. The main endpoints fetch today's quiz, start an attempt, submit an answer and read the result. Correct answers are not returned before the player submits an answer.

Health endpoints:

- `GET /api/v1/health` is the liveness endpoint. It checks whether the API process is running.
- `GET /api/v1/health/ready` checks whether the API can serve quiz traffic. It verifies database connectivity, today's quiz and the number of future quizzes. It returns `503` when the application is not ready.

## Local Development

Requirements: Go matching `backend/go.mod`, Node.js with npm, Docker and `goose` for migrations.

First run:

```bash
cp .env.example .env
docker compose up -d postgres

cd backend
set -a; source ../.env; set +a
goose -dir ./migrations postgres "$DATABASE_URL" up
go run ./cmd/seed-import
go run ./cmd/quiz-generator --days 7
go run ./cmd/api
```

Start the frontend in a second terminal:

```bash
cd frontend
npm install
npm run dev
```

By default, the API runs at `http://localhost:8080` and Vite runs at `http://localhost:5173`. The Vite proxy forwards `/api` requests to the backend.

On later runs, you usually only need to start PostgreSQL, the API and the frontend in separate terminals:

```bash
docker compose up -d postgres
cd backend && go run ./cmd/api
cd frontend && npm run dev
```

## Docker Setup

The Docker setup runs PostgreSQL, the Go API and the built frontend. The frontend is served by nginx and proxies `/api` to the backend service inside the Docker network.

First run:

```bash
cp .env.example .env
docker compose up -d postgres
docker compose run --rm --interactive=false -T migrate
docker compose run --rm --interactive=false -T data-validator
docker compose run --rm --interactive=false -T seed-import
docker compose run --rm --interactive=false -T quiz-generator
docker compose up -d backend frontend
```

`--interactive=false -T` makes the one-off commands safe to paste as one block in a terminal.

Then open:

```text
http://localhost:5173
```

The backend container healthcheck uses `/api/v1/health/ready`, so the backend becomes healthy only after migrations, seed import and quiz generation have prepared data for today.

If local Go or Vite processes already use the default ports, override them for Docker:

```bash
BACKEND_PORT=18080 FRONTEND_PORT=15173 docker compose up -d backend frontend
```

## Cloud Deployment

The production deployment needs three runtime parts:

- PostgreSQL database.
- Backend API container.
- Frontend container or another static hosting layer that proxies `/api` to the backend.

Required backend environment variables:

- `DATABASE_URL` - production PostgreSQL connection URL.
- `APP_NAME` - `Discerne`.
- `APP_TIMEZONE` - quiz publication timezone, currently `Europe/Warsaw`.
- `HTTP_ADDRESS` - backend listen address, usually `:8080` inside the container.
- `DEVICE_COOKIE_NAME` - anonymous browser cookie name, for example `discerne_device`.
- `SECURE_COOKIES` - use `true` in production behind HTTPS.
- `DISTRACTOR_*_WEIGHT` - distractor scoring weights. Use the same values as `.env.example` unless there is a reason to tune gameplay.

Initial deployment order:

1. Create the PostgreSQL database.
2. Run migrations:

   ```bash
   ./bin/goose -dir ./migrations postgres "$DATABASE_URL" up
   ```

3. Validate seed data:

   ```bash
   ./bin/data-validator
   ```

4. Import seed data:

   ```bash
   ./bin/seed-import
   ```

5. Generate initial quizzes:

   ```bash
   ./bin/quiz-generator --days 30
   ```

6. Start the backend API.
7. Start the frontend.
8. Check readiness:

   ```bash
   curl https://your-domain.example/api/v1/health/ready
   ```

Use health checks:

- liveness probe: `GET /api/v1/health`;
- readiness probe: `GET /api/v1/health/ready`.

### Scheduled Quiz Generation

Daily quizzes should be generated by a scheduled job, not by user requests. The API should only serve already generated quizzes.

Run this command periodically, for example once per day:

```bash
./bin/quiz-generator --ensure-future 7 --generate-days 30
```

It checks how many quizzes exist from tomorrow onward. If fewer than 7 future quizzes are available, it generates 30 more days. Existing quiz dates are skipped, so rerunning the job is safe.

In Docker Compose the same job is available as:

```bash
docker compose run --rm --interactive=false -T quiz-ensure-future
```

In a cloud provider this should be configured as a cron-style job, scheduled task or one-off container job using the same backend image and the production `DATABASE_URL`.

## Useful Tools

`go run ./cmd/data-validator` checks whether seed data satisfies the rules.

`go run ./cmd/quiz-preview --seed 1 --locale en-US` generates a quiz preview without writing to the database.

`go run ./cmd/quiz-generator --days 7` stores future quizzes in PostgreSQL. By default, generation starts from the current date in the `Europe/Warsaw` timezone.

`go run ./cmd/quiz-generator --ensure-future 7 --generate-days 30` is the scheduled mode used by the cloud deployment.

## Configuration

The most important settings are listed in `.env.example`:

- `DATABASE_URL` - PostgreSQL connection URL.
- `BACKEND_PORT` and `FRONTEND_PORT` - host ports used by Docker Compose.
- `APP_TIMEZONE` - quiz publication timezone, defaulting to `Europe/Warsaw`.
- `DEVICE_COOKIE_NAME` and `SECURE_COOKIES` - anonymous device cookie settings.
- `DISTRACTOR_*_WEIGHT` - weights used when selecting distractors.

## Verification

Backend:

```bash
cd backend
go test ./...
go vet ./...
```

API readiness:

```bash
curl http://localhost:8080/api/v1/health/ready
```

PostgreSQL integration test:

```bash
docker compose up -d postgres
cd backend
set -a; source ../.env; set +a
DISCERNE_TEST_DATABASE_URL="$DATABASE_URL" go test ./internal/transport/http -run TestDailyQuizHTTPFlowIntegration -count=1 -v
```

Frontend:

```bash
cd frontend
npm run lint
npm run build
```
