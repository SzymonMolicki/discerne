package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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
		connectionURL = databaseURLFromEnvironment()
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

func databaseURLFromEnvironment() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}

	for _, path := range []string{".env", "../.env"} {
		value, err := readEnvValue(path, "DATABASE_URL")
		if err == nil && value != "" {
			return value
		}
	}

	return ""
}

func readEnvValue(path string, name string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != name {
			continue
		}

		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		return value, nil
	}

	return "", scanner.Err()
}
