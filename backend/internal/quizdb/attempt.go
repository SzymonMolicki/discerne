package quizdb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Attempt is a quiz attempt started by an anonymous device.
type Attempt struct {
	ID       string
	DeviceID string
	Status   string
}

// StartAttempt creates an anonymous device when needed and starts a daily quiz attempt.
func (store Store) StartAttempt(ctx context.Context, quizDate string, deviceID string) (Attempt, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return Attempt{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	resolvedDeviceID, err := ensureDevice(ctx, tx, deviceID)
	if err != nil {
		return Attempt{}, err
	}

	dailyQuizID, err := dailyQuizIDForDate(ctx, tx, quizDate)
	if err != nil {
		return Attempt{}, err
	}

	attemptID, err := insertAttempt(ctx, tx, resolvedDeviceID, dailyQuizID)
	if err != nil {
		return Attempt{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Attempt{}, fmt.Errorf("commit transaction: %w", err)
	}

	return Attempt{
		ID:       attemptID,
		DeviceID: resolvedDeviceID,
		Status:   "in_progress",
	}, nil
}

func ensureDevice(ctx context.Context, tx pgx.Tx, deviceID string) (string, error) {
	if isUUID(deviceID) {
		var resolvedDeviceID string
		err := tx.QueryRow(
			ctx,
			`UPDATE anonymous_devices
			 SET last_seen_at = now()
			 WHERE id = $1::uuid
			 RETURNING id::text`,
			deviceID,
		).Scan(&resolvedDeviceID)
		if err == nil {
			return resolvedDeviceID, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("update anonymous device: %w", err)
		}
	}

	var newDeviceID string
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO anonymous_devices DEFAULT VALUES
		 RETURNING id::text`,
	).Scan(&newDeviceID); err != nil {
		return "", fmt.Errorf("insert anonymous device: %w", err)
	}

	return newDeviceID, nil
}

func dailyQuizIDForDate(ctx context.Context, tx pgx.Tx, quizDate string) (string, error) {
	var dailyQuizID string
	err := tx.QueryRow(
		ctx,
		`SELECT id::text
		 FROM daily_quizzes
		 WHERE quiz_date = $1::date`,
		quizDate,
	).Scan(&dailyQuizID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrDailyQuizNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select daily quiz for %s: %w", quizDate, err)
	}

	return dailyQuizID, nil
}

func insertAttempt(ctx context.Context, tx pgx.Tx, deviceID string, dailyQuizID string) (string, error) {
	var completedAttemptExists bool
	if err := tx.QueryRow(
		ctx,
		`SELECT EXISTS (
		   SELECT 1
		   FROM quiz_attempts
		   WHERE device_id = $1::uuid
		     AND daily_quiz_id = $2::uuid
		     AND completed_at IS NOT NULL
		 )`,
		deviceID,
		dailyQuizID,
	).Scan(&completedAttemptExists); err != nil {
		return "", fmt.Errorf("select completed quiz attempt: %w", err)
	}
	if completedAttemptExists {
		return "", ErrAttemptAlreadyCompleted
	}

	var existingAttemptID string
	err := tx.QueryRow(
		ctx,
		`SELECT id::text
		 FROM quiz_attempts
		 WHERE device_id = $1::uuid
		   AND daily_quiz_id = $2::uuid
		   AND completed_at IS NULL
		 ORDER BY started_at DESC
		 LIMIT 1`,
		deviceID,
		dailyQuizID,
	).Scan(&existingAttemptID)
	if err == nil {
		return existingAttemptID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("select active quiz attempt: %w", err)
	}

	var attemptID string
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO quiz_attempts (device_id, daily_quiz_id)
		 VALUES ($1::uuid, $2::uuid)
		 RETURNING id::text`,
		deviceID,
		dailyQuizID,
	).Scan(&attemptID); err != nil {
		return "", fmt.Errorf("insert quiz attempt: %w", err)
	}

	return attemptID, nil
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}

	for index, char := range strings.ToLower(value) {
		switch index {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
				return false
			}
		}
	}

	return true
}
