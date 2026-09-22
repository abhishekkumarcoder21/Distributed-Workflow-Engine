"use client";

import React, { useState } from "react";
import { Job, JobStatus, cancelJob, retryJob } from "@/lib/api";

interface JobsTabProps {
  jobs: Job[];
  total: number;
  statusFilter: string;
  setStatusFilter: (status: string) => void;
  priorityFilter: number | undefined;
  setPriorityFilter: (priority: number | undefined) => void;
  onRefresh: () => void;
  onOpenCreateJob: () => void;
}

export default function JobsTab({
  jobs,
  total,
  statusFilter,
  setStatusFilter,
  priorityFilter,
  setPriorityFilter,
  onRefresh,
  onOpenCreateJob,
}: JobsTabProps) {
  const [search, setSearch] = useState("");
  const [selectedJob, setSelectedJob] = useState<Job | null>(null);
  const [actionMsg, setActionMsg] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const statuses: Array<{ label: string; value: string }> = [
    { label: "All", value: "ALL" },
    { label: "Queued", value: "QUEUED" },
    { label: "Running", value: "RUNNING" },
    { label: "Succeeded", value: "SUCCEEDED" },
    { label: "Retrying", value: "RETRYING" },
    { label: "Failed", value: "FAILED" },
    { label: "Dead Letter", value: "DEAD_LETTER" },
    { label: "Cancelled", value: "CANCELLED" },
  ];

  const filteredJobs = jobs.filter((j) => {
    if (!search) return true;
    const q = search.toLowerCase();
    return j.id.toLowerCase().includes(q) || j.type.toLowerCase().includes(q);
  });

  const handleCancel = async (id: string) => {
    try {
      await cancelJob(id);
      setActionMsg({ type: "success", text: `Job ${id.slice(0, 8)}... cancelled.` });
      onRefresh();
      if (selectedJob && selectedJob.id === id) {
        setSelectedJob({ ...selectedJob, status: "CANCELLED" });
      }
    } catch (err: any) {
      setActionMsg({ type: "error", text: err.message });
    }
  };

  const handleRetry = async (id: string) => {
    try {
      await retryJob(id);
      setActionMsg({ type: "success", text: `Job ${id.slice(0, 8)}... re-enqueued for retry.` });
      onRefresh();
      if (selectedJob && selectedJob.id === id) {
        setSelectedJob({ ...selectedJob, status: "QUEUED" });
      }
    } catch (err: any) {
      setActionMsg({ type: "error", text: err.message });
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "1.5rem" }}>
      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <h2 style={{ fontSize: "1.5rem", fontWeight: 700 }} className="brand-font">
            Jobs Console
          </h2>
          <p style={{ fontSize: "0.875rem", color: "var(--text-secondary)" }}>
            Inspect, filter, retry, and cancel individual units of work across all worker nodes.
          </p>
        </div>
        <button className="btn btn-primary" onClick={onOpenCreateJob}>
          + Submit Job
        </button>
      </div>

      {actionMsg && (
        <div
          style={{
            padding: "0.75rem 1rem",
            background: actionMsg.type === "success" ? "var(--status-succeeded-bg)" : "var(--status-failed-bg)",
            border: `1px solid ${actionMsg.type === "success" ? "rgba(16,185,129,0.3)" : "rgba(244,63,94,0.3)"}`,
            borderRadius: "8px",
            color: actionMsg.type === "success" ? "var(--status-succeeded)" : "var(--status-failed)",
            fontSize: "0.875rem",
          }}
        >
          {actionMsg.text}
        </div>
      )}

      {/* Filters Bar */}
      <div className="glass-panel" style={{ padding: "1rem" }}>
        <div className="filter-bar" style={{ margin: 0 }}>
          {/* Status Pills */}
          <div className="pill-group">
            {statuses.map((s) => (
              <button
                key={s.value}
                className={`pill ${statusFilter === s.value ? "active" : ""}`}
                onClick={() => setStatusFilter(s.value)}
              >
                {s.label}
              </button>
            ))}
          </div>

          {/* Priority Filter */}
          <div className="pill-group">
            <button
              className={`pill ${priorityFilter === undefined ? "active" : ""}`}
              onClick={() => setPriorityFilter(undefined)}
            >
              All Priorities
            </button>
            <button
              className={`pill ${priorityFilter === 1 ? "active" : ""}`}
              onClick={() => setPriorityFilter(1)}
            >
              P1 High
            </button>
            <button
              className={`pill ${priorityFilter === 2 ? "active" : ""}`}
              onClick={() => setPriorityFilter(2)}
            >
              P2 Medium
            </button>
            <button
              className={`pill ${priorityFilter === 3 ? "active" : ""}`}
              onClick={() => setPriorityFilter(3)}
            >
              P3 Low
            </button>
          </div>

          {/* Search Input */}
          <div style={{ minWidth: "220px" }}>
            <input
              type="text"
              placeholder="Search by ID or Type..."
              className="form-input"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              style={{ padding: "0.4rem 0.75rem", fontSize: "0.8125rem" }}
            />
          </div>
        </div>
      </div>

      {/* Jobs Table */}
      <div className="glass-panel" style={{ padding: "1rem" }}>
        <div className="table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>Job ID</th>
                <th>Type</th>
                <th>Priority</th>
                <th>Status</th>
                <th>Attempts</th>
                <th>Worker ID</th>
                <th>Created At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredJobs.length === 0 ? (
                <tr>
                  <td colSpan={8} style={{ textAlign: "center", padding: "3rem", color: "var(--text-muted)" }}>
                    No matching jobs found.
                  </td>
                </tr>
              ) : (
                filteredJobs.map((job) => (
                  <tr key={job.id}>
                    <td
                      className="mono"
                      style={{ color: "var(--text-accent)", cursor: "pointer", fontWeight: 500 }}
                      onClick={() => setSelectedJob(job)}
                    >
                      {job.id.slice(0, 8)}...
                    </td>
                    <td style={{ fontWeight: 600, color: "var(--text-primary)" }}>{job.type}</td>
                    <td>
                      <span className={`badge badge-p${job.priority}`}>P{job.priority}</span>
                    </td>
                    <td>
                      <span className={`badge badge-${job.status.toLowerCase()}`}>
                        {job.status}
                      </span>
                    </td>
                    <td>
                      {job.attempt} / {job.max_attempts}
                    </td>
                    <td className="mono" style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>
                      {job.worker_id ? job.worker_id.slice(0, 10) : "—"}
                    </td>
                    <td style={{ fontSize: "0.8125rem", color: "var(--text-muted)" }}>
                      {new Date(job.created_at).toLocaleTimeString()}
                    </td>
                    <td>
                      <div style={{ display: "flex", gap: "0.35rem" }}>
                        <button
                          className="btn btn-secondary btn-sm"
                          onClick={() => setSelectedJob(job)}
                        >
                          Inspect
                        </button>
                        {job.status === "QUEUED" || job.status === "RUNNING" ? (
                          <button
                            className="btn btn-danger btn-sm"
                            onClick={() => handleCancel(job.id)}
                          >
                            Cancel
                          </button>
                        ) : null}
                        {job.status === "DEAD_LETTER" || job.status === "FAILED" ? (
                          <button
                            className="btn btn-primary btn-sm"
                            onClick={() => handleRetry(job.id)}
                          >
                            Retry
                          </button>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Inspect Job Modal */}
      {selectedJob && (
        <div className="modal-overlay" onClick={() => setSelectedJob(null)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.5rem" }}>
              <div>
                <h3 style={{ fontSize: "1.25rem", fontWeight: 700 }} className="brand-font">
                  Job Details
                </h3>
                <span className="mono" style={{ fontSize: "0.8125rem", color: "var(--text-muted)" }}>
                  {selectedJob.id}
                </span>
              </div>
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => setSelectedJob(null)}
              >
                ✕
              </button>
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1rem", marginBottom: "1.5rem" }}>
              <div style={{ background: "rgba(255,255,255,0.02)", padding: "0.75rem", borderRadius: "8px", border: "1px solid var(--border-glass)" }}>
                <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.25rem" }}>Type</div>
                <div style={{ fontWeight: 600 }}>{selectedJob.type}</div>
              </div>

              <div style={{ background: "rgba(255,255,255,0.02)", padding: "0.75rem", borderRadius: "8px", border: "1px solid var(--border-glass)" }}>
                <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.25rem" }}>Status</div>
                <span className={`badge badge-${selectedJob.status.toLowerCase()}`}>
                  {selectedJob.status}
                </span>
              </div>

              <div style={{ background: "rgba(255,255,255,0.02)", padding: "0.75rem", borderRadius: "8px", border: "1px solid var(--border-glass)" }}>
                <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.25rem" }}>Priority & Attempts</div>
                <div>Priority: <span className={`badge badge-p${selectedJob.priority}`}>P{selectedJob.priority}</span> | Attempt: {selectedJob.attempt} / {selectedJob.max_attempts}</div>
              </div>

              <div style={{ background: "rgba(255,255,255,0.02)", padding: "0.75rem", borderRadius: "8px", border: "1px solid var(--border-glass)" }}>
                <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.25rem" }}>Worker ID</div>
                <div className="mono" style={{ fontSize: "0.8125rem" }}>{selectedJob.worker_id || "Unassigned"}</div>
              </div>
            </div>

            {selectedJob.idempotency_key && (
              <div style={{ marginBottom: "1rem" }}>
                <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.25rem" }}>Idempotency Key</div>
                <div className="mono" style={{ fontSize: "0.8125rem", background: "rgba(255,255,255,0.02)", padding: "0.5rem 0.75rem", borderRadius: "6px" }}>
                  {selectedJob.idempotency_key}
                </div>
              </div>
            )}

            {selectedJob.error && (
              <div style={{ marginBottom: "1rem" }}>
                <div style={{ fontSize: "0.75rem", color: "var(--status-failed)", marginBottom: "0.25rem" }}>Last Error</div>
                <pre style={{ background: "var(--status-failed-bg)", border: "1px solid rgba(244,63,94,0.3)", padding: "0.75rem", borderRadius: "8px", color: "var(--status-failed)", fontSize: "0.75rem", whiteSpace: "pre-wrap" }}>
                  {selectedJob.error}
                </pre>
              </div>
            )}

            <div style={{ marginBottom: "1.5rem" }}>
              <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.25rem" }}>Payload (JSON)</div>
              <pre style={{ background: "var(--bg-input)", border: "1px solid var(--border-glass)", padding: "0.75rem", borderRadius: "8px", fontSize: "0.75rem", overflowX: "auto" }}>
                {JSON.stringify(selectedJob.payload, null, 2)}
              </pre>
            </div>

            <div style={{ display: "flex", justifyContent: "flex-end", gap: "0.75rem" }}>
              {selectedJob.status === "QUEUED" || selectedJob.status === "RUNNING" ? (
                <button
                  className="btn btn-danger"
                  onClick={() => handleCancel(selectedJob.id)}
                >
                  Cancel Job
                </button>
              ) : null}
              {selectedJob.status === "DEAD_LETTER" || selectedJob.status === "FAILED" ? (
                <button
                  className="btn btn-primary"
                  onClick={() => handleRetry(selectedJob.id)}
                >
                  Retry Job
                </button>
              ) : null}
              <button
                className="btn btn-secondary"
                onClick={() => setSelectedJob(null)}
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
