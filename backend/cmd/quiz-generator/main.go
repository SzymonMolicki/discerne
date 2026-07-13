package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"discerne/backend/internal/config"
	"discerne/backend/internal/database"
	"discerne/backend/internal/quiz"
	"discerne/backend/internal/quizdb"

	"github.com/jackc/pgx/v5"
)

func main() {
	from := flag.String("from", "", "first quiz date in YYYY-MM-DD format")
	days := flag.Int("days", 1, "number of daily quizzes to generate")
	dryRun := flag.Bool("dry-run", false, "generate without writing to the database")
	seed := flag.Int64("seed", 1, "base deterministic random seed")
	databaseURL := flag.String("database-url", "", "PostgreSQL connection URL")
	flag.Parse()

	if *days <= 0 {
		fmt.Fprintln(os.Stderr, "days must be greater than zero")
		os.Exit(1)
	}

	cfg, err := config.Load(os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "load configuration: %v\n", err)
		os.Exit(1)
	}

	startDate, err := firstQuizDate(*from, cfg.AppTimezone)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse from date: %v\n", err)
		os.Exit(1)
	}

	connectionURL := *databaseURL
	if connectionURL == "" {
		connectionURL = database.URLFromEnvironment()
	}
	if connectionURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is empty; set it in the environment, .env, or pass -database-url")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	catalog, err := quizdb.LoadCatalog(ctx, conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load generation input: %v\n", err)
		os.Exit(1)
	}

	for offset := 0; offset < *days; offset++ {
		quizDate := startDate.AddDate(0, 0, offset)
		quizDateText := quizDate.Format("2006-01-02")

		generator := quiz.Generator{
			Weights: cfg.DistractorWeights,
			Random:  quiz.NewSeededRandomSource(seedForDate(*seed, quizDate)),
		}
		generatedQuiz, err := generator.Generate(catalog.Languages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate quiz for %s: %v\n", quizDateText, err)
			os.Exit(1)
		}

		if *dryRun {
			fmt.Printf("generated %s in dry run: %d questions\n", quizDateText, len(generatedQuiz.Questions))
			continue
		}

		saved, err := saveQuiz(ctx, conn, quizDateText, generatedQuiz)
		if err != nil {
			fmt.Fprintf(os.Stderr, "save quiz for %s: %v\n", quizDateText, err)
			os.Exit(1)
		}
		if !saved {
			fmt.Printf("skipped %s: quiz already exists\n", quizDateText)
			continue
		}

		fmt.Printf("generated %s: %d questions\n", quizDateText, len(generatedQuiz.Questions))
	}
}

func firstQuizDate(rawDate string, location *time.Location) (time.Time, error) {
	if rawDate != "" {
		return time.ParseInLocation("2006-01-02", rawDate, location)
	}

	now := time.Now().In(location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location), nil
}

func seedForDate(baseSeed int64, quizDate time.Time) int64 {
	year, month, day := quizDate.Date()
	return baseSeed + int64(year*10000+int(month)*100+day)
}

func saveQuiz(ctx context.Context, conn *pgx.Conn, quizDate string, generatedQuiz quiz.GeneratedQuiz) (bool, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(context.Background())

	saved, err := quizdb.SaveDailyQuiz(ctx, tx, quizDate, generatedQuiz)
	if err != nil {
		return false, err
	}
	if !saved {
		return false, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}

	return true, nil
}
