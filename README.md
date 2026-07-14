# Discerne

Discerne is a browser-based daily language identification quiz. The player sees a short text in an unknown language and chooses one of five answers. Each quiz has five questions and the quiz for a given day is the same for every player.

The project does not require user accounts. Progress is tied to an anonymous identifier stored in an HTTP cookie and scoring is calculated on the server.

## How The Quiz Works

For each day, the generator selects five different languages and one approved text for each of them. Four distractors are selected for every correct answer. They are not sampled uniformly: languages that share the same family, group, subgroup, continent, or script get a higher weight, while unrelated languages still keep a non-zero chance of appearing.

Texts and language metadata are stored as seed data in `backend/data`. The current MVP dataset has eight enabled languages: Arabic, English, French, German, Japanese, Polish, Russian and Spanish. The list comes from [backend/data/languages.yaml](backend/data/languages.yaml) and each enabled language has five texts.

The interface is localized for:

- `pl-PL`
- `en-US`
- `es-ES`

Language names shown as answers are localized as well.

## Repository Layout

- `backend/` - Go HTTP API, PostgreSQL migrations, quiz generation logic, seed data validator and seed importer.
- `backend/data/` - versioned language data: families, groups, scripts, localized names and quiz texts.
- `frontend/` - React/Vite TypeScript application with three supported locales.
- `docker-compose.yml` - local PostgreSQL service.

The backend exposes a REST API under `/api/v1`. The main endpoints fetch today's quiz, start an attempt, submit an answer and read the result. Correct answers are not returned before the player submits an answer.

## Local Setup

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

## Useful Tools

`go run ./cmd/data-validator` checks whether seed data satisfies the rules.

`go run ./cmd/quiz-preview --seed 1 --locale en-US` generates a quiz preview without writing to the database.

`go run ./cmd/quiz-generator --days 7` stores future quizzes in PostgreSQL. By default, generation starts from the current date in the `Europe/Warsaw` timezone.

## Configuration

The most important settings are listed in `.env.example`:

- `DATABASE_URL` - PostgreSQL connection URL.
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
