"use client";

import React from "react";
import { SystemStats, QueueStats, Job } from "@/lib/api";

interface OverviewTabProps {
  stats: SystemStats | null;
  queueStats: QueueStats | null;
  recentJobs: Job[];
  onSelectJob: (job: Job) => void;
  onNavigateTab: (tab: "workflows" | "jobs" | "workers" | "queues") => void;
}

export default function OverviewTab({
  stats,
  queueStats,
  recentJobs,
  onSelectJob,
  onNavigateTab,
}: OverviewTabProps) {
  const total = stats?.total_jobs || 0;
  const succeeded = stats?.succeeded_jobs || 0;
  const successRate = total > 0 ? Math.round((succeeded / total) * 100) : 100;

  const p1 = queueStats?.priority_counts?.[1] || queueStats?.priority_counts?.["1"] || 0;
  const p2 = queueStats?.priority_counts?.[2] || queueStats?.priority_counts?.["2"] || 0;
  const p3 = queueStats?.priority_counts?.[3] || queueStats?.priority_counts?.["3"] || 0;
  const queueTotal = p1 + p2 + p3;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "2rem" }}>
      {/* Metrics Cards Grid */}
      <div className="stats-grid">
        <div className="glass-panel stat-card">
          <div className="stat-title">
            <span>Total Jobs</span>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="m9 12 2 2 4-4"/></svg>
          </div>
          <div className="stat-value">{stats?.total_jobs ?? 0}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Lifetime processed units</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--status-running)" }}>
          <div className="stat-title">
            <span>Running Jobs</span>
            <span className="pulse-dot running" />
          </div>
          <div className="stat-value" style={{ color: "var(--status-running)" }}>
            {stats?.running_jobs ?? 0}
          </div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Actively claimed by workers</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--status-succeeded)" }}>
          <div className="stat-title">
            <span>Success Rate</span>
            <span style={{ color: "var(--status-succeeded)", fontWeight: 600 }}>{successRate}%</span>
          </div>
          <div className="stat-value" style={{ color: "var(--status-succeeded)" }}>
            {stats?.succeeded_jobs ?? 0}
          </div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Succeeded executions</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--status-failed)" }}>
          <div className="stat-title">
            <span>Failed / Dead-Letter</span>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--status-failed)" strokeWidth="2"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>
          </div>
          <div className="stat-value" style={{ color: "var(--status-failed)" }}>
            {(stats?.failed_jobs ?? 0) + (stats?.dead_letter_jobs ?? 0)}
          </div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>
            {stats?.retrying_jobs ?? 0} currently retrying
          </div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--accent-cyan)" }}>
          <div className="stat-title">
            <span>Active Workers</span>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--accent-cyan)" strokeWidth="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
          </div>
          <div className="stat-value" style={{ color: "var(--accent-cyan)" }}>
            {stats?.active_workers ?? 0}
          </div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Heartbeat active</div>
        </div>
      </div>

      {/* Middle Section: Queue Backlog & Starvation Monitor */}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1.5rem" }}>
        <div className="glass-panel" style={{ padding: "1.5rem" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1rem" }}>
            <h3 style={{ fontSize: "1.125rem", fontWeight: 600 }}>Queue Backlog by Priority</h3>
            <button className="btn btn-secondary btn-sm" onClick={() => onNavigateTab("queues")}>
              View Queues →
            </button>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
            {/* Priority 1 (High) */}
            <div>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.8125rem", marginBottom: "0.35rem" }}>
                <span style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                  <span className="badge badge-p1">P1 High</span>
                  <span style={{ color: "var(--text-muted)" }}>Target SLA: Urgent</span>
                </span>
                <span className="mono" style={{ fontWeight: 600 }}>{p1}</span>
              </div>
              <div style={{ height: "6px", background: "rgba(255,255,255,0.06)", borderRadius: "3px", overflow: "hidden" }}>
                <div style={{ width: `${queueTotal > 0 ? (p1 / queueTotal) * 100 : 0}%`, height: "100%", background: "var(--priority-high)" }} />
              </div>
            </div>

            {/* Priority 2 (Medium) */}
            <div>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.8125rem", marginBottom: "0.35rem" }}>
                <span style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                  <span className="badge badge-p2">P2 Medium</span>
                  <span style={{ color: "var(--text-muted)" }}>Default priority</span>
                </span>
                <span className="mono" style={{ fontWeight: 600 }}>{p2}</span>
              </div>
              <div style={{ height: "6px", background: "rgba(255,255,255,0.06)", borderRadius: "3px", overflow: "hidden" }}>
                <div style={{ width: `${queueTotal > 0 ? (p2 / queueTotal) * 100 : 0}%`, height: "100%", background: "var(--priority-medium)" }} />
              </div>
            </div>

            {/* Priority 3 (Low) */}
            <div>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.8125rem", marginBottom: "0.35rem" }}>
                <span style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                  <span className="badge badge-p3">P3 Low</span>
                  <span style={{ color: "var(--text-muted)" }}>Background tasks</span>
                </span>
                <span className="mono" style={{ fontWeight: 600 }}>{p3}</span>
              </div>
              <div style={{ height: "6px", background: "rgba(255,255,255,0.06)", borderRadius: "3px", overflow: "hidden" }}>
                <div style={{ width: `${queueTotal > 0 ? (p3 / queueTotal) * 100 : 0}%`, height: "100%", background: "var(--priority-low)" }} />
              </div>
            </div>
          </div>
        </div>

        {/* Engine Architecture & Invariants Panel */}
        <div className="glass-panel" style={{ padding: "1.5rem" }}>
          <h3 style={{ fontSize: "1.125rem", fontWeight: 600, marginBottom: "0.75rem" }}>Engine Invariants</h3>
          <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem", fontSize: "0.8125rem", color: "var(--text-secondary)" }}>
            <div style={{ display: "flex", gap: "0.75rem", alignItems: "flex-start" }}>
              <span style={{ color: "var(--accent-cyan)", fontWeight: 700 }}>•</span>
              <div>
                <strong style={{ color: "var(--text-primary)" }}>Starvation Mitigation (6:3:1):</strong> Redis Lua cycles across High (60%), Medium (30%), and Low (10%) queues to guarantee lower-priority progress without latency spikes.
              </div>
            </div>
            <div style={{ display: "flex", gap: "0.75rem", alignItems: "flex-start" }}>
              <span style={{ color: "var(--accent-cyan)", fontWeight: 700 }}>•</span>
              <div>
                <strong style={{ color: "var(--text-primary)" }}>At-Least-Once Delivery:</strong> Visibility timeout recovers orphaned jobs if a worker crashes before acknowledging completion.
              </div>
            </div>
            <div style={{ display: "flex", gap: "0.75rem", alignItems: "flex-start" }}>
              <span style={{ color: "var(--accent-cyan)", fontWeight: 700 }}>•</span>
              <div>
                <strong style={{ color: "var(--text-primary)" }}>DAG Orchestration:</strong> Tasks with 0 unresolved dependencies run concurrently, automatically syncing workflow status upon completion.
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Activity Table */}
      <div className="glass-panel" style={{ padding: "1.5rem" }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1rem" }}>
          <h3 style={{ fontSize: "1.125rem", fontWeight: 600 }}>Recent Jobs</h3>
          <button className="btn btn-secondary btn-sm" onClick={() => onNavigateTab("jobs")}>
            View All Jobs →
          </button>
        </div>

        <div className="table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>Job ID</th>
                <th>Type</th>
                <th>Priority</th>
                <th>Status</th>
                <th>Attempts</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {recentJobs.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ textAlign: "center", padding: "2rem", color: "var(--text-muted)" }}>
                    No jobs created yet. Click <strong>+ Submit Job</strong> to run your first job!
                  </td>
                </tr>
              ) : (
                recentJobs.slice(0, 8).map((j) => (
                  <tr key={j.id} style={{ cursor: "pointer" }} onClick={() => onSelectJob(j)}>
                    <td className="mono" style={{ color: "var(--text-primary)", fontWeight: 500 }}>
                      {j.id.slice(0, 8)}...
                    </td>
                    <td>{j.type}</td>
                    <td>
                      <span className={`badge badge-p${j.priority}`}>P{j.priority}</span>
                    </td>
                    <td>
                      <span className={`badge badge-${j.status.toLowerCase()}`}>
                        {j.status}
                      </span>
                    </td>
                    <td>{j.attempt} / {j.max_attempts}</td>
                    <td style={{ fontSize: "0.8125rem", color: "var(--text-muted)" }}>
                      {new Date(j.created_at).toLocaleTimeString()}
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
