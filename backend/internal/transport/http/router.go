package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"discerne/backend/internal/config"
	"discerne/backend/internal/quizdb"
)

type DailyQuizReader interface {
	LoadDailyQuiz(rctx context.Context, quizDate time.Time, locale string) (quizdb.DailyQuiz, error)
}

type DailyQuizAttemptLoader interface {
	LoadDailyQuizAttempt(rctx context.Context, quizDate time.Time, deviceID string) (quizdb.AttemptResult, error)
}

type AttemptStarter interface {
	StartAttempt(rctx context.Context, quizDate string, deviceID string) (quizdb.Attempt, error)
}

type AnswerSubmitter interface {
	SubmitAnswer(rctx context.Context, input quizdb.SubmitAnswerInput) (quizdb.AnswerSubmission, error)
}

type AttemptLoader interface {
	LoadAttempt(rctx context.Context, attemptID string, deviceID string) (quizdb.AttemptResult, error)
}

type ReadinessLoader interface {
	LoadReadiness(rctx context.Context, today time.Time) (quizdb.ReadinessStatus, error)
}

type QuizService interface {
	DailyQuizReader
	DailyQuizAttemptLoader
	AttemptStarter
	AnswerSubmitter
	AttemptLoader
	ReadinessLoader
}

// NewRouter wires the API routes.
func NewRouter(cfg config.Config, logger *slog.Logger, quizzes QuizService) http.Handler {
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

	mux.HandleFunc("GET /api/v1/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if quizzes == nil {
			if err := respondJSON(w, http.StatusServiceUnavailable, readinessResponse{
				Status:          "unavailable",
				Database:        "unavailable",
				TodayQuiz:       "unknown",
				FutureQuizCount: 0,
			}); err != nil {
				logger.Error("write readiness response", "error", err)
			}
			return
		}

		today := time.Now().In(cfg.AppTimezone)
		status, err := quizzes.LoadReadiness(r.Context(), today)
		if err != nil {
			logger.Error("load readiness", "error", err)
			if err := respondJSON(w, http.StatusServiceUnavailable, readinessResponse{
				Status:          "unavailable",
				Database:        "unavailable",
				TodayQuiz:       "unknown",
				FutureQuizCount: 0,
			}); err != nil {
				logger.Error("write readiness response", "error", err)
			}
			return
		}

		response := readinessResponseFromStatus(status)
		httpStatus := http.StatusOK
		if response.Status != "ok" {
			httpStatus = http.StatusServiceUnavailable
		}
		if err := respondJSON(w, httpStatus, response); err != nil {
			logger.Error("write readiness response", "error", err)
		}
	})

	mux.HandleFunc("GET /api/v1/quizzes/today", func(w http.ResponseWriter, r *http.Request) {
		if quizzes == nil {
			respondError(w, http.StatusServiceUnavailable, "database_unavailable")
			return
		}

		locale, ok := supportedLocale(r.URL.Query().Get("locale"))
		if !ok {
			respondError(w, http.StatusBadRequest, "unsupported_locale")
			return
		}

		quizDate := time.Now().In(cfg.AppTimezone)
		dailyQuiz, err := quizzes.LoadDailyQuiz(r.Context(), quizDate, locale)
		if errors.Is(err, quizdb.ErrDailyQuizNotFound) {
			respondError(w, http.StatusNotFound, "quiz_not_found")
			return
		}
		if err != nil {
			logger.Error("load today quiz", "error", err)
			respondError(w, http.StatusInternalServerError, "internal_error")
			return
		}

		attempt := quizdb.AttemptResult{Status: "not_started"}
		if deviceID := deviceIDFromCookie(r, cfg.DeviceCookieName); deviceID != "" {
			loadedAttempt, err := quizzes.LoadDailyQuizAttempt(r.Context(), quizDate, deviceID)
			if err != nil && !errors.Is(err, quizdb.ErrAttemptNotFound) {
				logger.Error("load today quiz attempt", "error", err)
				respondError(w, http.StatusInternalServerError, "internal_error")
				return
			}
			if err == nil {
				attempt = loadedAttempt
			}
		}

		if err := respondJSON(w, http.StatusOK, todayQuizResponseFromDailyQuiz(dailyQuiz, attempt)); err != nil {
			logger.Error("write today quiz response", "error", err)
		}
	})

	mux.HandleFunc("POST /api/v1/quizzes/today/attempt", func(w http.ResponseWriter, r *http.Request) {
		if quizzes == nil {
			respondError(w, http.StatusServiceUnavailable, "database_unavailable")
			return
		}

		quizDate := time.Now().In(cfg.AppTimezone).Format("2006-01-02")
		attempt, err := quizzes.StartAttempt(r.Context(), quizDate, deviceIDFromCookie(r, cfg.DeviceCookieName))
		if errors.Is(err, quizdb.ErrDailyQuizNotFound) {
			respondError(w, http.StatusNotFound, "quiz_not_found")
			return
		}
		if errors.Is(err, quizdb.ErrAttemptAlreadyCompleted) {
			respondError(w, http.StatusConflict, "attempt_already_completed")
			return
		}
		if err != nil {
			logger.Error("start quiz attempt", "error", err)
			respondError(w, http.StatusInternalServerError, "internal_error")
			return
		}

		setDeviceCookie(w, cfg, attempt.DeviceID)
		if err := respondJSON(w, http.StatusCreated, startAttemptResponse{
			AttemptID: attempt.ID,
			Status:    attempt.Status,
		}); err != nil {
			logger.Error("write start attempt response", "error", err)
		}
	})

	mux.HandleFunc("POST /api/v1/attempts/{attemptId}/answers", func(w http.ResponseWriter, r *http.Request) {
		if quizzes == nil {
			respondError(w, http.StatusServiceUnavailable, "database_unavailable")
			return
		}

		var request submitAnswerRequest
		if err := decodeJSONRequest(w, r, &request); err != nil {
			respondError(w, http.StatusBadRequest, "invalid_request")
			return
		}

		answer, err := quizzes.SubmitAnswer(r.Context(), quizdb.SubmitAnswerInput{
			AttemptID:          r.PathValue("attemptId"),
			DeviceID:           deviceIDFromCookie(r, cfg.DeviceCookieName),
			QuestionID:         request.QuestionID,
			SelectedLanguageID: request.SelectedLanguageID,
			ResponseTimeMS:     request.ResponseTimeMS,
		})
		if errors.Is(err, quizdb.ErrAttemptNotFound) {
			respondError(w, http.StatusNotFound, "attempt_not_found")
			return
		}
		if errors.Is(err, quizdb.ErrInvalidAnswer) {
			respondError(w, http.StatusBadRequest, "invalid_answer")
			return
		}
		if errors.Is(err, quizdb.ErrAnswerAlreadySubmitted) {
			respondError(w, http.StatusConflict, "answer_already_submitted")
			return
		}
		if err != nil {
			logger.Error("submit quiz answer", "error", err)
			respondError(w, http.StatusInternalServerError, "internal_error")
			return
		}

		if err := respondJSON(w, http.StatusCreated, submitAnswerResponse{
			QuestionID:         answer.QuestionID,
			SelectedLanguageID: answer.SelectedLanguageID,
			CorrectLanguageID:  answer.CorrectLanguageID,
			IsCorrect:          answer.IsCorrect,
		}); err != nil {
			logger.Error("write submit answer response", "error", err)
		}
	})

	mux.HandleFunc("GET /api/v1/attempts/{attemptId}", func(w http.ResponseWriter, r *http.Request) {
		if quizzes == nil {
			respondError(w, http.StatusServiceUnavailable, "database_unavailable")
			return
		}

		attempt, err := quizzes.LoadAttempt(
			r.Context(),
			r.PathValue("attemptId"),
			deviceIDFromCookie(r, cfg.DeviceCookieName),
		)
		if errors.Is(err, quizdb.ErrAttemptNotFound) {
			respondError(w, http.StatusNotFound, "attempt_not_found")
			return
		}
		if err != nil {
			logger.Error("load quiz attempt", "error", err)
			respondError(w, http.StatusInternalServerError, "internal_error")
			return
		}

		if err := respondJSON(w, http.StatusOK, attemptResultResponseFromAttempt(attempt)); err != nil {
			logger.Error("write attempt response", "error", err)
		}
	})

	rateLimiter := newMutationRateLimiter(cfg)
	return requestLogger(logger, rateLimiter.middleware(cfg, mux))
}

type todayQuizResponse struct {
	QuizDate  string              `json:"quizDate"`
	Attempt   todayQuizAttempt    `json:"attempt"`
	Questions []todayQuizQuestion `json:"questions"`
}

type todayQuizAttempt struct {
	AttemptID     string                   `json:"attemptId,omitempty"`
	Status        string                   `json:"status"`
	AnsweredCount int                      `json:"answeredCount"`
	QuestionCount int                      `json:"questionCount"`
	Score         *int                     `json:"score"`
	Answers       []todayQuizAttemptAnswer `json:"answers"`
}

type todayQuizAttemptAnswer struct {
	QuestionID         string `json:"questionId"`
	SelectedLanguageID string `json:"selectedLanguageId"`
	CorrectLanguageID  string `json:"correctLanguageId"`
	IsCorrect          bool   `json:"isCorrect"`
}

type todayQuizQuestion struct {
	ID       string            `json:"id"`
	Position int               `json:"position"`
	Text     string            `json:"text"`
	Options  []todayQuizOption `json:"options"`
}

type todayQuizOption struct {
	LanguageID string `json:"languageId"`
	Name       string `json:"name"`
}

type startAttemptResponse struct {
	AttemptID string `json:"attemptId"`
	Status    string `json:"status"`
}

type submitAnswerRequest struct {
	QuestionID         string `json:"questionId"`
	SelectedLanguageID string `json:"selectedLanguageId"`
	ResponseTimeMS     *int   `json:"responseTimeMs"`
}

type submitAnswerResponse struct {
	QuestionID         string `json:"questionId"`
	SelectedLanguageID string `json:"selectedLanguageId"`
	CorrectLanguageID  string `json:"correctLanguageId"`
	IsCorrect          bool   `json:"isCorrect"`
}

type attemptResultResponse struct {
	AttemptID     string                   `json:"attemptId"`
	Status        string                   `json:"status"`
	AnsweredCount int                      `json:"answeredCount"`
	QuestionCount int                      `json:"questionCount"`
	Score         *int                     `json:"score"`
	Answers       []todayQuizAttemptAnswer `json:"answers"`
}

type healthResponse struct {
	AppName  string `json:"appName"`
	Status   string `json:"status"`
	Timezone string `json:"timezone"`
	Now      string `json:"now"`
}

type readinessResponse struct {
	Status          string `json:"status"`
	Database        string `json:"database"`
	TodayQuiz       string `json:"todayQuiz"`
	FutureQuizCount int    `json:"futureQuizCount"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func todayQuizResponseFromDailyQuiz(dailyQuiz quizdb.DailyQuiz, attempt quizdb.AttemptResult) todayQuizResponse {
	response := todayQuizResponse{
		QuizDate:  dailyQuiz.QuizDate,
		Attempt:   todayQuizAttemptFromAttempt(attempt, len(dailyQuiz.Questions)),
		Questions: make([]todayQuizQuestion, 0, len(dailyQuiz.Questions)),
	}

	for _, question := range dailyQuiz.Questions {
		responseQuestion := todayQuizQuestion{
			ID:       question.ID,
			Position: question.Position,
			Text:     question.Text,
			Options:  make([]todayQuizOption, 0, len(question.Options)),
		}

		for _, option := range question.Options {
			responseQuestion.Options = append(responseQuestion.Options, todayQuizOption{
				LanguageID: option.LanguageID,
				Name:       option.Name,
			})
		}

		response.Questions = append(response.Questions, responseQuestion)
	}

	return response
}

func attemptResultResponseFromAttempt(attempt quizdb.AttemptResult) attemptResultResponse {
	return attemptResultResponse{
		AttemptID:     attempt.ID,
		Status:        attempt.Status,
		AnsweredCount: attempt.AnsweredCount,
		QuestionCount: attempt.QuestionCount,
		Score:         attempt.Score,
		Answers:       attemptAnswersResponse(attempt.Answers),
	}
}

func todayQuizAttemptFromAttempt(attempt quizdb.AttemptResult, questionCount int) todayQuizAttempt {
	status := attempt.Status
	if status == "" {
		status = "not_started"
	}

	if attempt.QuestionCount > 0 {
		questionCount = attempt.QuestionCount
	}

	return todayQuizAttempt{
		AttemptID:     attempt.ID,
		Status:        status,
		AnsweredCount: attempt.AnsweredCount,
		QuestionCount: questionCount,
		Score:         attempt.Score,
		Answers:       attemptAnswersResponse(attempt.Answers),
	}
}

func attemptAnswersResponse(answers []quizdb.AttemptAnswer) []todayQuizAttemptAnswer {
	response := make([]todayQuizAttemptAnswer, 0, len(answers))
	for _, answer := range answers {
		response = append(response, todayQuizAttemptAnswer{
			QuestionID:         answer.QuestionID,
			SelectedLanguageID: answer.SelectedLanguageID,
			CorrectLanguageID:  answer.CorrectLanguageID,
			IsCorrect:          answer.IsCorrect,
		})
	}

	return response
}

func readinessResponseFromStatus(status quizdb.ReadinessStatus) readinessResponse {
	response := readinessResponse{
		Status:          "ok",
		Database:        "ok",
		TodayQuiz:       "ok",
		FutureQuizCount: status.FutureQuizCount,
	}

	if !status.DatabaseOK {
		response.Status = "unavailable"
		response.Database = "unavailable"
	}
	if !status.TodayQuizOK {
		response.Status = "unavailable"
		response.TodayQuiz = "missing"
	}

	return response
}

func supportedLocale(rawLocale string) (string, bool) {
	if rawLocale == "" {
		return "en-US", true
	}

	switch rawLocale {
	case "pl-PL", "en-US", "es-ES":
		return rawLocale, true
	default:
		return "", false
	}
}

func deviceIDFromCookie(r *http.Request, cookieName string) string {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setDeviceCookie(w http.ResponseWriter, cfg config.Config, deviceID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.DeviceCookieName,
		Value:    deviceID,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		HttpOnly: true,
		Secure:   cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, body any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(body); err != nil {
		return err
	}

	var extra struct{}
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return err
	} else if err == nil {
		return errors.New("request body must contain a single JSON value")
	}

	return nil
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

func respondError(w http.ResponseWriter, status int, code string) {
	_ = respondJSON(w, status, errorResponse{Error: code})
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
