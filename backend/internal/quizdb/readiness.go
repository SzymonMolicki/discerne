package quizdb

import (
	"context"
	"fmt"
	"time"
)

// ReadinessStatus describes whether the API is ready to serve quiz traffic.
type ReadinessStatus struct {
	DatabaseOK      bool
	TodayQuizOK     bool
	FutureQuizCount int
}

// LoadReadiness checks database connectivity and quiz availability.
func (store Store) LoadReadiness(ctx context.Context, today time.Time) (ReadinessStatus, error) {
	var probe int
	if err := store.db.QueryRow(ctx, `SELECT 1`).Scan(&probe); err != nil {
		return ReadinessStatus{}, fmt.Errorf("check database connectivity: %w", err)
	}

	todayText := today.Format("2006-01-02")
	tomorrowText := today.AddDate(0, 0, 1).Format("2006-01-02")

	var status ReadinessStatus
	status.DatabaseOK = probe == 1
	if err := store.db.QueryRow(
		ctx,
		`SELECT EXISTS (
		   SELECT 1
		   FROM daily_quizzes
		   WHERE quiz_date = $1::date
		 )`,
		todayText,
	).Scan(&status.TodayQuizOK); err != nil {
		return ReadinessStatus{}, fmt.Errorf("check today quiz: %w", err)
	}

	if err := store.db.QueryRow(
		ctx,
		`SELECT count(*)::int
		 FROM daily_quizzes
		 WHERE quiz_date >= $1::date`,
		tomorrowText,
	).Scan(&status.FutureQuizCount); err != nil {
		return ReadinessStatus{}, fmt.Errorf("count future quizzes: %w", err)
	}

	return status, nil
}
