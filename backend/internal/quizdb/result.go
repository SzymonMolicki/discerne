package quizdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// AttemptResult is the current server-side state of a quiz attempt.
type AttemptResult struct {
	ID            string
	Status        string
	AnsweredCount int
	QuestionCount int
	Score         *int
	Answers       []AttemptAnswer
}

// AttemptAnswer is one already submitted answer in an attempt.
type AttemptAnswer struct {
	QuestionID         string
	SelectedLanguageID string
	CorrectLanguageID  string
	IsCorrect          bool
}

// LoadAttempt reads an attempt owned by an anonymous device.
func (store Store) LoadAttempt(ctx context.Context, attemptID string, deviceID string) (AttemptResult, error) {
	return loadAttemptResult(ctx, store.db, attemptID, deviceID)
}

// LoadDailyQuizAttempt reads the current device attempt for a quiz date.
func (store Store) LoadDailyQuizAttempt(ctx context.Context, quizDate time.Time, deviceID string) (AttemptResult, error) {
	if !isUUID(deviceID) {
		return AttemptResult{}, ErrAttemptNotFound
	}

	var attemptID string
	err := store.db.QueryRow(
		ctx,
		`SELECT quiz_attempts.id::text
		 FROM quiz_attempts
		 JOIN daily_quizzes ON daily_quizzes.id = quiz_attempts.daily_quiz_id
		 WHERE quiz_attempts.device_id = $1::uuid
		   AND daily_quizzes.quiz_date = $2::date
		 ORDER BY quiz_attempts.completed_at IS NULL, quiz_attempts.started_at DESC
		 LIMIT 1`,
		deviceID,
		quizDate.Format("2006-01-02"),
	).Scan(&attemptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AttemptResult{}, ErrAttemptNotFound
	}
	if err != nil {
		return AttemptResult{}, fmt.Errorf("load daily quiz attempt: %w", err)
	}

	return loadAttemptResult(ctx, store.db, attemptID, deviceID)
}

func loadAttemptResult(ctx context.Context, db queryer, attemptID string, deviceID string) (AttemptResult, error) {
	if !isUUID(attemptID) || !isUUID(deviceID) {
		return AttemptResult{}, ErrAttemptNotFound
	}

	var result AttemptResult
	var score sql.NullInt64
	var correctCount int
	err := db.QueryRow(
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

	result.Answers, err = loadAttemptAnswers(ctx, db, attemptID)
	if err != nil {
		return AttemptResult{}, err
	}

	return result, nil
}

func loadAttemptAnswers(ctx context.Context, db queryer, attemptID string) ([]AttemptAnswer, error) {
	rows, err := db.Query(
		ctx,
		`SELECT
		   quiz_answers.question_id::text,
		   quiz_answers.selected_language_id::text,
		   correct_option.language_id::text,
		   quiz_answers.is_correct
		 FROM quiz_answers
		 JOIN daily_quiz_questions ON daily_quiz_questions.id = quiz_answers.question_id
		 JOIN daily_quiz_options correct_option
		   ON correct_option.question_id = quiz_answers.question_id
		  AND correct_option.is_correct
		 WHERE quiz_answers.attempt_id = $1::uuid
		 ORDER BY daily_quiz_questions.position`,
		attemptID,
	)
	if err != nil {
		return nil, fmt.Errorf("query attempt answers: %w", err)
	}
	defer rows.Close()

	var answers []AttemptAnswer
	for rows.Next() {
		var answer AttemptAnswer
		if err := rows.Scan(
			&answer.QuestionID,
			&answer.SelectedLanguageID,
			&answer.CorrectLanguageID,
			&answer.IsCorrect,
		); err != nil {
			return nil, fmt.Errorf("scan attempt answer: %w", err)
		}
		answers = append(answers, answer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read attempt answers: %w", err)
	}

	return answers, nil
}
