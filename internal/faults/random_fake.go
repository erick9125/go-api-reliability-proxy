package faults

// Test doubles, exported because the integration tests live in another package.

// FixedRandom always returns Value, so effects fire when Value < probability.
type FixedRandom struct {
	Value float64
}

func (f FixedRandom) Float64() float64 {
	return f.Value
}
