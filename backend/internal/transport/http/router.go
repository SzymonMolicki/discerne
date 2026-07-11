package httptransport

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"discerne/backend/internal/config"
)

// NewRouter wires the API routes.
func NewRouter(cfg config.Config, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if err := respondJSON(w, http.StatusOK, healthResponse{
			AppName:  cfg.AppName,
			Status:   "ok",
			Timezone: cfg.AppTimezone.String(),
			Now:      time.Now().In(cfg.AppTimezone).Format(time.RFC3339),
		}); err != nil {
			logger.Error("write health response", "error", err)
		}
	})

	return requestLogger(logger, mux)
}

type healthResponse struct {
	AppName  string `json:"appName"`
	Status   string `json:"status"`
	Timezone string `json:"timezone"`
	Now      string `json:"now"`
}

func respondJSON(w http.ResponseWriter, status int, body any) error {
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(body); err != nil {
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write(buffer.Bytes())
	return err
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}
