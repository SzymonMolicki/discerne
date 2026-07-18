package quiz

import "fmt"

const DefaultQuestionCount = 8
const DefaultOptionCount = 5

type Generator struct {
	Weights DistractorWeights
	Random  RandomSource
}

type GenerationLanguage struct {
	LanguageCandidate
	Texts []TextCandidate
}

type TextCandidate struct {
	ID       string
	Content  string
	Approved bool
}

type GeneratedQuiz struct {
	Questions []GeneratedQuestion
}

type GeneratedQuestion struct {
	Position        int
	Text            TextCandidate
	CorrectLanguage LanguageCandidate
	Options         []GeneratedOption
}

type GeneratedOption struct {
	Position           int
	Language           LanguageCandidate
	IsCorrect          bool
	WeightAtGeneration int
}

func (generator Generator) Generate(languages []GenerationLanguage) (GeneratedQuiz, error) {
	return generator.GenerateWithCounts(languages, DefaultQuestionCount, DefaultOptionCount)
}

func (generator Generator) GenerateWithCounts(
	languages []GenerationLanguage,
	questionCount int,
	optionCount int,
) (GeneratedQuiz, error) {
	if questionCount <= 0 {
		return GeneratedQuiz{}, fmt.Errorf("question count must be greater than zero")
	}
	if optionCount < 2 {
		return GeneratedQuiz{}, fmt.Errorf("option count must be at least two")
	}
	if generator.Random == nil {
		return GeneratedQuiz{}, fmt.Errorf("random source is required")
	}
	if err := generator.Weights.Validate(); err != nil {
		return GeneratedQuiz{}, err
	}

	eligibleLanguages, err := eligibleCorrectLanguages(languages)
	if err != nil {
		return GeneratedQuiz{}, err
	}
	if len(eligibleLanguages) < questionCount {
		return GeneratedQuiz{}, fmt.Errorf("not enough eligible correct languages: have %d, want %d", len(eligibleLanguages), questionCount)
	}
	if len(eligibleLanguages) < optionCount {
		return GeneratedQuiz{}, fmt.Errorf("not enough eligible option languages: have %d, want %d", len(eligibleLanguages), optionCount)
	}

	shuffleGenerationLanguages(eligibleLanguages, generator.Random)
	correctLanguages := eligibleLanguages[:questionCount]
	languageCandidates := languageCandidates(eligibleLanguages)

	quiz := GeneratedQuiz{
		Questions: make([]GeneratedQuestion, 0, questionCount),
	}
	usedTexts := make(map[string]struct{}, questionCount)

	for questionIndex, correctLanguage := range correctLanguages {
		text, err := selectApprovedText(correctLanguage.Texts, usedTexts, generator.Random)
		if err != nil {
			return GeneratedQuiz{}, fmt.Errorf("select text for language %q: %w", correctLanguage.ID, err)
		}
		usedTexts[text.ID] = struct{}{}

		distractors, err := SelectDistractors(
			correctLanguage.LanguageCandidate,
			languageCandidates,
			optionCount-1,
			generator.Weights,
			generator.Random,
		)
		if err != nil {
			return GeneratedQuiz{}, fmt.Errorf("select distractors for language %q: %w", correctLanguage.ID, err)
		}

		options := optionsForQuestion(correctLanguage.LanguageCandidate, distractors)
		generator.Random.Shuffle(len(options), func(i, j int) {
			options[i], options[j] = options[j], options[i]
		})
		for optionIndex := range options {
			options[optionIndex].Position = optionIndex + 1
		}

		quiz.Questions = append(quiz.Questions, GeneratedQuestion{
			Position:        questionIndex + 1,
			Text:            text,
			CorrectLanguage: correctLanguage.LanguageCandidate,
			Options:         options,
		})
	}

	return quiz, nil
}

func eligibleCorrectLanguages(languages []GenerationLanguage) ([]GenerationLanguage, error) {
	seen := make(map[string]struct{}, len(languages))
	eligible := make([]GenerationLanguage, 0, len(languages))

	for _, language := range languages {
		if language.ID == "" {
			return nil, fmt.Errorf("language id is required")
		}
		if _, exists := seen[language.ID]; exists {
			return nil, fmt.Errorf("duplicate language %q", language.ID)
		}
		seen[language.ID] = struct{}{}

		if !language.Enabled {
			continue
		}
		if !language.HasRequiredNames {
			continue
		}
		if approvedTextCount(language.Texts) == 0 {
			continue
		}

		eligible = append(eligible, language)
	}

	return eligible, nil
}

func approvedTextCount(texts []TextCandidate) int {
	count := 0
	for _, text := range texts {
		if text.Approved {
			count++
		}
	}
	return count
}

func selectApprovedText(texts []TextCandidate, usedTexts map[string]struct{}, random RandomSource) (TextCandidate, error) {
	eligible := make([]TextCandidate, 0, len(texts))
	for _, text := range texts {
		if !text.Approved {
			continue
		}
		if text.ID == "" {
			return TextCandidate{}, fmt.Errorf("text id is required")
		}
		if _, used := usedTexts[text.ID]; used {
			continue
		}
		eligible = append(eligible, text)
	}

	if len(eligible) == 0 {
		return TextCandidate{}, fmt.Errorf("no approved unused texts")
	}

	return eligible[random.Intn(len(eligible))], nil
}

func languageCandidates(languages []GenerationLanguage) []LanguageCandidate {
	candidates := make([]LanguageCandidate, 0, len(languages))
	for _, language := range languages {
		candidates = append(candidates, language.LanguageCandidate)
	}
	return candidates
}

func optionsForQuestion(correct LanguageCandidate, distractors []SelectedDistractor) []GeneratedOption {
	options := make([]GeneratedOption, 0, len(distractors)+1)
	options = append(options, GeneratedOption{
		Language:           correct,
		IsCorrect:          true,
		WeightAtGeneration: 0,
	})

	for _, distractor := range distractors {
		options = append(options, GeneratedOption{
			Language:           distractor.Language,
			IsCorrect:          false,
			WeightAtGeneration: distractor.Weight,
		})
	}

	return options
}

func shuffleGenerationLanguages(languages []GenerationLanguage, random RandomSource) {
	random.Shuffle(len(languages), func(i, j int) {
		languages[i], languages[j] = languages[j], languages[i]
	})
}
