package quiz

import "fmt"

type RandomSource interface {
	Intn(n int) int
	Float64() float64
	Shuffle(n int, swap func(i, j int))
}

type LanguageCandidate struct {
	ID               string
	Metadata         LanguageMetadata
	Enabled          bool
	HasRequiredNames bool
}

type SelectedDistractor struct {
	Language LanguageCandidate
	Weight   int
}

type weightedCandidate struct {
	language LanguageCandidate
	weight   int
}

func SelectDistractors(
	correct LanguageCandidate,
	candidates []LanguageCandidate,
	count int,
	weights DistractorWeights,
	random RandomSource,
) ([]SelectedDistractor, error) {
	if count < 0 {
		return nil, fmt.Errorf("distractor count must be non-negative")
	}
	if count == 0 {
		return nil, nil
	}
	if random == nil {
		return nil, fmt.Errorf("random source is required")
	}
	if correct.ID == "" {
		return nil, fmt.Errorf("correct language id is required")
	}
	if err := weights.Validate(); err != nil {
		return nil, err
	}

	eligible, err := eligibleDistractors(correct, candidates, weights)
	if err != nil {
		return nil, err
	}
	if len(eligible) < count {
		return nil, fmt.Errorf("not enough eligible distractors: have %d, want %d", len(eligible), count)
	}

	selected := make([]SelectedDistractor, 0, count)
	for len(selected) < count {
		index, err := weightedIndex(eligible, random)
		if err != nil {
			return nil, err
		}

		chosen := eligible[index]
		selected = append(selected, SelectedDistractor{
			Language: chosen.language,
			Weight:   chosen.weight,
		})

		eligible = append(eligible[:index], eligible[index+1:]...)
	}

	return selected, nil
}

func eligibleDistractors(
	correct LanguageCandidate,
	candidates []LanguageCandidate,
	weights DistractorWeights,
) ([]weightedCandidate, error) {
	seen := make(map[string]struct{}, len(candidates))
	eligible := make([]weightedCandidate, 0, len(candidates))

	for _, candidate := range candidates {
		if candidate.ID == "" {
			return nil, fmt.Errorf("candidate language id is required")
		}
		if _, exists := seen[candidate.ID]; exists {
			return nil, fmt.Errorf("duplicate candidate language %q", candidate.ID)
		}
		seen[candidate.ID] = struct{}{}

		if candidate.ID == correct.ID {
			continue
		}
		if !candidate.Enabled {
			continue
		}
		if !candidate.HasRequiredNames {
			continue
		}

		weight, err := DistractorWeight(correct.Metadata, candidate.Metadata, weights)
		if err != nil {
			return nil, err
		}
		if weight <= 0 {
			return nil, fmt.Errorf("candidate language %q has non-positive weight %d", candidate.ID, weight)
		}

		eligible = append(eligible, weightedCandidate{
			language: candidate,
			weight:   weight,
		})
	}

	return eligible, nil
}

func weightedIndex(candidates []weightedCandidate, random RandomSource) (int, error) {
	total := 0
	for _, candidate := range candidates {
		if candidate.weight <= 0 {
			return 0, fmt.Errorf("candidate language %q has non-positive weight %d", candidate.language.ID, candidate.weight)
		}
		total += candidate.weight
	}
	if total <= 0 {
		return 0, fmt.Errorf("total weight must be positive")
	}

	draw := random.Intn(total)
	running := 0
	for index, candidate := range candidates {
		running += candidate.weight
		if draw < running {
			return index, nil
		}
	}

	return 0, fmt.Errorf("weighted selection failed")
}
