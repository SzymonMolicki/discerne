package httptransport

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"discerne/backend/internal/config"
	"discerne/backend/internal/quizdb"
)

func TestHealthEndpoint(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:     "Discerne",
		HTTPAddress: ":8080",
		AppTimezone: location,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body struct {
		AppName  string `json:"appName"`
		Status   string `json:"status"`
		Timezone string `json:"timezone"`
		Now      string `json:"now"`
	}
	bodyBytes := response.Body.Bytes()
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.AppName != "Discerne" {
		t.Fatalf("AppName = %q, want %q", body.AppName, "Discerne")
	}

	if body.Status != "ok" {
		t.Fatalf("Status = %q, want %q", body.Status, "ok")
	}

	if body.Timezone != "Europe/Warsaw" {
		t.Fatalf("Timezone = %q, want %q", body.Timezone, "Europe/Warsaw")
	}

	if body.Now == "" {
		t.Fatal("Now is empty")
	}
}

func TestTodayQuizEndpoint(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	reader := &fakeQuizService{
		quiz: quizdb.DailyQuiz{
			QuizDate: "2026-08-01",
			Questions: []quizdb.DailyQuizQuestion{
				{
					ID:       "question-id",
					Position: 1,
					Text:     "Example text.",
					Options: []quizdb.DailyQuizOption{
						{LanguageID: "language-id", Position: 1, Name: "hiszpański"},
					},
				},
			},
		},
	}

	router := NewRouter(config.Config{
		AppName:     "Discerne",
		HTTPAddress: ":8080",
		AppTimezone: location,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), reader)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/quizzes/today?locale=pl-PL", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if reader.locale != "pl-PL" {
		t.Fatalf("locale = %q, want %q", reader.locale, "pl-PL")
	}

	var body struct {
		QuizDate string `json:"quizDate"`
		Attempt  struct {
			Status string `json:"status"`
		} `json:"attempt"`
		Questions []struct {
			ID      string `json:"id"`
			Text    string `json:"text"`
			Options []struct {
				LanguageID string `json:"languageId"`
				Name       string `json:"name"`
			} `json:"options"`
		} `json:"questions"`
	}
	bodyBytes := response.Body.Bytes()
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.QuizDate != "2026-08-01" {
		t.Fatalf("QuizDate = %q, want %q", body.QuizDate, "2026-08-01")
	}
	if body.Attempt.Status != "not_started" {
		t.Fatalf("Attempt.Status = %q, want %q", body.Attempt.Status, "not_started")
	}
	if len(body.Questions) != 1 {
		t.Fatalf("len(Questions) = %d, want %d", len(body.Questions), 1)
	}
	if body.Questions[0].Options[0].Name != "hiszpański" {
		t.Fatalf("option name = %q, want %q", body.Questions[0].Options[0].Name, "hiszpański")
	}

	bodyText := string(bodyBytes)
	if strings.Contains(bodyText, "isCorrect") || strings.Contains(bodyText, "correctLanguageId") {
		t.Fatal("response exposes answer correctness")
	}
}

func TestTodayQuizEndpointRejectsUnsupportedLocale(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:     "Discerne",
		HTTPAddress: ":8080",
		AppTimezone: location,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeQuizService{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/quizzes/today?locale=fr-FR", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestTodayQuizEndpointReturnsNotFound(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:     "Discerne",
		HTTPAddress: ":8080",
		AppTimezone: location,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeQuizService{loadErr: quizdb.ErrDailyQuizNotFound})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/quizzes/today", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestStartAttemptEndpointCreatesAttemptAndSetsCookie(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	service := &fakeQuizService{
		attempt: quizdb.Attempt{
			ID:       "attempt-id",
			DeviceID: "device-id",
			Status:   "in_progress",
		},
	}
	router := NewRouter(config.Config{
		AppName:          "Discerne",
		HTTPAddress:      ":8080",
		AppTimezone:      location,
		DeviceCookieName: "discerne_device",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), service)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/quizzes/today/attempt", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if service.startDeviceID != "" {
		t.Fatalf("startDeviceID = %q, want empty", service.startDeviceID)
	}

	var body struct {
		AttemptID string `json:"attemptId"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AttemptID != "attempt-id" {
		t.Fatalf("AttemptID = %q, want %q", body.AttemptID, "attempt-id")
	}
	if body.Status != "in_progress" {
		t.Fatalf("Status = %q, want %q", body.Status, "in_progress")
	}

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("len(cookies) = %d, want %d", len(cookies), 1)
	}
	cookie := cookies[0]
	if cookie.Name != "discerne_device" {
		t.Fatalf("cookie.Name = %q, want %q", cookie.Name, "discerne_device")
	}
	if cookie.Value != "device-id" {
		t.Fatalf("cookie.Value = %q, want %q", cookie.Value, "device-id")
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie.HttpOnly = false, want true")
	}
	if cookie.Path != "/" {
		t.Fatalf("cookie.Path = %q, want %q", cookie.Path, "/")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie.SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
	}
}

func TestStartAttemptEndpointUsesExistingDeviceCookie(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	service := &fakeQuizService{
		attempt: quizdb.Attempt{
			ID:       "attempt-id",
			DeviceID: "existing-device-id",
			Status:   "in_progress",
		},
	}
	router := NewRouter(config.Config{
		AppName:          "Discerne",
		HTTPAddress:      ":8080",
		AppTimezone:      location,
		DeviceCookieName: "discerne_device",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), service)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/quizzes/today/attempt", nil)
	request.AddCookie(&http.Cookie{Name: "discerne_device", Value: "existing-device-id"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if service.startDeviceID != "existing-device-id" {
		t.Fatalf("startDeviceID = %q, want %q", service.startDeviceID, "existing-device-id")
	}
}

func TestStartAttemptEndpointReturnsNotFound(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:          "Discerne",
		HTTPAddress:      ":8080",
		AppTimezone:      location,
		DeviceCookieName: "discerne_device",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeQuizService{startErr: quizdb.ErrDailyQuizNotFound})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/quizzes/today/attempt", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitAnswerEndpoint(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	service := &fakeQuizService{
		answer: quizdb.AnswerSubmission{
			QuestionID:         "question-id",
			SelectedLanguageID: "language-id",
			CorrectLanguageID:  "correct-language-id",
			IsCorrect:          true,
		},
	}
	router := NewRouter(config.Config{
		AppName:          "Discerne",
		HTTPAddress:      ":8080",
		AppTimezone:      location,
		DeviceCookieName: "discerne_device",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/attempts/attempt-id/answers",
		strings.NewReader(`{"questionId":"question-id","selectedLanguageId":"language-id","responseTimeMs":5400}`),
	)
	request.AddCookie(&http.Cookie{Name: "discerne_device", Value: "device-id"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if service.submitInput.AttemptID != "attempt-id" {
		t.Fatalf("AttemptID = %q, want %q", service.submitInput.AttemptID, "attempt-id")
	}
	if service.submitInput.DeviceID != "device-id" {
		t.Fatalf("DeviceID = %q, want %q", service.submitInput.DeviceID, "device-id")
	}
	if service.submitInput.QuestionID != "question-id" {
		t.Fatalf("QuestionID = %q, want %q", service.submitInput.QuestionID, "question-id")
	}
	if service.submitInput.SelectedLanguageID != "language-id" {
		t.Fatalf("SelectedLanguageID = %q, want %q", service.submitInput.SelectedLanguageID, "language-id")
	}
	if service.submitInput.ResponseTimeMS == nil || *service.submitInput.ResponseTimeMS != 5400 {
		t.Fatalf("ResponseTimeMS = %v, want %d", service.submitInput.ResponseTimeMS, 5400)
	}

	var body struct {
		QuestionID         string `json:"questionId"`
		SelectedLanguageID string `json:"selectedLanguageId"`
		CorrectLanguageID  string `json:"correctLanguageId"`
		IsCorrect          bool   `json:"isCorrect"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.QuestionID != "question-id" {
		t.Fatalf("QuestionID = %q, want %q", body.QuestionID, "question-id")
	}
	if body.SelectedLanguageID != "language-id" {
		t.Fatalf("SelectedLanguageID = %q, want %q", body.SelectedLanguageID, "language-id")
	}
	if body.CorrectLanguageID != "correct-language-id" {
		t.Fatalf("CorrectLanguageID = %q, want %q", body.CorrectLanguageID, "correct-language-id")
	}
	if !body.IsCorrect {
		t.Fatal("IsCorrect = false, want true")
	}
}

func TestSubmitAnswerEndpointRejectsInvalidRequest(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:     "Discerne",
		HTTPAddress: ":8080",
		AppTimezone: location,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeQuizService{})

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/attempts/attempt-id/answers",
		strings.NewReader(`{"questionId":"question-id"`),
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSubmitAnswerEndpointReturnsNotFound(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:     "Discerne",
		HTTPAddress: ":8080",
		AppTimezone: location,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeQuizService{submitErr: quizdb.ErrAttemptNotFound})

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/attempts/missing-attempt/answers",
		strings.NewReader(`{"questionId":"question-id","selectedLanguageId":"language-id"}`),
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestSubmitAnswerEndpointRejectsInvalidAnswer(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:     "Discerne",
		HTTPAddress: ":8080",
		AppTimezone: location,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeQuizService{submitErr: quizdb.ErrInvalidAnswer})

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/attempts/attempt-id/answers",
		strings.NewReader(`{"questionId":"question-id","selectedLanguageId":"language-id"}`),
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSubmitAnswerEndpointRejectsDuplicateAnswer(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:     "Discerne",
		HTTPAddress: ":8080",
		AppTimezone: location,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeQuizService{submitErr: quizdb.ErrAnswerAlreadySubmitted})

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/attempts/attempt-id/answers",
		strings.NewReader(`{"questionId":"question-id","selectedLanguageId":"language-id"}`),
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestGetAttemptEndpoint(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	score := 4
	service := &fakeQuizService{
		result: quizdb.AttemptResult{
			ID:            "attempt-id",
			Status:        "completed",
			AnsweredCount: 5,
			QuestionCount: 5,
			Score:         &score,
		},
	}
	router := NewRouter(config.Config{
		AppName:          "Discerne",
		HTTPAddress:      ":8080",
		AppTimezone:      location,
		DeviceCookieName: "discerne_device",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), service)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/attempts/attempt-id", nil)
	request.AddCookie(&http.Cookie{Name: "discerne_device", Value: "device-id"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if service.resultAttemptID != "attempt-id" {
		t.Fatalf("resultAttemptID = %q, want %q", service.resultAttemptID, "attempt-id")
	}
	if service.resultDeviceID != "device-id" {
		t.Fatalf("resultDeviceID = %q, want %q", service.resultDeviceID, "device-id")
	}

	var body struct {
		AttemptID     string `json:"attemptId"`
		Status        string `json:"status"`
		AnsweredCount int    `json:"answeredCount"`
		QuestionCount int    `json:"questionCount"`
		Score         *int   `json:"score"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AttemptID != "attempt-id" {
		t.Fatalf("AttemptID = %q, want %q", body.AttemptID, "attempt-id")
	}
	if body.Status != "completed" {
		t.Fatalf("Status = %q, want %q", body.Status, "completed")
	}
	if body.AnsweredCount != 5 {
		t.Fatalf("AnsweredCount = %d, want %d", body.AnsweredCount, 5)
	}
	if body.QuestionCount != 5 {
		t.Fatalf("QuestionCount = %d, want %d", body.QuestionCount, 5)
	}
	if body.Score == nil || *body.Score != 4 {
		t.Fatalf("Score = %v, want %d", body.Score, 4)
	}
}

func TestGetAttemptEndpointReturnsNotFound(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	router := NewRouter(config.Config{
		AppName:          "Discerne",
		HTTPAddress:      ":8080",
		AppTimezone:      location,
		DeviceCookieName: "discerne_device",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeQuizService{resultErr: quizdb.ErrAttemptNotFound})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/attempts/missing-attempt", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

type fakeQuizService struct {
	quiz            quizdb.DailyQuiz
	loadErr         error
	locale          string
	attempt         quizdb.Attempt
	startErr        error
	startDeviceID   string
	answer          quizdb.AnswerSubmission
	submitErr       error
	submitInput     quizdb.SubmitAnswerInput
	result          quizdb.AttemptResult
	resultErr       error
	resultAttemptID string
	resultDeviceID  string
}

func (reader *fakeQuizService) LoadDailyQuiz(_ context.Context, _ time.Time, locale string) (quizdb.DailyQuiz, error) {
	reader.locale = locale
	return reader.quiz, reader.loadErr
}

func (reader *fakeQuizService) StartAttempt(_ context.Context, _ string, deviceID string) (quizdb.Attempt, error) {
	reader.startDeviceID = deviceID
	return reader.attempt, reader.startErr
}

func (reader *fakeQuizService) SubmitAnswer(_ context.Context, input quizdb.SubmitAnswerInput) (quizdb.AnswerSubmission, error) {
	reader.submitInput = input
	return reader.answer, reader.submitErr
}

func (reader *fakeQuizService) LoadAttempt(_ context.Context, attemptID string, deviceID string) (quizdb.AttemptResult, error) {
	reader.resultAttemptID = attemptID
	reader.resultDeviceID = deviceID
	return reader.result, reader.resultErr
}
