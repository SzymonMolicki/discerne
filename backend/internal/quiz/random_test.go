package quiz

import (
	"bytes"
	"testing"
)

func TestSeededRandomSourceIsDeterministic(t *testing.T) {
	left := NewSeededRandomSource(123)
	right := NewSeededRandomSource(123)

	for i := 0; i < 10; i++ {
		leftValue := left.Intn(1000)
		rightValue := right.Intn(1000)
		if leftValue != rightValue {
			t.Fatalf("Intn value %d = %d, want %d", i, leftValue, rightValue)
		}
	}

	leftFloat := left.Float64()
	rightFloat := right.Float64()
	if leftFloat != rightFloat {
		t.Fatalf("Float64 = %f, want %f", leftFloat, rightFloat)
	}
}

func TestSeededRandomSourceShuffleIsDeterministic(t *testing.T) {
	leftValues := []int{1, 2, 3, 4, 5}
	rightValues := []int{1, 2, 3, 4, 5}

	left := NewSeededRandomSource(456)
	right := NewSeededRandomSource(456)

	left.Shuffle(len(leftValues), func(i, j int) {
		leftValues[i], leftValues[j] = leftValues[j], leftValues[i]
	})
	right.Shuffle(len(rightValues), func(i, j int) {
		rightValues[i], rightValues[j] = rightValues[j], rightValues[i]
	})

	for i := range leftValues {
		if leftValues[i] != rightValues[i] {
			t.Fatalf("shuffled value %d = %d, want %d", i, leftValues[i], rightValues[i])
		}
	}
}

func TestRandomSeedReadsEightBytes(t *testing.T) {
	seed, err := randomSeed(bytes.NewReader([]byte{1, 0, 0, 0, 0, 0, 0, 0}))
	if err != nil {
		t.Fatalf("randomSeed() error = %v", err)
	}

	if seed != 1 {
		t.Fatalf("seed = %d, want %d", seed, 1)
	}
}

func TestRandomSeedRejectsShortReader(t *testing.T) {
	_, err := randomSeed(bytes.NewReader([]byte{1, 2, 3}))
	if err == nil {
		t.Fatal("randomSeed() error = nil, want error")
	}
}
