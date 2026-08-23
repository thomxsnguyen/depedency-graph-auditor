package worker

import (
	"math"
	"math/rand"
	"time"
)

const (
	baseDelay = 1 * time.Second
	maxDelay  = 30 * time.Second
)

// Backoff returns the jittered sleep duration for the given attempt number.
// attempt is 1-indexed: attempt=1 is the first retry after an initial failure.
//
// Formula: rand(0, min(maxDelay, baseDelay × 2^(attempt-1)))
//
// Full jitter is used (not additive) so that retries across a fleet are
// spread uniformly across the window, avoiding thundering herds.
func Backoff(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt-1))
	window := time.Duration(float64(baseDelay) * exp)
	if window > maxDelay {
		window = maxDelay
	}
	return time.Duration(rand.Int63n(int64(window)))
}
