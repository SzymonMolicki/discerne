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

type fakeQuizService struct {
	quiz          quizdb.DailyQuiz
	loadErr       error
	locale        string
	attempt       quizdb.Attempt
	startErr      error
	startDeviceID string
}

func (reader *fakeQuizService) LoadDailyQuiz(_ context.Context, _ time.Time, locale string) (quizdb.DailyQuiz, error) {
	reader.locale = locale
	return reader.quiz, reader.loadErr
}

func (reader *fakeQuizService) StartAttempt(_ context.Context, _ string, deviceID string) (quizdb.Attempt, error) {
	reader.startDeviceID = deviceID
	return reader.attempt, reader.startErr
}
