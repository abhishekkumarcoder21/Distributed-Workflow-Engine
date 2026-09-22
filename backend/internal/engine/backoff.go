package engine

import (
	"math/rand/v2"
	"time"
)

const (
	DefaultBaseDelay = 1 * time.Second
	DefaultMaxDelay  = 1 * time.Hour
)

// CalculateBackoff computes exponential backoff with full jitter.
// Formula:
//
//	temp = min(maxDelay, baseDelay * 2^(attempt-1))
//	delay = rand(0, temp)
//
// Full jitter prevents the thundering herd problem when multiple jobs fail
// simultaneously due to a shared dependency (e.g. third-party API outage).
func CalculateBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	if baseDelay <= 0 {
		baseDelay = DefaultBaseDelay
	}
	if maxDelay <= 0 {
		maxDelay = DefaultMaxDelay
	}
	if attempt <= 0 {
		attempt = 1
	}

	// Calculate 2^(attempt-1) without integer overflow
	var multiplier int64 = 1
	if attempt > 1 {
		shift := attempt - 1
		if shift > 30 {
			shift = 30
		}
		multiplier = int64(1) << shift
	}

	temp := float64(baseDelay) * float64(multiplier)
	if temp > float64(maxDelay) {
		temp = float64(maxDelay)
	}

	// Full jitter: uniformly random between 0 and temp
	if temp <= 0 {
		return 0
	}
	jitter := rand.Float64() * temp
	return time.Duration(jitter)
}
