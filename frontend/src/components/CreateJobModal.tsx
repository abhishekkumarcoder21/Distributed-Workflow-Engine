"use client";

import React, { useState } from "react";
import { createJob } from "@/lib/api";

interface CreateJobModalProps {
  isOpen: boolean;
  onClose: () => void;
  onJobCreated: () => void;
}

const TEMPLATES = [
  {
    label: "Send Email",
    type: "send_email",
    payload: { to: "user@example.com", subject: "Welcome to Distributed Workflow Engine!" },
    priority: 2,
  },
  {
    label: "Generate PDF Report",
    type: "generate_report",
    payload: { type: "pdf", user_id: "usr_48291" },
    priority: 1,
  },
  {
    label: "Deliver Webhook",
    type: "webhook_delivery",
    payload: { url: "https://httpbin.org/post", event: "user.signup" },
    priority: 3,
  },
];

export default function CreateJobModal({
  isOpen,
  onClose,
  onJobCreated,
}: CreateJobModalProps) {
  const [type, setType] = useState("send_email");
  const [priority, setPriority] = useState(2);
  const [maxAttempts, setMaxAttempts] = useState(3);
  const [idempotencyKey, setIdempotencyKey] = useState("");
  const [payloadText, setPayloadText] = useState(
    JSON.stringify({ to: "user@example.com", subject: "Welcome!" }, null, 2)
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleApplyTemplate = (tmpl: typeof TEMPLATES[0]) => {
    setType(tmpl.type);
    setPriority(tmpl.priority);
    setPayloadText(JSON.stringify(tmpl.payload, null, 2));
    setError(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    let parsedPayload = {};
    try {
      if (payloadText.trim()) {
        parsedPayload = JSON.parse(payloadText);
      }
    } catch {
      setError("Payload must be valid JSON");
      return;
    }

    setLoading(true);
    try {
      await createJob({
        type: type.trim(),
        payload: parsedPayload,
        priority: Number(priority),
        max_attempts: Number(maxAttempts),
        idempotency_key: idempotencyKey.trim() ? idempotencyKey.trim() : undefined,
      });
      onJobCreated();
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.5rem" }}>
          <h3 style={{ fontSize: "1.25rem", fontWeight: 700 }} className="brand-font">
            Submit New Job
          </h3>
          <button className="btn btn-secondary btn-sm" onClick={onClose}>
            ✕
          </button>
        </div>

        {/* Templates selector */}
        <div style={{ marginBottom: "1.5rem" }}>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.5rem" }}>
            Quick-start Templates:
          </div>
          <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
            {TEMPLATES.map((t) => (
              <button
                key={t.label}
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => handleApplyTemplate(t)}
              >
                {t.label}
              </button>
            ))}
          </div>
        </div>

        {error && (
          <div style={{ padding: "0.75rem", background: "var(--status-failed-bg)", border: "1px solid rgba(244,63,94,0.3)", borderRadius: "8px", color: "var(--status-failed)", fontSize: "0.8125rem", marginBottom: "1rem" }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Job Type</label>
            <input
              type="text"
              className="form-input"
              value={type}
              onChange={(e) => setType(e.target.value)}
              placeholder="e.g. send_email, generate_report"
              required
            />
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1rem" }}>
            <div className="form-group">
              <label className="form-label">Priority</label>
              <select
                className="form-select"
                value={priority}
                onChange={(e) => setPriority(Number(e.target.value))}
              >
                <option value={1}>1 - High Priority (Urgent)</option>
                <option value={2}>2 - Medium Priority (Default)</option>
                <option value={3}>3 - Low Priority (Background)</option>
              </select>
            </div>

            <div className="form-group">
              <label className="form-label">Max Attempts</label>
              <input
                type="number"
                min={1}
                max={10}
                className="form-input"
                value={maxAttempts}
                onChange={(e) => setMaxAttempts(Number(e.target.value))}
                required
              />
            </div>
          </div>

          <div className="form-group">
            <label className="form-label">Idempotency Key (Optional)</label>
            <input
              type="text"
              className="form-input mono"
              value={idempotencyKey}
              onChange={(e) => setIdempotencyKey(e.target.value)}
              placeholder="e.g. signup:usr_12345"
            />
            <div style={{ fontSize: "0.6875rem", color: "var(--text-muted)", marginTop: "0.25rem" }}>
              Prevents duplicate job executions on network retries.
            </div>
          </div>

          <div className="form-group">
            <label className="form-label">Payload (JSON)</label>
            <textarea
              className="form-textarea"
              value={payloadText}
              onChange={(e) => setPayloadText(e.target.value)}
              rows={5}
            />
          </div>

          <div style={{ display: "flex", justifyContent: "flex-end", gap: "0.75rem", marginTop: "1.5rem" }}>
            <button type="button" className="btn btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? "Submitting..." : "Enqueue Job"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
