"use client";

import React, { useState } from "react";
import { createWorkflow } from "@/lib/api";

interface CreateWorkflowModalProps {
  isOpen: boolean;
  onClose: () => void;
  onWorkflowCreated: () => void;
}

const WORKFLOW_TEMPLATES = [
  {
    name: "User Onboarding Pipeline",
    tasks: [
      {
        name: "create_user_record",
        type: "generate_report",
        payload: { user_id: "usr_new_01", action: "create" },
        dependencies: [],
      },
      {
        name: "send_welcome_email",
        type: "send_email",
        payload: { to: "newuser@example.com", subject: "Welcome to our platform!" },
        dependencies: ["create_user_record"],
      },
      {
        name: "provision_storage",
        type: "generate_report",
        payload: { user_id: "usr_new_01", bucket: "user-assets" },
        dependencies: ["create_user_record"],
      },
      {
        name: "notify_onboarding_complete",
        type: "webhook_delivery",
        payload: { url: "https://httpbin.org/post", event: "onboarding.completed" },
        dependencies: ["send_welcome_email", "provision_storage"],
      },
    ],
  },
  {
    name: "Financial Reconciliation Diamond DAG",
    tasks: [
      {
        name: "extract_daily_ledger",
        type: "generate_report",
        payload: { ledger_date: "2026-09-20" },
        dependencies: [],
      },
      {
        name: "match_transactions",
        type: "generate_report",
        payload: { stage: "matching" },
        dependencies: ["extract_daily_ledger"],
      },
      {
        name: "compute_forex_rates",
        type: "generate_report",
        payload: { stage: "forex_conversion" },
        dependencies: ["extract_daily_ledger"],
      },
      {
        name: "generate_audit_report",
        type: "send_email",
        payload: { to: "cfo@company.com", subject: "Daily Reconciliation Completed" },
        dependencies: ["match_transactions", "compute_forex_rates"],
      },
    ],
  },
];

export default function CreateWorkflowModal({
  isOpen,
  onClose,
  onWorkflowCreated,
}: CreateWorkflowModalProps) {
  const [name, setName] = useState("User Onboarding Pipeline");
  const [tasksText, setTasksText] = useState(
    JSON.stringify(WORKFLOW_TEMPLATES[0].tasks, null, 2)
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleApplyTemplate = (tmpl: typeof WORKFLOW_TEMPLATES[0]) => {
    setName(tmpl.name);
    setTasksText(JSON.stringify(tmpl.tasks, null, 2));
    setError(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    let parsedTasks = [];
    try {
      parsedTasks = JSON.parse(tasksText);
      if (!Array.isArray(parsedTasks) || parsedTasks.length === 0) {
        setError("Tasks must be a non-empty array of task objects");
        return;
      }
    } catch {
      setError("Tasks must be valid JSON array");
      return;
    }

    setLoading(true);
    try {
      await createWorkflow({
        name: name.trim(),
        tasks: parsedTasks,
      });
      onWorkflowCreated();
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
            Create Workflow (DAG)
          </h3>
          <button className="btn btn-secondary btn-sm" onClick={onClose}>
            ✕
          </button>
        </div>

        {/* Templates selector */}
        <div style={{ marginBottom: "1.5rem" }}>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.5rem" }}>
            Select Workflow Template:
          </div>
          <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
            {WORKFLOW_TEMPLATES.map((t) => (
              <button
                key={t.name}
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => handleApplyTemplate(t)}
              >
                {t.name}
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
            <label className="form-label">Workflow Name</label>
            <input
              type="text"
              className="form-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Order Processing Pipeline"
              required
            />
          </div>

          <div className="form-group">
            <label className="form-label">
              Tasks & Dependencies (JSON Array)
            </label>
            <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.5rem" }}>
              Specify task <code>name</code>, <code>type</code>, <code>payload</code>, and <code>dependencies</code> (list of task names that must succeed before this task runs).
            </div>
            <textarea
              className="form-textarea"
              value={tasksText}
              onChange={(e) => setTasksText(e.target.value)}
              rows={12}
            />
          </div>

          <div style={{ display: "flex", justifyContent: "flex-end", gap: "0.75rem", marginTop: "1.5rem" }}>
            <button type="button" className="btn btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? "Validating & Submitting..." : "Submit Workflow"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
