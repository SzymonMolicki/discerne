package httptransport

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"discerne/backend/internal/config"
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
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

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
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
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
