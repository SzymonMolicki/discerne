package quizdb

import (
	"context"
	"fmt"

	"discerne/backend/internal/quiz"
	"discerne/backend/internal/seeddata"

	"github.com/jackc/pgx/v5"
)

// Catalog contains database data prepared for quiz generation.
type Catalog struct {
	Languages     []quiz.GenerationLanguage
	LanguageNames map[string]map[string]string
}

// LoadCatalog reads quiz generation inputs from PostgreSQL.
func LoadCatalog(ctx context.Context, conn *pgx.Conn) (Catalog, error) {
	names, err := loadLanguageNames(ctx, conn)
	if err != nil {
		return Catalog{}, err
	}

	languages, err := loadLanguages(ctx, conn, names)
	if err != nil {
		return Catalog{}, err
	}

	if err := loadTexts(ctx, conn, languages); err != nil {
		return Catalog{}, err
	}

	return Catalog{
		Languages:     languages,
		LanguageNames: names,
	}, nil
}

func loadLanguages(
	ctx context.Context,
	conn *pgx.Conn,
	names map[string]map[string]string,
) ([]quiz.GenerationLanguage, error) {
	rows, err := conn.Query(
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

func loadLanguageNames(ctx context.Context, conn *pgx.Conn) (map[string]map[string]string, error) {
	rows, err := conn.Query(
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

func loadTexts(ctx context.Context, conn *pgx.Conn, languages []quiz.GenerationLanguage) error {
	rows, err := conn.Query(
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
