package quizseed

import (
	"fmt"
	"strconv"

	"discerne/backend/internal/quiz"
	"discerne/backend/internal/seeddata"
)

func LoadGenerationLanguages(dataDir string) ([]quiz.GenerationLanguage, error) {
	catalog, err := seeddata.Load(dataDir)
	if err != nil {
		return nil, err
	}
	return GenerationLanguages(catalog)
}

func GenerationLanguages(catalog seeddata.Catalog) ([]quiz.GenerationLanguage, error) {
	languages := make([]quiz.GenerationLanguage, 0, len(catalog.Languages))

	for _, language := range catalog.Languages {
		textFile, hasTextFile := catalog.Texts[language.ISO6393]
		if hasTextFile && textFile.Language != language.ISO6393 {
			return nil, fmt.Errorf("text file for %q declares language %q", language.ISO6393, textFile.Language)
		}

		generationLanguage := quiz.GenerationLanguage{
			LanguageCandidate: quiz.LanguageCandidate{
				ID: language.ISO6393,
				Metadata: quiz.LanguageMetadata{
					Family:    language.Family,
					Group:     language.Group,
					Subgroup:  language.Subgroup,
					Continent: language.Continent,
					Script:    language.Script,
				},
				Enabled:          language.Enabled,
				HasRequiredNames: hasRequiredNames(catalog.LanguageNames[language.ISO6393]),
			},
		}

		for index, text := range textFile.Texts {
			generationLanguage.Texts = append(generationLanguage.Texts, quiz.TextCandidate{
				ID:       language.ISO6393 + "-text-" + strconv.Itoa(index+1),
				Content:  text.Content,
				Approved: text.Approved,
			})
		}

		languages = append(languages, generationLanguage)
	}

	return languages, nil
}

func hasRequiredNames(names map[string]string) bool {
	for _, locale := range seeddata.RequiredLocales() {
		if names[locale] == "" {
			return false
		}
	}
	return true
}
