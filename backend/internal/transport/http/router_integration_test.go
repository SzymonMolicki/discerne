package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"discerne/backend/internal/config"
	"discerne/backend/internal/quiz"
	"discerne/backend/internal/quizdb"
	"discerne/backend/internal/seeddata"
	"discerne/backend/internal/seedimport"

	"github.com/jackc/pgx/v5"
)

func TestDailyQuizHTTPFlowIntegration(t *testing.T) {
	databaseURL := os.Getenv("DISCERNE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set DISCERNE_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, schema := openIntegrationDatabase(ctx, t, databaseURL)
	store := quizdb.NewStore(conn)

	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	cfg := config.Config{
		AppName:           "Discerne",
		AppTimezone:       location,
		DeviceCookieName:  "discerne_device",
		DistractorWeights: quiz.DefaultDistractorWeights(),
	}

	importSeedData(ctx, t, conn)
	quizDate := time.Now().In(location).Format("2006-01-02")
	saveGeneratedQuiz(ctx, t, conn, quizDate, cfg.DistractorWeights)

	router := NewRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), store)

	initialQuiz := getTodayQuiz(t, router, nil)
	if initialQuiz.Attempt.Status != "not_started" {
		t.Fatalf("initial attempt status = %q, want %q", initialQuiz.Attempt.Status, "not_started")
	}
	if len(initialQuiz.Questions) != quiz.DefaultQuestionCount {
		t.Fatalf("initial question count = %d, want %d", len(initialQuiz.Questions), quiz.DefaultQuestionCount)
	}

	attempt, deviceCookie := startAttempt(t, router, nil)
	if attempt.Status != "in_progress" {
		t.Fatalf("started attempt status = %q, want %q", attempt.Status, "in_progress")
	}
	if deviceCookie.Value == "" {
		t.Fatal("device cookie value is empty")
	}

	loadedQuiz := getTodayQuiz(t, router, deviceCookie)
	if loadedQuiz.Attempt.AttemptID != attempt.AttemptID {
		t.Fatalf("loaded attempt id = %q, want %q", loadedQuiz.Attempt.AttemptID, attempt.AttemptID)
	}
	if loadedQuiz.Attempt.Status != "in_progress" {
		t.Fatalf("loaded attempt status = %q, want %q", loadedQuiz.Attempt.Status, "in_progress")
	}

	submittedAnswers := make([]submitAnswerResponse, 0, len(loadedQuiz.Questions))
	for _, question := range loadedQuiz.Questions {
		if len(question.Options) != quiz.DefaultOptionCount {
			t.Fatalf("question %q option count = %d, want %d", question.ID, len(question.Options), quiz.DefaultOptionCount)
		}

		answer := submitQuestionAnswer(t, router, deviceCookie, attempt.AttemptID, question.ID, question.Options[0].LanguageID)
		if answer.QuestionID != question.ID {
			t.Fatalf("answer question id = %q, want %q", answer.QuestionID, question.ID)
		}
		if answer.SelectedLanguageID != question.Options[0].LanguageID {
			t.Fatalf("selected language id = %q, want %q", answer.SelectedLanguageID, question.Options[0].LanguageID)
		}
		if answer.CorrectLanguageID == "" {
			t.Fatal("correct language id is empty")
		}
		submittedAnswers = append(submittedAnswers, answer)
	}

	result := getAttemptResult(t, router, deviceCookie, attempt.AttemptID)
	if result.Status != "completed" {
		t.Fatalf("result status = %q, want %q", result.Status, "completed")
	}
	if result.AnsweredCount != quiz.DefaultQuestionCount {
		t.Fatalf("answered count = %d, want %d", result.AnsweredCount, quiz.DefaultQuestionCount)
	}
	if result.QuestionCount != quiz.DefaultQuestionCount {
		t.Fatalf("question count = %d, want %d", result.QuestionCount, quiz.DefaultQuestionCount)
	}
	if result.Score == nil {
		t.Fatal("score is nil for completed attempt")
	}
	if len(result.Answers) != quiz.DefaultQuestionCount {
		t.Fatalf("result answers count = %d, want %d", len(result.Answers), quiz.DefaultQuestionCount)
	}

	completedQuiz := getTodayQuiz(t, router, deviceCookie)
	if completedQuiz.Attempt.Status != "completed" {
		t.Fatalf("completed quiz attempt status = %q, want %q", completedQuiz.Attempt.Status, "completed")
	}
	if completedQuiz.Attempt.Score == nil || *completedQuiz.Attempt.Score != *result.Score {
		t.Fatalf("completed quiz score = %v, want %d", completedQuiz.Attempt.Score, *result.Score)
	}
	if len(completedQuiz.Attempt.Answers) != len(submittedAnswers) {
		t.Fatalf("completed quiz answers count = %d, want %d", len(completedQuiz.Attempt.Answers), len(submittedAnswers))
	}

	restartResponse := httptest.NewRecorder()
	restartRequest := httptest.NewRequest(http.MethodPost, "/api/v1/quizzes/today/attempt", nil)
	restartRequest.AddCookie(deviceCookie)
	router.ServeHTTP(restartResponse, restartRequest)
	if restartResponse.Code != http.StatusConflict {
		t.Fatalf("restart status = %d, want %d", restartResponse.Code, http.StatusConflict)
	}

	var restartError errorResponse
	decodeResponse(t, restartResponse, &restartError)
	if restartError.Error != "attempt_already_completed" {
		t.Fatalf("restart error = %q, want %q", restartError.Error, "attempt_already_completed")
	}

	t.Logf("integration test used temporary schema %s", schema)
}

func openIntegrationDatabase(ctx context.Context, t *testing.T, databaseURL string) (*pgx.Conn, string) {
	t.Helper()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close(context.Background())
	})

	schema := fmt.Sprintf("discerne_it_%d", time.Now().UnixNano())
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
			t.Logf("drop schema %s: %v", schema, err)
		}
	})

	if _, err := conn.Exec(ctx, "SET search_path TO "+schema+", public"); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	applyInitialMigration(ctx, t, conn)

	return conn, schema
}

func applyInitialMigration(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	path := filepath.Join("..", "..", "..", "migrations", "00001_initial_schema.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}

	contents := string(data)
	_, upSQL, found := strings.Cut(contents, "-- +goose Up")
	if !found {
		t.Fatalf("migration %s does not contain goose Up marker", path)
	}
	upSQL, _, _ = strings.Cut(upSQL, "-- +goose Down")

	for _, statement := range strings.Split(upSQL, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := conn.Exec(ctx, statement); err != nil {
			t.Fatalf("apply migration statement %q: %v", statement, err)
		}
	}
}

func importSeedData(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	dataDir := filepath.Join("..", "..", "..", "data")
	catalog, err := seeddata.Load(dataDir)
	if err != nil {
		t.Fatalf("load seed data: %v", err)
	}
	if report := seeddata.ValidateCatalog(catalog); len(report.Errors) > 0 {
		t.Fatalf("seed data is invalid: %s", strings.Join(report.Errors, "; "))
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin seed import transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	if err := seedimport.Import(ctx, tx, catalog); err != nil {
		t.Fatalf("import seed data: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit seed import transaction: %v", err)
	}
}

func saveGeneratedQuiz(ctx context.Context, t *testing.T, conn *pgx.Conn, quizDate string, weights quiz.DistractorWeights) {
	t.Helper()

	catalog, err := quizdb.LoadCatalog(ctx, conn)
	if err != nil {
		t.Fatalf("load quiz catalog: %v", err)
	}

	generator := quiz.Generator{
		Weights: weights,
		Random:  quiz.NewSeededRandomSource(1),
	}
	generatedQuiz, err := generator.Generate(catalog.Languages)
	if err != nil {
		t.Fatalf("generate quiz: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin quiz save transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	saved, err := quizdb.SaveDailyQuiz(ctx, tx, quizDate, generatedQuiz)
	if err != nil {
		t.Fatalf("save daily quiz: %v", err)
	}
	if !saved {
		t.Fatalf("daily quiz for %s was not saved", quizDate)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit quiz save transaction: %v", err)
	}
}

func getTodayQuiz(t *testing.T, router http.Handler, cookie *http.Cookie) todayQuizResponse {
	t.Helper()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/quizzes/today?locale=en-US", nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("today quiz status = %d, want %d, body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body todayQuizResponse
	decodeResponse(t, response, &body)
	return body
}

func startAttempt(t *testing.T, router http.Handler, cookie *http.Cookie) (startAttemptResponse, *http.Cookie) {
	t.Helper()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/quizzes/today/attempt", nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("start attempt status = %d, want %d, body: %s", response.Code, http.StatusCreated, response.Body.String())
	}

	var body startAttemptResponse
	decodeResponse(t, response, &body)

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("start attempt set %d cookies, want %d", len(cookies), 1)
	}

	return body, cookies[0]
}

func submitQuestionAnswer(
	t *testing.T,
	router http.Handler,
	cookie *http.Cookie,
	attemptID string,
	questionID string,
	selectedLanguageID string,
) submitAnswerResponse {
	t.Helper()

	body := fmt.Sprintf(
		`{"questionId":%q,"selectedLanguageId":%q,"responseTimeMs":100}`,
		questionID,
		selectedLanguageID,
	)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/attempts/"+attemptID+"/answers", strings.NewReader(body))
	request.AddCookie(cookie)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("submit answer status = %d, want %d, body: %s", response.Code, http.StatusCreated, response.Body.String())
	}

	var answer submitAnswerResponse
	decodeResponse(t, response, &answer)
	return answer
}

func getAttemptResult(t *testing.T, router http.Handler, cookie *http.Cookie, attemptID string) attemptResultResponse {
	t.Helper()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/attempts/"+attemptID, nil)
	request.AddCookie(cookie)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("attempt result status = %d, want %d, body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var result attemptResultResponse
	decodeResponse(t, response, &result)
	return result
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, body any) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), body); err != nil {
		t.Fatalf("decode response body %q: %v", response.Body.String(), err)
	}
}
