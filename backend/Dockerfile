FROM golang:1.26-alpine AS build

WORKDIR /src/backend

RUN apk add --no-cache git

COPY backend/go.mod backend/go.sum ./
RUN go mod download
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

COPY backend/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/data-validator ./cmd/data-validator
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/quiz-generator ./cmd/quiz-generator
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/quiz-preview ./cmd/quiz-preview
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/seed-import ./cmd/seed-import

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

RUN adduser -D -H -u 10001 discerne

COPY --from=build /out/ /app/bin/
COPY --from=build /go/bin/goose /app/bin/goose
COPY backend/data/ /app/data/
COPY backend/migrations/ /app/migrations/

USER discerne

EXPOSE 8080

CMD ["./bin/api"]
