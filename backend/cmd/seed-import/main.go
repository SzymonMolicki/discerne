package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"discerne/backend/internal/database"
	"discerne/backend/internal/seeddata"
	"discerne/backend/internal/seedimport"

	"github.com/jackc/pgx/v5"
)

func main() {
	dataDir := flag.String("data-dir", seeddata.DefaultDataDir(), "path to seed data directory")
	databaseURL := flag.String("database-url", "", "PostgreSQL connection URL")
	flag.Parse()

	connectionURL := *databaseURL
	if connectionURL == "" {
		connectionURL = database.URLFromEnvironment()
	}
	if connectionURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is empty; set it in the environment, .env, or pass -database-url")
		os.Exit(1)
	}

	catalog, err := seeddata.Load(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load seed data: %v\n", err)
		os.Exit(1)
	}

	report := seeddata.ValidateCatalog(catalog)
	if len(report.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "seed data is invalid:")
		for _, validationError := range report.Errors {
			fmt.Fprintf(os.Stderr, "- %s\n", validationError)
		}
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	tx, err := conn.Begin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "begin transaction: %v\n", err)
		os.Exit(1)
	}
	defer tx.Rollback(context.Background())

	if err := seedimport.Import(ctx, tx, catalog); err != nil {
		fmt.Fprintf(os.Stderr, "import seed data: %v\n", err)
		os.Exit(1)
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "commit transaction: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("seed data imported")
}
