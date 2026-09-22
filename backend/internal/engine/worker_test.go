package engine

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestHandleSendEmail(t *testing.T) {
	ctx := context.Background()

	// Missing recipient should fail
	err := HandleSendEmail(ctx, json.RawMessage(`{"subject": "Hello"}`))
	if err == nil {
		t.Error("expected error for missing recipient, got nil")
	}

	// Invalid payload JSON should fail
	err = HandleSendEmail(ctx, json.RawMessage(`invalid-json`))
	if err == nil {
		t.Error("expected error for invalid json, got nil")
	}

	// Valid payload with context cancellation should return context error
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	err = HandleSendEmail(cancelCtx, json.RawMessage(`{"recipient": "user@example.com"}`))
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

func TestHandleGenerateReport(t *testing.T) {
	ctx := context.Background()

	// Missing report type should fail
	err := HandleGenerateReport(ctx, json.RawMessage(`{"user_id": "123"}`))
	if err == nil {
		t.Error("expected error for missing report_type, got nil")
	}

	// Context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	err = HandleGenerateReport(cancelCtx, json.RawMessage(`{"report_type": "pdf"}`))
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

func TestHandleWebhookDelivery(t *testing.T) {
	ctx := context.Background()

	// Missing URL should fail
	err := HandleWebhookDelivery(ctx, json.RawMessage(`{"method": "POST"}`))
	if err == nil {
		t.Error("expected error for missing url, got nil")
	}

	// Context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	err = HandleWebhookDelivery(cancelCtx, json.RawMessage(`{"url": "https://example.com"}`))
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

func TestWorker_RegisterHandler(t *testing.T) {
	w := NewWorker("w-1", nil, nil, nil, 100*time.Millisecond, 5*time.Second, 30*time.Second, 10*time.Second, 5, nil)
	w.RegisterHandler("custom_job", func(ctx context.Context, payload json.RawMessage) error {
		return nil
	})

	if _, ok := w.handlers["custom_job"]; !ok {
		t.Error("expected custom_job handler to be registered")
	}
}

func TestWorker_ConcurrencyAndDraining(t *testing.T) {
	w := NewWorker("w-test", nil, nil, nil, 10*time.Millisecond, 1*time.Second, 10*time.Second, 5*time.Second, 3, nil)
	if w.concurrency != 3 {
		t.Errorf("expected concurrency 3, got %d", w.concurrency)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately to test clean drain

	err := w.Run(ctx)
	if err != nil {
		t.Errorf("expected clean shutdown, got %v", err)
	}
}
