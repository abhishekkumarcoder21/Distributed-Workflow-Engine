"use client";

import React from "react";
import { Worker } from "@/lib/api";

interface WorkersTabProps {
  workers: Worker[];
  onRefresh: () => void;
}

export default function WorkersTab({ workers, onRefresh }: WorkersTabProps) {
  const activeCount = workers.filter((w) => w.status === "ACTIVE").length;
  const drainingCount = workers.filter((w) => w.status === "DRAINING").length;
  const offlineCount = workers.filter((w) => w.status === "OFFLINE").length;

  const totalCompleted = workers.reduce((acc, w) => acc + (w.completed_count || 0), 0);
  const totalFailed = workers.reduce((acc, w) => acc + (w.failed_count || 0), 0);

  const getRelativeTime = (isoString: string) => {
    if (!isoString) return "—";
    const diffSec = Math.round((Date.now() - new Date(isoString).getTime()) / 1000);
    if (diffSec < 5) return "just now";
    if (diffSec < 60) return `${diffSec}s ago`;
    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) return `${diffMin}m ago`;
    return `${Math.floor(diffMin / 60)}h ago`;
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "2rem" }}>
      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <h2 style={{ fontSize: "1.5rem", fontWeight: 700 }} className="brand-font">
            Worker Nodes
          </h2>
          <p style={{ fontSize: "0.875rem", color: "var(--text-secondary)" }}>
            Real-time worker concurrency pools, heartbeat monitoring, and dead worker recovery.
          </p>
        </div>
        <button className="btn btn-secondary" onClick={onRefresh}>
          ↻ Refresh Nodes
        </button>
      </div>

      {/* Summary Cards */}
      <div className="stats-grid">
        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--accent-cyan)" }}>
          <div className="stat-title">
            <span>Total Nodes</span>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect width="20" height="8" x="2" y="2" rx="2"/><rect width="20" height="8" x="2" y="14" rx="2"/><line x1="6" x2="6.01" y1="6" y2="6"/><line x1="6" x2="6.01" y1="18" y2="18"/></svg>
          </div>
          <div className="stat-value">{workers.length}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Registered in cluster</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--status-succeeded)" }}>
          <div className="stat-title">
            <span>Active Workers</span>
            <span className="pulse-dot online" />
          </div>
          <div className="stat-value" style={{ color: "var(--status-succeeded)" }}>{activeCount}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Consuming queues</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--status-retrying)" }}>
          <div className="stat-title">
            <span>Draining</span>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--status-retrying)" strokeWidth="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
          </div>
          <div className="stat-value" style={{ color: "var(--status-retrying)" }}>{drainingCount}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Graceful shutdown in-flight</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--status-failed)" }}>
          <div className="stat-title">
            <span>Offline</span>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--status-failed)" strokeWidth="2"><line x1="18" x2="6" y1="6" y2="18"/><line x1="6" x2="18" y1="6" y2="18"/></svg>
          </div>
          <div className="stat-value" style={{ color: "var(--text-muted)" }}>{offlineCount}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Heartbeat expired or stopped</div>
        </div>

        <div className="glass-panel stat-card">
          <div className="stat-title">
            <span>Throughput Stats</span>
            <span style={{ fontSize: "0.75rem", color: "var(--status-succeeded)" }}>{totalCompleted} OK</span>
          </div>
          <div className="stat-value">{totalCompleted + totalFailed}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>{totalFailed} failed executions</div>
        </div>
      </div>

      {/* Workers Table */}
      <div className="glass-panel" style={{ padding: "1.25rem" }}>
        <div className="table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>Worker ID</th>
                <th>Status</th>
                <th>Heartbeat</th>
                <th>Current Job</th>
                <th>Completed</th>
                <th>Failed</th>
                <th>Started At</th>
              </tr>
            </thead>
            <tbody>
              {workers.length === 0 ? (
                <tr>
                  <td colSpan={7} style={{ textAlign: "center", padding: "3rem", color: "var(--text-muted)" }}>
                    No worker processes currently connected. Start a worker via:
                    <div style={{ marginTop: "0.5rem" }}>
                      <code style={{ background: "rgba(0,0,0,0.5)", padding: "4px 8px", borderRadius: "4px", color: "var(--accent-cyan)" }}>
                        go run ./cmd/worker
                      </code>
                    </div>
                  </td>
                </tr>
              ) : (
                workers.map((worker) => (
                  <tr key={worker.id}>
                    <td className="mono" style={{ fontWeight: 600, color: "var(--text-primary)" }}>
                      {worker.id}
                    </td>
                    <td>
                      <span
                        className="badge"
                        style={{
                          background:
                            worker.status === "ACTIVE"
                              ? "var(--status-succeeded-bg)"
                              : worker.status === "DRAINING"
                              ? "var(--status-retrying-bg)"
                              : "rgba(255,255,255,0.05)",
                          color:
                            worker.status === "ACTIVE"
                              ? "var(--status-succeeded)"
                              : worker.status === "DRAINING"
                              ? "var(--status-retrying)"
                              : "var(--text-muted)",
                          border: `1px solid ${
                            worker.status === "ACTIVE"
                              ? "rgba(16,185,129,0.3)"
                              : worker.status === "DRAINING"
                              ? "rgba(245,158,11,0.3)"
                              : "var(--border-glass)"
                          }`,
                        }}
                      >
                        {worker.status === "ACTIVE" && <span className="pulse-dot online" style={{ width: 6, height: 6 }} />}
                        {worker.status}
                      </span>
                    </td>
                    <td style={{ fontSize: "0.8125rem", color: "var(--text-secondary)" }}>
                      {getRelativeTime(worker.last_heartbeat)}
                    </td>
                    <td className="mono" style={{ fontSize: "0.75rem", color: worker.current_job_id ? "var(--accent-cyan)" : "var(--text-muted)" }}>
                      {worker.current_job_id ? `${worker.current_job_id.slice(0, 8)}...` : "Idle"}
                    </td>
                    <td style={{ fontWeight: 600, color: "var(--status-succeeded)" }}>
                      {worker.completed_count}
                    </td>
                    <td style={{ fontWeight: 600, color: worker.failed_count > 0 ? "var(--status-failed)" : "var(--text-muted)" }}>
                      {worker.failed_count}
                    </td>
                    <td style={{ fontSize: "0.8125rem", color: "var(--text-muted)" }}>
                      {new Date(worker.started_at).toLocaleTimeString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
