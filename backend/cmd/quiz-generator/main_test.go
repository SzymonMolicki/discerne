package main

import (
	"context"
	"testing"
	"time"
)

func TestGenerationPlanUsesManualDaysWhenEnsureFutureIsDisabled(t *testing.T) {
	location := mustLoadLocation(t)

	startDate, days, err := generationPlan(context.Background(), generationPlanInput{
		From:         "2026-08-01",
		Location:     location,
		Days:         7,
		EnsureFuture: 0,
		GenerateDays: 30,
	}, func(context.Context, time.Time) (int, error) {
		t.Fatal("future quiz counter should not be called")
		return 0, nil
	})

	if err != nil {
		t.Fatalf("generationPlan() error = %v", err)
	}
	if got := startDate.Format("2006-01-02"); got != "2026-08-01" {
		t.Fatalf("startDate = %q, want %q", got, "2026-08-01")
	}
	if days != 7 {
		t.Fatalf("days = %d, want %d", days, 7)
	}
}

func TestGenerationPlanGeneratesWhenNoFutureQuizzesExist(t *testing.T) {
	startDate, days := ensureFuturePlan(t, 0)

	if got := startDate.Format("2006-01-02"); got != "2026-08-01" {
		t.Fatalf("startDate = %q, want %q", got, "2026-08-01")
	}
	if days != 30 {
		t.Fatalf("days = %d, want %d", days, 30)
	}
}

func TestGenerationPlanGeneratesWhenFutureBufferIsBelowThreshold(t *testing.T) {
	_, days := ensureFuturePlan(t, 6)

	if days != 30 {
		t.Fatalf("days = %d, want %d", days, 30)
	}
}

func TestGenerationPlanSkipsWhenFutureBufferMeetsThreshold(t *testing.T) {
	_, days := ensureFuturePlan(t, 7)

	if days != 0 {
		t.Fatalf("days = %d, want %d", days, 0)
	}
}

func ensureFuturePlan(t *testing.T, futureQuizCount int) (time.Time, int) {
	t.Helper()

	location := mustLoadLocation(t)
	var countedFrom string
	startDate, days, err := generationPlan(context.Background(), generationPlanInput{
		From:         "2026-08-01",
		Location:     location,
		Days:         1,
		EnsureFuture: 7,
		GenerateDays: 30,
	}, func(_ context.Context, startDate time.Time) (int, error) {
		countedFrom = startDate.Format("2006-01-02")
		return futureQuizCount, nil
	})

	if err != nil {
		t.Fatalf("generationPlan() error = %v", err)
	}
	if countedFrom != "2026-08-01" {
		t.Fatalf("countedFrom = %q, want %q", countedFrom, "2026-08-01")
	}

	return startDate, days
}

func mustLoadLocation(t *testing.T) *time.Location {
	t.Helper()

	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}
	return location
}
