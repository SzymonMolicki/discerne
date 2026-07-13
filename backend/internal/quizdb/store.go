package quizdb

import (
	"context"
	"errors"
	"fmt"

	"discerne/backend/internal/quiz"

	"github.com/jackc/pgx/v5"
)

// SaveDailyQuiz stores a generated quiz. It returns false when the date already exists.
func SaveDailyQuiz(ctx context.Context, tx pgx.Tx, quizDate string, generatedQuiz quiz.GeneratedQuiz) (bool, error) {
	var dailyQuizID string
	err := tx.QueryRow(
		ctx,
		`INSERT INTO daily_quizzes (quiz_date)
		 VALUES ($1)
		 ON CONFLICT (quiz_date) DO NOTHING
		 RETURNING id`,
		quizDate,
	).Scan(&dailyQuizID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("insert daily quiz for %s: %w", quizDate, err)
	}

	for _, question := range generatedQuiz.Questions {
		questionID, err := insertQuestion(ctx, tx, dailyQuizID, question)
		if err != nil {
			return false, err
		}

		for _, option := range question.Options {
			if err := insertOption(ctx, tx, questionID, option); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func insertQuestion(ctx context.Context, tx pgx.Tx, dailyQuizID string, question quiz.GeneratedQuestion) (string, error) {
	var questionID string
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO daily_quiz_questions (
		   daily_quiz_id,
		   position,
		   text_id,
		   correct_language_id
		 )
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		dailyQuizID,
		question.Position,
		question.Text.ID,
		question.CorrectLanguage.ID,
	).Scan(&questionID); err != nil {
		return "", fmt.Errorf("insert question %d: %w", question.Position, err)
	}

	return questionID, nil
}

func insertOption(ctx context.Context, tx pgx.Tx, questionID string, option quiz.GeneratedOption) error {
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO daily_quiz_options (
		   question_id,
		   language_id,
		   position,
		   is_correct,
		   weight_at_generation
		 )
		 VALUES ($1, $2, $3, $4, $5)`,
		questionID,
		option.Language.ID,
		option.Position,
		option.IsCorrect,
		option.WeightAtGeneration,
	); err != nil {
		return fmt.Errorf("insert option %d: %w", option.Position, err)
	}

	return nil
}
