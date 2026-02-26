package randex

import (
	"math/rand/v2"
	"time"
)

func NewTimePCGRand() *rand.Rand {
	seed := uint64(time.Now().Unix())
	randSource := rand.NewPCG(seed, ^seed)

	_rand := rand.New(randSource)
	randSource.Seed(_rand.Uint64(), _rand.Uint64())

	return _rand
}
