"use client";

import React from "react";
import { QueueStats } from "@/lib/api";

interface QueuesTabProps {
  queueStats: QueueStats | null;
  onRefresh: () => void;
}

export default function QueuesTab({ queueStats, onRefresh }: QueuesTabProps) {
  const p1 = queueStats?.priority_counts?.[1] || queueStats?.priority_counts?.["1"] || 0;
  const p2 = queueStats?.priority_counts?.[2] || queueStats?.priority_counts?.["2"] || 0;
  const p3 = queueStats?.priority_counts?.[3] || queueStats?.priority_counts?.["3"] || 0;
  const inProgress = queueStats?.in_progress_count || 0;
  const total = p1 + p2 + p3 + inProgress;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "2rem" }}>
      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <h2 style={{ fontSize: "1.5rem", fontWeight: 700 }} className="brand-font">
            Queue Topology & Priority Scheduling
          </h2>
          <p style={{ fontSize: "0.875rem", color: "var(--text-secondary)" }}>
            Redis sorted-set priority dispatch layer with weighted round-robin starvation mitigation.
          </p>
        </div>
        <button className="btn btn-secondary" onClick={onRefresh}>
          ↻ Refresh Queues
        </button>
      </div>

      {/* Main Queue Stats Cards */}
      <div className="stats-grid">
        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--priority-high)" }}>
          <div className="stat-title">
            <span>High Priority (P1)</span>
            <span className="badge badge-p1">P1</span>
          </div>
          <div className="stat-value" style={{ color: "var(--priority-high)" }}>{p1}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Score = timestamp (FIFO within P1)</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--priority-medium)" }}>
          <div className="stat-title">
            <span>Medium Priority (P2)</span>
            <span className="badge badge-p2">P2</span>
          </div>
          <div className="stat-value" style={{ color: "var(--priority-medium)" }}>{p2}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Default priority (FIFO within P2)</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--priority-low)" }}>
          <div className="stat-title">
            <span>Low Priority (P3)</span>
            <span className="badge badge-p3">P3</span>
          </div>
          <div className="stat-value" style={{ color: "var(--priority-low)" }}>{p3}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Background priority (FIFO within P3)</div>
        </div>

        <div className="glass-panel stat-card" style={{ borderLeft: "3px solid var(--status-running)" }}>
          <div className="stat-title">
            <span>In-Progress Active</span>
            <span className="pulse-dot running" />
          </div>
          <div className="stat-value" style={{ color: "var(--status-running)" }}>{inProgress}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Tracked with visibility timeout</div>
        </div>
      </div>

      {/* Starvation Mitigation Architecture Explanation */}
      <div className="glass-panel" style={{ padding: "1.5rem" }}>
        <h3 style={{ fontSize: "1.125rem", fontWeight: 700, marginBottom: "0.75rem" }} className="brand-font">
          Weighted Starvation Mitigation Cycle (6 : 3 : 1)
        </h3>
        <p style={{ fontSize: "0.875rem", color: "var(--text-secondary)", marginBottom: "1.5rem" }}>
          In pure priority queues, a continuous influx of high-priority jobs completely starves medium and low-priority jobs.
          Our atomic Lua script uses a 10-slot cycle in Redis to guarantee progress across all priority levels:
        </p>

        {/* 10-slot visual cycle */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(10, 1fr)", gap: "0.5rem", marginBottom: "1.5rem" }}>
          {[0, 1, 2, 3, 4, 5].map((slot) => (
            <div
              key={slot}
              style={{
                background: "rgba(244, 63, 94, 0.15)",
                border: "1px solid rgba(244, 63, 94, 0.4)",
                borderRadius: "8px",
                padding: "0.75rem 0.5rem",
                textAlign: "center",
              }}
            >
              <div style={{ fontSize: "0.6875rem", color: "var(--text-muted)" }}>Slot {slot}</div>
              <div style={{ fontSize: "0.875rem", fontWeight: 700, color: "var(--priority-high)" }}>High</div>
              <div style={{ fontSize: "0.6875rem", color: "var(--text-muted)" }}>P1 &rarr; P2 &rarr; P3</div>
            </div>
          ))}

          {[6, 7, 8].map((slot) => (
            <div
              key={slot}
              style={{
                background: "rgba(245, 158, 11, 0.15)",
                border: "1px solid rgba(245, 158, 11, 0.4)",
                borderRadius: "8px",
                padding: "0.75rem 0.5rem",
                textAlign: "center",
              }}
            >
              <div style={{ fontSize: "0.6875rem", color: "var(--text-muted)" }}>Slot {slot}</div>
              <div style={{ fontSize: "0.875rem", fontWeight: 700, color: "var(--priority-medium)" }}>Med</div>
              <div style={{ fontSize: "0.6875rem", color: "var(--text-muted)" }}>P2 &rarr; P1 &rarr; P3</div>
            </div>
          ))}

          {[9].map((slot) => (
            <div
              key={slot}
              style={{
                background: "rgba(59, 130, 246, 0.15)",
                border: "1px solid rgba(59, 130, 246, 0.4)",
                borderRadius: "8px",
                padding: "0.75rem 0.5rem",
                textAlign: "center",
              }}
            >
              <div style={{ fontSize: "0.6875rem", color: "var(--text-muted)" }}>Slot {slot}</div>
              <div style={{ fontSize: "0.875rem", fontWeight: 700, color: "var(--priority-low)" }}>Low</div>
              <div style={{ fontSize: "0.6875rem", color: "var(--text-muted)" }}>P3 &rarr; P1 &rarr; P2</div>
            </div>
          ))}
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1rem", fontSize: "0.8125rem", color: "var(--text-secondary)" }}>
          <div style={{ background: "rgba(255,255,255,0.02)", padding: "1rem", borderRadius: "8px", border: "1px solid var(--border-glass)" }}>
            <strong style={{ color: "var(--text-primary)" }}>Zero-Latency Fallback:</strong> If slot 9 prioritizes Low, but Low is empty, the Lua script immediately checks High and Medium without sleeping or waiting.
          </div>
          <div style={{ background: "rgba(255,255,255,0.02)", padding: "1rem", borderRadius: "8px", border: "1px solid var(--border-glass)" }}>
            <strong style={{ color: "var(--text-primary)" }}>Atomic In-Progress ZSET:</strong> When claimed, the job is atomically popped from the priority set and inserted into <code>queue:&#123;name&#125;:in-progress</code> with score = <code>now + visibility_timeout</code>.
          </div>
        </div>
      </div>
    </div>
  );
}
