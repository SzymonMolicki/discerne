package quizdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrAttemptNotFound means the requested active attempt does not exist.
	ErrAttemptNotFound = errors.New("attempt not found")

	// ErrInvalidAnswer means the question or selected language is not valid for the attempt.
	ErrInvalidAnswer = errors.New("invalid answer")

	// ErrAnswerAlreadySubmitted means the attempt already has an answer for the question.
	ErrAnswerAlreadySubmitted = errors.New("answer already submitted")
)

// SubmitAnswerInput contains the answer submitted by the player.
type SubmitAnswerInput struct {
	AttemptID          string
	DeviceID           string
	QuestionID         string
	SelectedLanguageID string
	ResponseTimeMS     *int
}

// AnswerSubmission is the saved result for one submitted answer.
type AnswerSubmission struct {
	ID                 string
	QuestionID         string
	SelectedLanguageID string
	CorrectLanguageID  string
	IsCorrect          bool
}

// SubmitAnswer stores one answer and calculates correctness on the server.
func (store Store) SubmitAnswer(ctx context.Context, input SubmitAnswerInput) (AnswerSubmission, error) {
	if err := validateSubmitAnswerInput(input); err != nil {
		return AnswerSubmission{}, err
	}

	tx, err := store.db.Begin(ctx)
	if err != nil {
		return AnswerSubmission{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	dailyQuizID, err := activeAttemptDailyQuizID(ctx, tx, input.AttemptID, input.DeviceID)
	if err != nil {
		return AnswerSubmission{}, err
	}

	answerCorrectness, err := submittedOptionCorrectness(ctx, tx, dailyQuizID, input.QuestionID, input.SelectedLanguageID)
	if err != nil {
		return AnswerSubmission{}, err
	}

	answerID, err := insertAnswer(ctx, tx, input, answerCorrectness.IsCorrect)
	if err != nil {
		return AnswerSubmission{}, err
	}

	progress, err := loadAttemptProgress(ctx, tx, input.AttemptID, dailyQuizID)
	if err != nil {
		return AnswerSubmission{}, err
	}
	if progress.QuestionCount > 0 && progress.AnsweredCount >= progress.QuestionCount {
		if err := completeAttempt(ctx, tx, input.AttemptID, progress.Score); err != nil {
			return AnswerSubmission{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return AnswerSubmission{}, fmt.Errorf("commit transaction: %w", err)
	}

	return AnswerSubmission{
		ID:                 answerID,
		QuestionID:         input.QuestionID,
		SelectedLanguageID: input.SelectedLanguageID,
		CorrectLanguageID:  answerCorrectness.CorrectLanguageID,
		IsCorrect:          answerCorrectness.IsCorrect,
	}, nil
}

func validateSubmitAnswerInput(input SubmitAnswerInput) error {
	if !isUUID(input.AttemptID) || !isUUID(input.QuestionID) || !isUUID(input.SelectedLanguageID) {
		return ErrInvalidAnswer
	}
	if !isUUID(input.DeviceID) {
		return ErrAttemptNotFound
	}
	if input.ResponseTimeMS != nil && *input.ResponseTimeMS < 0 {
		return ErrInvalidAnswer
	}
	return nil
}

func activeAttemptDailyQuizID(ctx context.Context, tx pgx.Tx, attemptID string, deviceID string) (string, error) {
	var dailyQuizID string
	err := tx.QueryRow(
		ctx,
		`SELECT daily_quiz_id::text
		 FROM quiz_attempts
		 WHERE id = $1::uuid
		   AND device_id = $2::uuid
		   AND completed_at IS NULL
		 FOR UPDATE`,
		attemptID,
		deviceID,
	).Scan(&dailyQuizID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAttemptNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select active quiz attempt: %w", err)
	}

	return dailyQuizID, nil
}

type submittedOptionResult struct {
	IsCorrect         bool
	CorrectLanguageID string
}

func submittedOptionCorrectness(
	ctx context.Context,
	tx pgx.Tx,
	dailyQuizID string,
	questionID string,
	selectedLanguageID string,
) (submittedOptionResult, error) {
	var result submittedOptionResult
	err := tx.QueryRow(
		ctx,
		`SELECT selected_option.is_correct, correct_option.language_id::text
		 FROM daily_quiz_questions
		 JOIN daily_quiz_options selected_option
		   ON selected_option.question_id = daily_quiz_questions.id
		  AND selected_option.language_id = $3::uuid
		 JOIN daily_quiz_options correct_option
		   ON correct_option.question_id = daily_quiz_questions.id
		  AND correct_option.is_correct
		 WHERE daily_quiz_questions.daily_quiz_id = $1::uuid
		   AND daily_quiz_questions.id = $2::uuid`,
		dailyQuizID,
		questionID,
		selectedLanguageID,
	).Scan(&result.IsCorrect, &result.CorrectLanguageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return submittedOptionResult{}, ErrInvalidAnswer
	}
	if err != nil {
		return submittedOptionResult{}, fmt.Errorf("select submitted option: %w", err)
	}

	return result, nil
}

func insertAnswer(ctx context.Context, tx pgx.Tx, input SubmitAnswerInput, isCorrect bool) (string, error) {
	var responseTimeMS any
	if input.ResponseTimeMS != nil {
		responseTimeMS = *input.ResponseTimeMS
	}

	var answerID string
	err := tx.QueryRow(
		ctx,
		`INSERT INTO quiz_answers (
		   attempt_id,
		   question_id,
		   selected_language_id,
		   is_correct,
		   response_time_ms
		 )
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)
		 ON CONFLICT (attempt_id, question_id) DO NOTHING
		 RETURNING id::text`,
		input.AttemptID,
		input.QuestionID,
		input.SelectedLanguageID,
		isCorrect,
		responseTimeMS,
	).Scan(&answerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAnswerAlreadySubmitted
	}
	if err != nil {
		return "", fmt.Errorf("insert quiz answer: %w", err)
	}

	return answerID, nil
}

type attemptProgress struct {
	QuestionCount int
	AnsweredCount int
	Score         int
}

func loadAttemptProgress(ctx context.Context, tx pgx.Tx, attemptID string, dailyQuizID string) (attemptProgress, error) {
	var progress attemptProgress
	if err := tx.QueryRow(
		ctx,
		`SELECT
		   count(daily_quiz_questions.id)::int,
		   count(quiz_answers.id)::int,
		   (count(quiz_answers.id) FILTER (WHERE quiz_answers.is_correct))::int
		 FROM daily_quiz_questions
		 LEFT JOIN quiz_answers
		   ON quiz_answers.question_id = daily_quiz_questions.id
		  AND quiz_answers.attempt_id = $1::uuid
		 WHERE daily_quiz_questions.daily_quiz_id = $2::uuid`,
		attemptID,
		dailyQuizID,
	).Scan(&progress.QuestionCount, &progress.AnsweredCount, &progress.Score); err != nil {
		return attemptProgress{}, fmt.Errorf("load attempt progress: %w", err)
	}

	return progress, nil
}

func completeAttempt(ctx context.Context, tx pgx.Tx, attemptID string, score int) error {
	if _, err := tx.Exec(
		ctx,
		`UPDATE quiz_attempts
		 SET completed_at = COALESCE(completed_at, now()),
		     score = $2
		 WHERE id = $1::uuid`,
		attemptID,
		score,
	); err != nil {
		return fmt.Errorf("complete quiz attempt: %w", err)
	}

	return nil
}
