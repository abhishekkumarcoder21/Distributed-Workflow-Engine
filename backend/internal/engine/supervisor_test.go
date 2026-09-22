package engine

import (
	"context"
	"testing"
	"time"
)

func TestNewWorkerSupervisor(t *testing.T) {
	s := NewWorkerSupervisor(nil, nil, 5*time.Second, 20*time.Second, nil)
	if s.checkPeriod != 5*time.Second {
		t.Errorf("expected check period 5s, got %v", s.checkPeriod)
	}
	if s.deadTimeout != 20*time.Second {
		t.Errorf("expected dead timeout 20s, got %v", s.deadTimeout)
	}

	// Defaults
	sDef := NewWorkerSupervisor(nil, nil, 0, 0, nil)
	if sDef.checkPeriod != 10*time.Second {
		t.Errorf("expected default check period 10s, got %v", sDef.checkPeriod)
	}
	if sDef.deadTimeout != 30*time.Second {
		t.Errorf("expected default dead timeout 30s, got %v", sDef.deadTimeout)
	}
}

func TestWorkerSupervisor_ContextCancel(t *testing.T) {
	s := NewWorkerSupervisor(nil, nil, 10*time.Millisecond, 10*time.Millisecond, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Run(ctx)
	if err != nil {
		t.Errorf("expected nil error on clean shutdown, got %v", err)
	}
}
