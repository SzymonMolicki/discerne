package quiz

import (
	"strconv"
	"testing"
)

func TestGeneratorCreatesQuizWithDistinctQuestionsAndOptions(t *testing.T) {
	generator := Generator{
		Weights: DefaultDistractorWeights(),
		Random:  NewSeededRandomSource(1),
	}

	quiz, err := generator.GenerateWithCounts(generationLanguages(), 5, 5)
	if err != nil {
		t.Fatalf("GenerateWithCounts() error = %v", err)
	}

	if len(quiz.Questions) != 5 {
		t.Fatalf("len(Questions) = %d, want %d", len(quiz.Questions), 5)
	}

	seenCorrectLanguages := make(map[string]struct{})
	seenTexts := make(map[string]struct{})
	for _, question := range quiz.Questions {
		if question.Position == 0 {
			t.Fatal("question position is zero")
		}
		if question.Text.ID == "" {
			t.Fatal("question text id is empty")
		}
		if _, exists := seenTexts[question.Text.ID]; exists {
			t.Fatalf("duplicate text %q", question.Text.ID)
		}
		seenTexts[question.Text.ID] = struct{}{}

		if _, exists := seenCorrectLanguages[question.CorrectLanguage.ID]; exists {
			t.Fatalf("duplicate correct language %q", question.CorrectLanguage.ID)
		}
		seenCorrectLanguages[question.CorrectLanguage.ID] = struct{}{}

		if len(question.Options) != 5 {
			t.Fatalf("len(Options) = %d, want %d", len(question.Options), 5)
		}
		assertQuestionOptions(t, question)
	}
}

func TestGeneratorRejectsMissingRandomSource(t *testing.T) {
	generator := Generator{
		Weights: DefaultDistractorWeights(),
	}

	_, err := generator.GenerateWithCounts(generationLanguages(), 5, 5)
	if err == nil {
		t.Fatal("GenerateWithCounts() error = nil, want error")
	}
}

func TestGeneratorRejectsTooFewEligibleLanguages(t *testing.T) {
	generator := Generator{
		Weights: DefaultDistractorWeights(),
		Random:  NewSeededRandomSource(1),
	}

	_, err := generator.GenerateWithCounts(generationLanguages()[:4], 5, 5)
	if err == nil {
		t.Fatal("GenerateWithCounts() error = nil, want error")
	}
}

func assertQuestionOptions(t *testing.T, question GeneratedQuestion) {
	t.Helper()

	correctCount := 0
	seenOptions := make(map[string]struct{}, len(question.Options))
	for _, option := range question.Options {
		if option.Position == 0 {
			t.Fatal("option position is zero")
		}
		if _, exists := seenOptions[option.Language.ID]; exists {
			t.Fatalf("duplicate option language %q", option.Language.ID)
		}
		seenOptions[option.Language.ID] = struct{}{}

		if option.IsCorrect {
			correctCount++
			if option.Language.ID != question.CorrectLanguage.ID {
				t.Fatalf("correct option language = %q, want %q", option.Language.ID, question.CorrectLanguage.ID)
			}
			if option.WeightAtGeneration != 0 {
				t.Fatalf("correct option weight = %d, want 0", option.WeightAtGeneration)
			}
		} else if option.WeightAtGeneration <= 0 {
			t.Fatalf("distractor option weight = %d, want positive", option.WeightAtGeneration)
		}
	}

	if correctCount != 1 {
		t.Fatalf("correct option count = %d, want %d", correctCount, 1)
	}
}

func generationLanguages() []GenerationLanguage {
	return []GenerationLanguage{
		generationLanguage("arb", "afro-asiatic", "semitic", "central-semitic", "AS", "Arab"),
		generationLanguage("deu", "indo-european", "germanic", "west-germanic", "EU", "Latn"),
		generationLanguage("eng", "indo-european", "germanic", "west-germanic", "EU", "Latn"),
		generationLanguage("fra", "indo-european", "italic", "romance", "EU", "Latn"),
		generationLanguage("jpn", "japonic", "japanese", "japanese", "AS", "Jpan"),
		generationLanguage("pol", "indo-european", "balto-slavic", "west-slavic", "EU", "Latn"),
		generationLanguage("rus", "indo-european", "balto-slavic", "east-slavic", "EU", "Cyrl"),
		generationLanguage("spa", "indo-european", "italic", "romance", "EU", "Latn"),
	}
}

func generationLanguage(id string, family string, group string, subgroup string, continent string, script string) GenerationLanguage {
	language := GenerationLanguage{
		LanguageCandidate: LanguageCandidate{
			ID: id,
			Metadata: LanguageMetadata{
				Family:    family,
				Group:     group,
				Subgroup:  subgroup,
				Continent: continent,
				Script:    script,
			},
			Enabled:          true,
			HasRequiredNames: true,
		},
	}

	for i := 1; i <= 5; i++ {
		language.Texts = append(language.Texts, TextCandidate{
			ID:       id + "-text-" + strconv.Itoa(i),
			Content:  "Example text.",
			Approved: true,
		})
	}

	return language
}
