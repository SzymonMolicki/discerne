package quizdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"discerne/backend/internal/quiz"
	"discerne/backend/internal/seeddata"

	"github.com/jackc/pgx/v5"
)

// ErrDailyQuizNotFound means there is no generated quiz for the requested date.
var ErrDailyQuizNotFound = errors.New("daily quiz not found")

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type storeDB interface {
	queryer
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Catalog contains database data prepared for quiz generation.
type Catalog struct {
	Languages     []quiz.GenerationLanguage
	LanguageNames map[string]map[string]string
}

// Store reads quiz data from PostgreSQL.
type Store struct {
	db storeDB
}

// DailyQuiz is a generated quiz ready to be shown to a player.
type DailyQuiz struct {
	QuizDate  string
	Questions []DailyQuizQuestion
}

// DailyQuizQuestion is one prompt in a daily quiz.
type DailyQuizQuestion struct {
	ID       string
	Position int
	Text     string
	Options  []DailyQuizOption
}

// DailyQuizOption is one localized answer option in a daily quiz.
type DailyQuizOption struct {
	LanguageID string
	Position   int
	Name       string
}

// NewStore creates a quiz store.
func NewStore(db storeDB) Store {
	return Store{db: db}
}

// LoadCatalog reads quiz generation inputs from PostgreSQL.
func LoadCatalog(ctx context.Context, db queryer) (Catalog, error) {
	names, err := loadLanguageNames(ctx, db)
	if err != nil {
		return Catalog{}, err
	}

	languages, err := loadLanguages(ctx, db, names)
	if err != nil {
		return Catalog{}, err
	}

	if err := loadTexts(ctx, db, languages); err != nil {
		return Catalog{}, err
	}

	return Catalog{
		Languages:     languages,
		LanguageNames: names,
	}, nil
}

// LoadDailyQuiz reads a generated daily quiz without exposing correct answers.
func (store Store) LoadDailyQuiz(ctx context.Context, quizDate time.Time, locale string) (DailyQuiz, error) {
	rows, err := store.db.Query(
		ctx,
		`SELECT
		   daily_quizzes.quiz_date::text,
		   daily_quiz_questions.id::text,
		   daily_quiz_questions.position,
		   language_texts.content,
		   daily_quiz_options.language_id::text,
		   daily_quiz_options.position,
		   language_names.name
		 FROM daily_quizzes
		 JOIN daily_quiz_questions ON daily_quiz_questions.daily_quiz_id = daily_quizzes.id
		 JOIN language_texts ON language_texts.id = daily_quiz_questions.text_id
		 JOIN daily_quiz_options ON daily_quiz_options.question_id = daily_quiz_questions.id
		 JOIN language_names ON language_names.language_id = daily_quiz_options.language_id
		 WHERE daily_quizzes.quiz_date = $1::date
		   AND language_names.locale = $2
		 ORDER BY daily_quiz_questions.position, daily_quiz_options.position`,
		quizDate.Format("2006-01-02"),
		locale,
	)
	if err != nil {
		return DailyQuiz{}, fmt.Errorf("query daily quiz: %w", err)
	}
	defer rows.Close()

	var dailyQuiz DailyQuiz
	questionIndexes := make(map[string]int)

	for rows.Next() {
		var question DailyQuizQuestion
		var option DailyQuizOption
		if err := rows.Scan(
			&dailyQuiz.QuizDate,
			&question.ID,
			&question.Position,
			&question.Text,
			&option.LanguageID,
			&option.Position,
			&option.Name,
		); err != nil {
			return DailyQuiz{}, fmt.Errorf("scan daily quiz row: %w", err)
		}

		questionIndex, exists := questionIndexes[question.ID]
		if !exists {
			question.Options = make([]DailyQuizOption, 0, quiz.DefaultOptionCount)
			dailyQuiz.Questions = append(dailyQuiz.Questions, question)
			questionIndex = len(dailyQuiz.Questions) - 1
			questionIndexes[question.ID] = questionIndex
		}

		dailyQuiz.Questions[questionIndex].Options = append(dailyQuiz.Questions[questionIndex].Options, option)
	}
	if err := rows.Err(); err != nil {
		return DailyQuiz{}, fmt.Errorf("read daily quiz: %w", err)
	}
	if len(dailyQuiz.Questions) == 0 {
		return DailyQuiz{}, ErrDailyQuizNotFound
	}

	return dailyQuiz, nil
}

func loadLanguages(
	ctx context.Context,
	db queryer,
	names map[string]map[string]string,
) ([]quiz.GenerationLanguage, error) {
	rows, err := db.Query(
		ctx,
		`SELECT
		   languages.id::text,
		   language_families.code,
		   language_groups.code,
		   language_subgroups.code,
		   continents.code,
		   scripts.iso_15924_code,
		   languages.enabled
		 FROM languages
		 JOIN language_families ON language_families.id = languages.family_id
		 JOIN language_groups ON language_groups.id = languages.group_id
		 JOIN language_subgroups ON language_subgroups.id = languages.subgroup_id
		 JOIN continents ON continents.id = languages.continent_id
		 JOIN scripts ON scripts.id = languages.script_id
		 ORDER BY languages.iso_639_3`,
	)
	if err != nil {
		return nil, fmt.Errorf("query languages: %w", err)
	}
	defer rows.Close()

	var languages []quiz.GenerationLanguage
	for rows.Next() {
		var language quiz.GenerationLanguage
		if err := rows.Scan(
			&language.ID,
			&language.Metadata.Family,
			&language.Metadata.Group,
			&language.Metadata.Subgroup,
			&language.Metadata.Continent,
			&language.Metadata.Script,
			&language.Enabled,
		); err != nil {
			return nil, fmt.Errorf("scan language: %w", err)
		}
		language.HasRequiredNames = hasRequiredNames(names[language.ID])
		languages = append(languages, language)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read languages: %w", err)
	}

	return languages, nil
}

func loadLanguageNames(ctx context.Context, db queryer) (map[string]map[string]string, error) {
	rows, err := db.Query(
		ctx,
		`SELECT languages.id::text, language_names.locale, language_names.name
		 FROM language_names
		 JOIN languages ON languages.id = language_names.language_id
		 ORDER BY languages.iso_639_3, language_names.locale`,
	)
	if err != nil {
		return nil, fmt.Errorf("query language names: %w", err)
	}
	defer rows.Close()

	names := make(map[string]map[string]string)
	for rows.Next() {
		var languageID string
		var locale string
		var name string
		if err := rows.Scan(&languageID, &locale, &name); err != nil {
			return nil, fmt.Errorf("scan language name: %w", err)
		}
		if names[languageID] == nil {
			names[languageID] = make(map[string]string)
		}
		names[languageID][locale] = name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read language names: %w", err)
	}

	return names, nil
}

func loadTexts(ctx context.Context, db queryer, languages []quiz.GenerationLanguage) error {
	rows, err := db.Query(
		ctx,
		`SELECT languages.id::text, language_texts.id::text, language_texts.content, language_texts.approved
		 FROM language_texts
		 JOIN languages ON languages.id = language_texts.language_id
		 ORDER BY languages.iso_639_3, language_texts.created_at, language_texts.id`,
	)
	if err != nil {
		return fmt.Errorf("query language texts: %w", err)
	}
	defer rows.Close()

	index := make(map[string]int, len(languages))
	for position, language := range languages {
		index[language.ID] = position
	}

	for rows.Next() {
		var languageID string
		var text quiz.TextCandidate
		if err := rows.Scan(&languageID, &text.ID, &text.Content, &text.Approved); err != nil {
			return fmt.Errorf("scan language text: %w", err)
		}

		position, ok := index[languageID]
		if !ok {
			return fmt.Errorf("text references unknown language %q", languageID)
		}
		languages[position].Texts = append(languages[position].Texts, text)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read language texts: %w", err)
	}

	return nil
}

func hasRequiredNames(names map[string]string) bool {
	for _, locale := range seeddata.RequiredLocales() {
		if names[locale] == "" {
			return false
		}
	}
	return true
}
