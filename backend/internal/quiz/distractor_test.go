package quiz

import "testing"

func TestDistractorWeightUsesCumulativeMetadata(t *testing.T) {
	correct := LanguageMetadata{
		Family:    "indo-european",
		Group:     "italic",
		Subgroup:  "romance",
		Continent: "EU",
		Script:    "Latn",
	}
	candidate := LanguageMetadata{
		Family:    "indo-european",
		Group:     "italic",
		Subgroup:  "romance",
		Continent: "EU",
		Script:    "Latn",
	}

	weight, err := DistractorWeight(correct, candidate, DefaultDistractorWeights())
	if err != nil {
		t.Fatalf("DistractorWeight() error = %v", err)
	}

	if weight != 45 {
		t.Fatalf("weight = %d, want %d", weight, 45)
	}
}

func TestDistractorWeightKeepsUnrelatedLanguagesSelectable(t *testing.T) {
	correct := LanguageMetadata{
		Family:    "indo-european",
		Group:     "italic",
		Subgroup:  "romance",
		Continent: "EU",
		Script:    "Latn",
	}
	candidate := LanguageMetadata{
		Family:    "japonic",
		Group:     "japanese",
		Subgroup:  "japanese",
		Continent: "AS",
		Script:    "Jpan",
	}

	weight, err := DistractorWeight(correct, candidate, DefaultDistractorWeights())
	if err != nil {
		t.Fatalf("DistractorWeight() error = %v", err)
	}

	if weight != 1 {
		t.Fatalf("weight = %d, want %d", weight, 1)
	}
}

func TestDistractorWeightsRejectInvalidValues(t *testing.T) {
	weights := DefaultDistractorWeights()
	weights.Base = 0

	if err := weights.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}

	weights = DefaultDistractorWeights()
	weights.SameScript = -1

	if err := weights.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
