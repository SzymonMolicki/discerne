package quiz

import "testing"

type fakeRandomSource struct {
	values []int
	index  int
}

func (source *fakeRandomSource) Intn(n int) int {
	if source.index >= len(source.values) {
		return 0
	}
	value := source.values[source.index]
	source.index++
	return value % n
}

func (source *fakeRandomSource) Float64() float64 {
	return 0
}

func (source *fakeRandomSource) Shuffle(n int, swap func(i, j int)) {
}

func TestSelectDistractorsUsesWeightedSamplingWithoutReplacement(t *testing.T) {
	correct := candidate("spa", "indo-european", "italic", "romance", "EU", "Latn")
	candidates := []LanguageCandidate{
		correct,
		candidate("fra", "indo-european", "italic", "romance", "EU", "Latn"),
		candidate("deu", "indo-european", "germanic", "west-germanic", "EU", "Latn"),
		candidate("jpn", "japonic", "japanese", "japanese", "AS", "Jpan"),
		candidate("arb", "afro-asiatic", "semitic", "central-semitic", "AS", "Arab"),
	}
	random := &fakeRandomSource{values: []int{0, 21, 21}}

	selected, err := SelectDistractors(correct, candidates, 3, DefaultDistractorWeights(), random)
	if err != nil {
		t.Fatalf("SelectDistractors() error = %v", err)
	}

	if len(selected) != 3 {
		t.Fatalf("len(selected) = %d, want %d", len(selected), 3)
	}

	assertSelected(t, selected[0], "fra", 39)
	assertSelected(t, selected[1], "jpn", 1)
	assertSelected(t, selected[2], "arb", 1)
}

func TestSelectDistractorsFiltersIneligibleLanguages(t *testing.T) {
	correct := candidate("spa", "indo-european", "italic", "romance", "EU", "Latn")

	disabled := candidate("fra", "indo-european", "italic", "romance", "EU", "Latn")
	disabled.Enabled = false

	missingNames := candidate("deu", "indo-european", "germanic", "west-germanic", "EU", "Latn")
	missingNames.HasRequiredNames = false

	candidates := []LanguageCandidate{
		correct,
		disabled,
		missingNames,
		candidate("jpn", "japonic", "japanese", "japanese", "AS", "Jpan"),
	}
	random := &fakeRandomSource{values: []int{0}}

	selected, err := SelectDistractors(correct, candidates, 1, DefaultDistractorWeights(), random)
	if err != nil {
		t.Fatalf("SelectDistractors() error = %v", err)
	}

	assertSelected(t, selected[0], "jpn", 1)
}

func TestSelectDistractorsRejectsDuplicateCandidates(t *testing.T) {
	correct := candidate("spa", "indo-european", "italic", "romance", "EU", "Latn")
	candidates := []LanguageCandidate{
		candidate("fra", "indo-european", "italic", "romance", "EU", "Latn"),
		candidate("fra", "indo-european", "italic", "romance", "EU", "Latn"),
	}

	_, err := SelectDistractors(correct, candidates, 1, DefaultDistractorWeights(), &fakeRandomSource{})
	if err == nil {
		t.Fatal("SelectDistractors() error = nil, want error")
	}
}

func TestSelectDistractorsRejectsTooFewEligibleCandidates(t *testing.T) {
	correct := candidate("spa", "indo-european", "italic", "romance", "EU", "Latn")
	candidates := []LanguageCandidate{
		candidate("fra", "indo-european", "italic", "romance", "EU", "Latn"),
	}

	_, err := SelectDistractors(correct, candidates, 2, DefaultDistractorWeights(), &fakeRandomSource{})
	if err == nil {
		t.Fatal("SelectDistractors() error = nil, want error")
	}
}

func candidate(id string, family string, group string, subgroup string, continent string, script string) LanguageCandidate {
	return LanguageCandidate{
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
	}
}

func assertSelected(t *testing.T, selected SelectedDistractor, id string, weight int) {
	t.Helper()

	if selected.Language.ID != id {
		t.Fatalf("selected language = %q, want %q", selected.Language.ID, id)
	}
	if selected.Weight != weight {
		t.Fatalf("selected weight = %d, want %d", selected.Weight, weight)
	}
}
