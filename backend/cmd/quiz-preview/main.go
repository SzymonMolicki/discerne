package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"discerne/backend/internal/config"
	"discerne/backend/internal/database"
	"discerne/backend/internal/quiz"
	"discerne/backend/internal/quizdb"
	"discerne/backend/internal/quizseed"
	"discerne/backend/internal/seeddata"

	"github.com/jackc/pgx/v5"
)

type previewQuiz struct {
	Questions []previewQuestion `json:"questions"`
}

type previewQuestion struct {
	Position        int             `json:"position"`
	TextID          string          `json:"textId"`
	Text            string          `json:"text"`
	CorrectLanguage previewLanguage `json:"correctLanguage"`
	Options         []previewOption `json:"options"`
}

type previewOption struct {
	Position           int             `json:"position"`
	Language           previewLanguage `json:"language"`
	IsCorrect          bool            `json:"isCorrect"`
	WeightAtGeneration int             `json:"weightAtGeneration"`
}

type previewLanguage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	dataDir := flag.String("data-dir", seeddata.DefaultDataDir(), "path to seed data directory")
	source := flag.String("source", "seed", "quiz data source: seed or database")
	databaseURL := flag.String("database-url", "", "PostgreSQL connection URL")
	locale := flag.String("locale", "en-US", "locale used for language names")
	seed := flag.Int64("seed", 1, "deterministic random seed")
	flag.Parse()

	languages, languageNames, err := loadGenerationInput(*source, *dataDir, *databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load generation input: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "load configuration: %v\n", err)
		os.Exit(1)
	}

	generator := quiz.Generator{
		Weights: cfg.DistractorWeights,
		Random:  quiz.NewSeededRandomSource(*seed),
	}
	generatedQuiz, err := generator.Generate(languages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate quiz preview: %v\n", err)
		os.Exit(1)
	}

	output, err := buildPreview(generatedQuiz, languageNames, *locale)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build quiz preview: %v\n", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "write quiz preview: %v\n", err)
		os.Exit(1)
	}
}

func loadGenerationInput(source string, dataDir string, databaseURL string) ([]quiz.GenerationLanguage, map[string]map[string]string, error) {
	switch source {
	case "seed":
		return loadSeedGenerationInput(dataDir)
	case "database":
		return loadDatabaseGenerationInput(databaseURL)
	default:
		return nil, nil, fmt.Errorf("unknown source %q", source)
	}
}

func loadSeedGenerationInput(dataDir string) ([]quiz.GenerationLanguage, map[string]map[string]string, error) {
	catalog, err := seeddata.Load(dataDir)
	if err != nil {
		return nil, nil, fmt.Errorf("load seed data: %w", err)
	}

	report := seeddata.ValidateCatalog(catalog)
	if len(report.Errors) > 0 {
		return nil, nil, fmt.Errorf("seed data is invalid: %s", report.Errors[0])
	}

	languages, err := quizseed.GenerationLanguages(catalog)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare quiz languages: %w", err)
	}

	return languages, catalog.LanguageNames, nil
}

func loadDatabaseGenerationInput(databaseURL string) ([]quiz.GenerationLanguage, map[string]map[string]string, error) {
	connectionURL := databaseURL
	if connectionURL == "" {
		connectionURL = database.URLFromEnvironment()
	}
	if connectionURL == "" {
		return nil, nil, fmt.Errorf("DATABASE_URL is empty; set it in the environment, .env, or pass -database-url")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connectionURL)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to database: %w", err)
	}
	defer conn.Close(context.Background())

	catalog, err := quizdb.LoadCatalog(ctx, conn)
	if err != nil {
		return nil, nil, err
	}

	return catalog.Languages, catalog.LanguageNames, nil
}

func buildPreview(generatedQuiz quiz.GeneratedQuiz, languageNames map[string]map[string]string, locale string) (previewQuiz, error) {
	output := previewQuiz{
		Questions: make([]previewQuestion, 0, len(generatedQuiz.Questions)),
	}

	for _, question := range generatedQuiz.Questions {
		correctLanguage, err := previewLanguageFor(languageNames, question.CorrectLanguage.ID, locale)
		if err != nil {
			return previewQuiz{}, err
		}

		previewQuestion := previewQuestion{
			Position:        question.Position,
			TextID:          question.Text.ID,
			Text:            question.Text.Content,
			CorrectLanguage: correctLanguage,
			Options:         make([]previewOption, 0, len(question.Options)),
		}

		for _, option := range question.Options {
			language, err := previewLanguageFor(languageNames, option.Language.ID, locale)
			if err != nil {
				return previewQuiz{}, err
			}

			previewQuestion.Options = append(previewQuestion.Options, previewOption{
				Position:           option.Position,
				Language:           language,
				IsCorrect:          option.IsCorrect,
				WeightAtGeneration: option.WeightAtGeneration,
			})
		}

		output.Questions = append(output.Questions, previewQuestion)
	}

	return output, nil
}

func previewLanguageFor(languageNames map[string]map[string]string, languageID string, locale string) (previewLanguage, error) {
	names, ok := languageNames[languageID]
	if !ok {
		return previewLanguage{}, fmt.Errorf("missing language names for %q", languageID)
	}

	name := names[locale]
	if name == "" {
		return previewLanguage{}, fmt.Errorf("missing %s name for language %q", locale, languageID)
	}

	return previewLanguage{
		ID:   languageID,
		Name: name,
	}, nil
}
