package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Example job handlers that simulate realistic work.
// These are compiled into the worker binary — adding a new job type means
// adding a handler function here and registering it in cmd/worker/main.go.

type emailPayload struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

func HandleSendEmail(ctx context.Context, payload json.RawMessage) error {
	var p emailPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid email payload: %w", err)
	}
	if p.Recipient == "" {
		return fmt.Errorf("recipient is required")
	}

	// Simulate sending an email.
	slog.Info("sending email", "to", p.Recipient, "subject", p.Subject)
	select {
	case <-time.After(200 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type reportPayload struct {
	ReportType string `json:"report_type"`
	UserID     string `json:"user_id"`
}

func HandleGenerateReport(ctx context.Context, payload json.RawMessage) error {
	var p reportPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid report payload: %w", err)
	}
	if p.ReportType == "" {
		return fmt.Errorf("report_type is required")
	}

	// Simulate report generation — takes a bit longer.
	slog.Info("generating report", "type", p.ReportType, "user_id", p.UserID)
	select {
	case <-time.After(500 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type webhookPayload struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

func HandleWebhookDelivery(ctx context.Context, payload json.RawMessage) error {
	var p webhookPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid webhook payload: %w", err)
	}
	if p.URL == "" {
		return fmt.Errorf("url is required")
	}

	slog.Info("delivering webhook", "url", p.URL, "method", p.Method)
	select {
	case <-time.After(150 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
