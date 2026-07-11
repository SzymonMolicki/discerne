package main

import (
	"flag"
	"fmt"
	"os"

	"discerne/backend/internal/seeddata"
)

func main() {
	dataDir := flag.String("data-dir", seeddata.DefaultDataDir(), "path to seed data directory")
	flag.Parse()

	report, err := seeddata.Validate(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "data validator failed: %v\n", err)
		os.Exit(1)
	}

	if len(report.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "seed data is invalid:")
		for _, validationError := range report.Errors {
			fmt.Fprintf(os.Stderr, "- %s\n", validationError)
		}
		os.Exit(1)
	}

	fmt.Printf("seed data valid: %d enabled languages, %d approved texts\n", report.EnabledLanguages, report.ApprovedTexts)
}
