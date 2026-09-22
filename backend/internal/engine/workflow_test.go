package engine

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewWorkflowEngine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	eng := NewWorkflowEngine(nil, nil, nil, logger)
	if eng == nil {
		t.Fatal("expected non-nil WorkflowEngine")
	}
	if eng.pollInterval != 1*time.Second {
		t.Errorf("expected 1s poll interval, got %v", eng.pollInterval)
	}
}

func TestWorkflowEngine_ContextCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	eng := NewWorkflowEngine(nil, nil, nil, logger)
	eng.pollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		eng.Run(ctx)
		close(done)
	}()

	time.Sleep(25 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Clean exit
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WorkflowEngine did not exit after context cancellation")
	}
}

func TestWorkflowEngine_AdvanceWorkflow_NilStore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	eng := NewWorkflowEngine(nil, nil, nil, logger)

	err := eng.AdvanceWorkflow(context.Background(), uuid.New())
	if err == nil {
		t.Error("expected error when workflow store is nil")
	}
}
