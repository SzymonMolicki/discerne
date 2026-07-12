package seedimport

import (
	"context"
	"fmt"

	"discerne/backend/internal/seeddata"

	"github.com/jackc/pgx/v5"
)

// Import writes a seed catalog into PostgreSQL using one transaction.
func Import(ctx context.Context, tx pgx.Tx, catalog seeddata.Catalog) error {
	familyIDs, err := importFamilies(ctx, tx, catalog.Families)
	if err != nil {
		return err
	}

	groupIDs, err := importGroups(ctx, tx, catalog.Groups, familyIDs)
	if err != nil {
		return err
	}

	subgroupIDs, err := importSubgroups(ctx, tx, catalog.Subgroups, groupIDs)
	if err != nil {
		return err
	}

	continentIDs, err := importContinents(ctx, tx, catalog.Continents)
	if err != nil {
		return err
	}

	scriptIDs, err := importScripts(ctx, tx, catalog.Scripts)
	if err != nil {
		return err
	}

	languageIDs, err := importLanguages(ctx, tx, catalog.Languages, familyIDs, groupIDs, subgroupIDs, continentIDs, scriptIDs)
	if err != nil {
		return err
	}

	if err := importLanguageNames(ctx, tx, catalog.LanguageNames, languageIDs); err != nil {
		return err
	}

	if err := importLanguageTexts(ctx, tx, catalog.Texts, languageIDs); err != nil {
		return err
	}

	return nil
}

func importFamilies(ctx context.Context, tx pgx.Tx, families []seeddata.Family) (map[string]string, error) {
	ids := make(map[string]string, len(families))
	for _, family := range families {
		id, err := upsertReturningID(
			ctx,
			tx,
			`INSERT INTO language_families (code, name)
			 VALUES ($1, $2)
			 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			family.Code,
			family.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("import family %q: %w", family.Code, err)
		}
		ids[family.Code] = id
	}
	return ids, nil
}

func importGroups(ctx context.Context, tx pgx.Tx, groups []seeddata.Group, familyIDs map[string]string) (map[string]string, error) {
	ids := make(map[string]string, len(groups))
	for _, group := range groups {
		familyID, ok := familyIDs[group.Family]
		if !ok {
			return nil, fmt.Errorf("group %q references unknown family %q", group.Code, group.Family)
		}

		id, err := upsertReturningID(
			ctx,
			tx,
			`INSERT INTO language_groups (family_id, code, name)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (code) DO UPDATE SET
			   family_id = EXCLUDED.family_id,
			   name = EXCLUDED.name
			 RETURNING id`,
			familyID,
			group.Code,
			group.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("import group %q: %w", group.Code, err)
		}
		ids[group.Code] = id
	}
	return ids, nil
}

func importSubgroups(ctx context.Context, tx pgx.Tx, subgroups []seeddata.Subgroup, groupIDs map[string]string) (map[string]string, error) {
	ids := make(map[string]string, len(subgroups))
	for _, subgroup := range subgroups {
		groupID, ok := groupIDs[subgroup.Group]
		if !ok {
			return nil, fmt.Errorf("subgroup %q references unknown group %q", subgroup.Code, subgroup.Group)
		}

		id, err := upsertReturningID(
			ctx,
			tx,
			`INSERT INTO language_subgroups (group_id, code, name)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (code) DO UPDATE SET
			   group_id = EXCLUDED.group_id,
			   name = EXCLUDED.name
			 RETURNING id`,
			groupID,
			subgroup.Code,
			subgroup.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("import subgroup %q: %w", subgroup.Code, err)
		}
		ids[subgroup.Code] = id
	}
	return ids, nil
}

func importContinents(ctx context.Context, tx pgx.Tx, continents []seeddata.Continent) (map[string]string, error) {
	ids := make(map[string]string, len(continents))
	for _, continent := range continents {
		id, err := upsertReturningID(
			ctx,
			tx,
			`INSERT INTO continents (code, name)
			 VALUES ($1, $2)
			 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			continent.Code,
			continent.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("import continent %q: %w", continent.Code, err)
		}
		ids[continent.Code] = id
	}
	return ids, nil
}

func importScripts(ctx context.Context, tx pgx.Tx, scripts []seeddata.Script) (map[string]string, error) {
	ids := make(map[string]string, len(scripts))
	for _, script := range scripts {
		id, err := upsertReturningID(
			ctx,
			tx,
			`INSERT INTO scripts (iso_15924_code, name)
			 VALUES ($1, $2)
			 ON CONFLICT (iso_15924_code) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			script.Code,
			script.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("import script %q: %w", script.Code, err)
		}
		ids[script.Code] = id
	}
	return ids, nil
}

func importLanguages(
	ctx context.Context,
	tx pgx.Tx,
	languages []seeddata.Language,
	familyIDs map[string]string,
	groupIDs map[string]string,
	subgroupIDs map[string]string,
	continentIDs map[string]string,
	scriptIDs map[string]string,
) (map[string]string, error) {
	ids := make(map[string]string, len(languages))
	for _, language := range languages {
		familyID, ok := familyIDs[language.Family]
		if !ok {
			return nil, fmt.Errorf("language %q references unknown family %q", language.ISO6393, language.Family)
		}
		groupID, ok := groupIDs[language.Group]
		if !ok {
			return nil, fmt.Errorf("language %q references unknown group %q", language.ISO6393, language.Group)
		}
		subgroupID, ok := subgroupIDs[language.Subgroup]
		if !ok {
			return nil, fmt.Errorf("language %q references unknown subgroup %q", language.ISO6393, language.Subgroup)
		}
		continentID, ok := continentIDs[language.Continent]
		if !ok {
			return nil, fmt.Errorf("language %q references unknown continent %q", language.ISO6393, language.Continent)
		}
		scriptID, ok := scriptIDs[language.Script]
		if !ok {
			return nil, fmt.Errorf("language %q references unknown script %q", language.ISO6393, language.Script)
		}

		id, err := upsertReturningID(
			ctx,
			tx,
			`INSERT INTO languages (
			   iso_639_3,
			   iso_639_1,
			   canonical_name,
			   family_id,
			   group_id,
			   subgroup_id,
			   continent_id,
			   script_id,
			   enabled
			 )
			 VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6, $7, $8, $9)
			 ON CONFLICT (iso_639_3) DO UPDATE SET
			   iso_639_1 = EXCLUDED.iso_639_1,
			   canonical_name = EXCLUDED.canonical_name,
			   family_id = EXCLUDED.family_id,
			   group_id = EXCLUDED.group_id,
			   subgroup_id = EXCLUDED.subgroup_id,
			   continent_id = EXCLUDED.continent_id,
			   script_id = EXCLUDED.script_id,
			   enabled = EXCLUDED.enabled,
			   updated_at = now()
			 RETURNING id`,
			language.ISO6393,
			language.ISO6391,
			language.CanonicalName,
			familyID,
			groupID,
			subgroupID,
			continentID,
			scriptID,
			language.Enabled,
		)
		if err != nil {
			return nil, fmt.Errorf("import language %q: %w", language.ISO6393, err)
		}
		ids[language.ISO6393] = id
	}
	return ids, nil
}

func importLanguageNames(ctx context.Context, tx pgx.Tx, names map[string]map[string]string, languageIDs map[string]string) error {
	for languageCode, localizedNames := range names {
		languageID, ok := languageIDs[languageCode]
		if !ok {
			return fmt.Errorf("language names reference unknown language %q", languageCode)
		}

		for locale, name := range localizedNames {
			if _, err := tx.Exec(
				ctx,
				`INSERT INTO language_names (language_id, locale, name)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (language_id, locale) DO UPDATE SET name = EXCLUDED.name`,
				languageID,
				locale,
				name,
			); err != nil {
				return fmt.Errorf("import %s language name for %q: %w", locale, languageCode, err)
			}
		}
	}
	return nil
}

func importLanguageTexts(ctx context.Context, tx pgx.Tx, files map[string]seeddata.TextFile, languageIDs map[string]string) error {
	for languageCode, textFile := range files {
		languageID, ok := languageIDs[textFile.Language]
		if !ok {
			return fmt.Errorf("text file %q references unknown language %q", languageCode, textFile.Language)
		}

		contents := make([]string, 0, len(textFile.Texts))
		for _, text := range textFile.Texts {
			contents = append(contents, text.Content)
			if _, err := tx.Exec(
				ctx,
				`INSERT INTO language_texts (
				   language_id,
				   content,
				   sentence_count,
				   source_type,
				   source_reference,
				   license,
				   approved
				 )
				 VALUES ($1, $2, $3, NULL, NULL, $4, $5)
				 ON CONFLICT (language_id, content) DO UPDATE SET
				   sentence_count = EXCLUDED.sentence_count,
				   source_type = EXCLUDED.source_type,
				   source_reference = EXCLUDED.source_reference,
				   license = EXCLUDED.license,
				   approved = EXCLUDED.approved,
				   updated_at = now()`,
				languageID,
				text.Content,
				text.SentenceCount,
				text.License,
				text.Approved,
			); err != nil {
				return fmt.Errorf("import text for language %q: %w", textFile.Language, err)
			}
		}

		if _, err := tx.Exec(
			ctx,
			`UPDATE language_texts
			 SET approved = false,
			     updated_at = now()
			 WHERE language_id = $1
			   AND NOT (content = ANY($2))`,
			languageID,
			contents,
		); err != nil {
			return fmt.Errorf("disable stale texts for language %q: %w", textFile.Language, err)
		}
	}
	return nil
}

func upsertReturningID(ctx context.Context, tx pgx.Tx, query string, args ...any) (string, error) {
	var id string
	if err := tx.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}
