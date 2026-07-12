package quiz

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	mathrand "math/rand"
)

type SeededRandomSource struct {
	random *mathrand.Rand
}

func NewSeededRandomSource(seed int64) *SeededRandomSource {
	return &SeededRandomSource{
		random: mathrand.New(mathrand.NewSource(seed)),
	}
}

func NewCryptoSeededRandomSource() (*SeededRandomSource, error) {
	seed, err := randomSeed(rand.Reader)
	if err != nil {
		return nil, err
	}
	return NewSeededRandomSource(seed), nil
}

func (source *SeededRandomSource) Intn(n int) int {
	return source.random.Intn(n)
}

func (source *SeededRandomSource) Float64() float64 {
	return source.random.Float64()
}

func (source *SeededRandomSource) Shuffle(n int, swap func(i, j int)) {
	source.random.Shuffle(n, swap)
}

func randomSeed(reader io.Reader) (int64, error) {
	var buffer [8]byte
	if _, err := io.ReadFull(reader, buffer[:]); err != nil {
		return 0, fmt.Errorf("read random seed: %w", err)
	}
	return int64(binary.LittleEndian.Uint64(buffer[:])), nil
}
