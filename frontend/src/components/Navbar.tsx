"use client";

import React from "react";

interface NavbarProps {
  activeTab: "overview" | "workflows" | "jobs" | "workers" | "queues";
  setActiveTab: (tab: "overview" | "workflows" | "jobs" | "workers" | "queues") => void;
  isOnline: boolean;
  refreshInterval: number;
  setRefreshInterval: (interval: number) => void;
  onRefresh: () => void;
  onOpenCreateJob: () => void;
  onOpenCreateWorkflow: () => void;
}

export default function Navbar({
  activeTab,
  setActiveTab,
  isOnline,
  refreshInterval,
  setRefreshInterval,
  onRefresh,
  onOpenCreateJob,
  onOpenCreateWorkflow,
}: NavbarProps) {
  return (
    <header className="header">
      <div className="header-inner">
        <div style={{ display: "flex", alignItems: "center", gap: "2.5rem" }}>
          <div className="brand">
            <div className="brand-icon">
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <path d="m4.93 4.93 4.24 4.24" />
                <path d="m14.83 9.17 4.24-4.24" />
                <path d="m14.83 14.83 4.24 4.24" />
                <path d="m9.17 14.83-4.24 4.24" />
                <circle cx="12" cy="12" r="4" />
              </svg>
            </div>
            <div>
              <div style={{ fontSize: "1.125rem", fontWeight: 700 }} className="brand-font">
                Workflow Engine
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: "0.5rem", fontSize: "0.75rem", color: "var(--text-muted)" }}>
                <span className={`pulse-dot ${isOnline ? "online" : "failed"}`} />
                <span>{isOnline ? "API Connected (:8080)" : "Disconnected"}</span>
              </div>
            </div>
          </div>

          <nav className="nav-tabs">
            <button
              className={`nav-tab ${activeTab === "overview" ? "active" : ""}`}
              onClick={() => setActiveTab("overview")}
            >
              Overview
            </button>
            <button
              className={`nav-tab ${activeTab === "workflows" ? "active" : ""}`}
              onClick={() => setActiveTab("workflows")}
            >
              Workflows (DAG)
            </button>
            <button
              className={`nav-tab ${activeTab === "jobs" ? "active" : ""}`}
              onClick={() => setActiveTab("jobs")}
            >
              Jobs
            </button>
            <button
              className={`nav-tab ${activeTab === "workers" ? "active" : ""}`}
              onClick={() => setActiveTab("workers")}
            >
              Workers
            </button>
            <button
              className={`nav-tab ${activeTab === "queues" ? "active" : ""}`}
              onClick={() => setActiveTab("queues")}
            >
              Queues
            </button>
          </nav>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
          {/* Auto Refresh dropdown */}
          <div style={{ display: "flex", alignItems: "center", gap: "0.35rem", background: "rgba(255,255,255,0.04)", padding: "4px 8px", borderRadius: "8px", border: "1px solid var(--border-glass)" }}>
            <span style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Poll:</span>
            <select
              value={refreshInterval}
              onChange={(e) => setRefreshInterval(Number(e.target.value))}
              style={{ background: "transparent", border: "none", color: "var(--text-primary)", fontSize: "0.75rem", outline: "none", cursor: "pointer" }}
            >
              <option value={2000} style={{ background: "#0d1322" }}>2s</option>
              <option value={5000} style={{ background: "#0d1322" }}>5s</option>
              <option value={10000} style={{ background: "#0d1322" }}>10s</option>
              <option value={0} style={{ background: "#0d1322" }}>Off</option>
            </select>
            <button
              onClick={onRefresh}
              className="btn btn-secondary btn-sm"
              title="Refresh now"
              style={{ padding: "2px 6px" }}
            >
              ↻
            </button>
          </div>

          <button className="btn btn-secondary" onClick={onOpenCreateJob}>
            + Submit Job
          </button>
          <button className="btn btn-primary" onClick={onOpenCreateWorkflow}>
            + Create Workflow
          </button>
        </div>
      </div>
    </header>
  );
}
