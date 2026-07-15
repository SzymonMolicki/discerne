package quiz

import "fmt"

type LanguageMetadata struct {
	Family    string
	Group     string
	Subgroup  string
	Continent string
	Script    string
}

type DistractorWeights struct {
	Base          int
	SameFamily    int
	SameGroup     int
	SameSubgroup  int
	SameContinent int
	SameScript    int
}

func DefaultDistractorWeights() DistractorWeights {
	return DistractorWeights{
		Base:          1,
		SameFamily:    6,
		SameGroup:     8,
		SameSubgroup:  10,
		SameContinent: 6,
		SameScript:    8,
	}
}

func (weights DistractorWeights) Validate() error {
	if weights.Base <= 0 {
		return fmt.Errorf("base weight must be greater than zero")
	}
	if weights.SameFamily < 0 {
		return fmt.Errorf("same family weight must be non-negative")
	}
	if weights.SameGroup < 0 {
		return fmt.Errorf("same group weight must be non-negative")
	}
	if weights.SameSubgroup < 0 {
		return fmt.Errorf("same subgroup weight must be non-negative")
	}
	if weights.SameContinent < 0 {
		return fmt.Errorf("same continent weight must be non-negative")
	}
	if weights.SameScript < 0 {
		return fmt.Errorf("same script weight must be non-negative")
	}
	return nil
}

func DistractorWeight(correct LanguageMetadata, candidate LanguageMetadata, weights DistractorWeights) (int, error) {
	if err := weights.Validate(); err != nil {
		return 0, err
	}

	weight := weights.Base
	if sameNonEmpty(correct.Family, candidate.Family) {
		weight += weights.SameFamily
	}
	if sameNonEmpty(correct.Group, candidate.Group) {
		weight += weights.SameGroup
	}
	if sameNonEmpty(correct.Subgroup, candidate.Subgroup) {
		weight += weights.SameSubgroup
	}
	if sameNonEmpty(correct.Continent, candidate.Continent) {
		weight += weights.SameContinent
	}
	if sameNonEmpty(correct.Script, candidate.Script) {
		weight += weights.SameScript
	}

	return weight, nil
}

func sameNonEmpty(left string, right string) bool {
	return left != "" && left == right
}
