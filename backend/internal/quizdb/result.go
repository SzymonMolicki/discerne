package quizdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AttemptResult is the current server-side state of a quiz attempt.
type AttemptResult struct {
	ID            string
	Status        string
	AnsweredCount int
	QuestionCount int
	Score         *int
}

// LoadAttempt reads an attempt owned by an anonymous device.
func (store Store) LoadAttempt(ctx context.Context, attemptID string, deviceID string) (AttemptResult, error) {
	if !isUUID(attemptID) || !isUUID(deviceID) {
		return AttemptResult{}, ErrAttemptNotFound
	}

	var result AttemptResult
	var score sql.NullInt64
	var correctCount int
	err := store.db.QueryRow(
		ctx,
		`SELECT
		   quiz_attempts.id::text,
		   CASE
		     WHEN quiz_attempts.completed_at IS NULL THEN 'in_progress'
		     ELSE 'completed'
		   END,
		   count(daily_quiz_questions.id)::int,
		   count(quiz_answers.id)::int,
		   quiz_attempts.score,
		   (count(quiz_answers.id) FILTER (WHERE quiz_answers.is_correct))::int
		 FROM quiz_attempts
		 JOIN daily_quiz_questions
		   ON daily_quiz_questions.daily_quiz_id = quiz_attempts.daily_quiz_id
		 LEFT JOIN quiz_answers
		   ON quiz_answers.question_id = daily_quiz_questions.id
		  AND quiz_answers.attempt_id = quiz_attempts.id
		 WHERE quiz_attempts.id = $1::uuid
		   AND quiz_attempts.device_id = $2::uuid
		 GROUP BY quiz_attempts.id, quiz_attempts.completed_at, quiz_attempts.score`,
		attemptID,
		deviceID,
	).Scan(
		&result.ID,
		&result.Status,
		&result.QuestionCount,
		&result.AnsweredCount,
		&score,
		&correctCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AttemptResult{}, ErrAttemptNotFound
	}
	if err != nil {
		return AttemptResult{}, fmt.Errorf("load quiz attempt: %w", err)
	}

	if result.Status == "completed" {
		value := correctCount
		if score.Valid {
			value = int(score.Int64)
		}
		result.Score = &value
	}

	return result, nil
}
