package faults

import (
	"math/rand/v2"
	"sync"
	"time"
)

type Random interface {
	Float64() float64
}

type LockedRandom struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func NewLockedRandom(seed *int64) *LockedRandom {
	var s1, s2 uint64
	if seed == nil {
		now := uint64(time.Now().UnixNano())
		s1 = now
		s2 = now ^ 0x9e3779b97f4a7c15
	} else {
		s1 = uint64(*seed)
		s2 = uint64(*seed) ^ 0x9e3779b97f4a7c15
	}
	return &LockedRandom{rng: rand.New(rand.NewPCG(s1, s2))}
}

func (l *LockedRandom) Float64() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rng.Float64()
}
