package engine

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	base := 1 * time.Second
	max := 10 * time.Second

	// Attempt 1: delay must be in [0, 1s]
	for i := 0; i < 20; i++ {
		d := CalculateBackoff(1, base, max)
		if d < 0 || d > base {
			t.Errorf("attempt 1: expected 0 <= d <= %v, got %v", base, d)
		}
	}

	// Attempt 2: delay must be in [0, 2s]
	for i := 0; i < 20; i++ {
		d := CalculateBackoff(2, base, max)
		if d < 0 || d > 2*base {
			t.Errorf("attempt 2: expected 0 <= d <= %v, got %v", 2*base, d)
		}
	}

	// Attempt 3: delay must be in [0, 4s]
	for i := 0; i < 20; i++ {
		d := CalculateBackoff(3, base, max)
		if d < 0 || d > 4*base {
			t.Errorf("attempt 3: expected 0 <= d <= %v, got %v", 4*base, d)
		}
	}

	// Large attempt: must never exceed max
	for i := 0; i < 20; i++ {
		d := CalculateBackoff(50, base, max)
		if d < 0 || d > max {
			t.Errorf("large attempt: expected 0 <= d <= %v, got %v", max, d)
		}
	}

	// Non-positive attempt defaults
	d := CalculateBackoff(0, base, max)
	if d < 0 || d > base {
		t.Errorf("attempt 0: expected 0 <= d <= %v, got %v", base, d)
	}
}
