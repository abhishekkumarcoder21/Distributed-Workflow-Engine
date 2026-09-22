package engine

import (
	"context"
	"testing"
	"time"
)

func TestNewRetryScheduler(t *testing.T) {
	s := NewRetryScheduler(nil, nil, 2*time.Second, 50, nil)
	if s.interval != 2*time.Second {
		t.Errorf("expected interval 2s, got %v", s.interval)
	}
	if s.batchSize != 50 {
		t.Errorf("expected batch size 50, got %d", s.batchSize)
	}

	// Default fallback
	sDef := NewRetryScheduler(nil, nil, 0, 0, nil)
	if sDef.interval != 1*time.Second {
		t.Errorf("expected default interval 1s, got %v", sDef.interval)
	}
	if sDef.batchSize != 100 {
		t.Errorf("expected default batch size 100, got %d", sDef.batchSize)
	}
}

func TestRetryScheduler_ContextCancel(t *testing.T) {
	s := NewRetryScheduler(nil, nil, 10*time.Millisecond, 10, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Run(ctx)
	if err != nil {
		t.Errorf("expected nil error on clean shutdown, got %v", err)
	}
}
