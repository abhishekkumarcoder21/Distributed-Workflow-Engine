"use client";

import React, { useState, useEffect } from "react";
import { Workflow, WorkflowTask, fetchWorkflow, cancelWorkflow } from "@/lib/api";

interface WorkflowsTabProps {
  workflows: Workflow[];
  onOpenCreateWorkflow: () => void;
  onRefresh: () => void;
}

export default function WorkflowsTab({
  workflows,
  onOpenCreateWorkflow,
  onRefresh,
}: WorkflowsTabProps) {
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(
    workflows.length > 0 ? workflows[0].id : null
  );
  const [selectedWorkflow, setSelectedWorkflow] = useState<Workflow | null>(null);
  const [loadingDetails, setLoadingDetails] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  // When selectedWorkflowId changes, fetch full details with tasks
  useEffect(() => {
    if (!selectedWorkflowId) return;
    let isCurrent = true;
    setLoadingDetails(true);
    fetchWorkflow(selectedWorkflowId)
      .then((wf) => {
        if (isCurrent) setSelectedWorkflow(wf);
      })
      .catch((err) => {
        if (isCurrent) setActionError(err.message);
      })
      .finally(() => {
        if (isCurrent) setLoadingDetails(false);
      });
    return () => {
      isCurrent = false;
    };
  }, [selectedWorkflowId]);

  const handleCancel = async (id: string) => {
    try {
      await cancelWorkflow(id);
      onRefresh();
      if (selectedWorkflowId === id) {
        const updated = await fetchWorkflow(id);
        setSelectedWorkflow(updated);
      }
    } catch (err: any) {
      setActionError(err.message);
    }
  };

  // Helper to arrange tasks into topological levels for DAG visualization
  const computeDAGLevels = (tasks: WorkflowTask[]): WorkflowTask[][] => {
    if (!tasks || tasks.length === 0) return [];

    const levels: Map<string, number> = new Map();
    const taskMap: Map<string, WorkflowTask> = new Map();
    tasks.forEach((t) => taskMap.set(t.name, t));

    // Compute level recursively
    const getLevel = (taskName: string, visited: Set<string>): number => {
      if (levels.has(taskName)) return levels.get(taskName)!;
      if (visited.has(taskName)) return 0; // Avoid infinite loop in malformed data
      visited.add(taskName);

      const task = taskMap.get(taskName);
      if (!task || !task.dependencies || task.dependencies.length === 0) {
        levels.set(taskName, 0);
        return 0;
      }

      let maxDepLevel = -1;
      for (const dep of task.dependencies) {
        const depLevel = getLevel(dep, new Set(visited));
        if (depLevel > maxDepLevel) maxDepLevel = depLevel;
      }

      const myLevel = maxDepLevel + 1;
      levels.set(taskName, myLevel);
      return myLevel;
    };

    tasks.forEach((t) => getLevel(t.name, new Set()));

    // Group into array of levels
    let maxLevel = 0;
    levels.forEach((lvl) => {
      if (lvl > maxLevel) maxLevel = lvl;
    });

    const result: WorkflowTask[][] = Array.from({ length: maxLevel + 1 }, () => []);
    tasks.forEach((t) => {
      const lvl = levels.get(t.name) ?? 0;
      result[lvl].push(t);
    });

    return result;
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "2rem" }}>
      {/* Top Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <h2 style={{ fontSize: "1.5rem", fontWeight: 700 }} className="brand-font">
            Workflows & DAG Orchestration
          </h2>
          <p style={{ fontSize: "0.875rem", color: "var(--text-secondary)" }}>
            Multi-step execution pipelines with dependency graph validation and automated task progression.
          </p>
        </div>
        <button className="btn btn-primary" onClick={onOpenCreateWorkflow}>
          + New Workflow
        </button>
      </div>

      {actionError && (
        <div style={{ padding: "0.75rem 1rem", background: "var(--status-failed-bg)", border: "1px solid rgba(244,63,94,0.3)", borderRadius: "8px", color: "var(--status-failed)", fontSize: "0.875rem" }}>
          {actionError}
        </div>
      )}

      {/* Main Grid: Workflows List (Left) + DAG Visualizer (Right) */}
      <div style={{ display: "grid", gridTemplateColumns: "380px 1fr", gap: "1.5rem", alignItems: "start" }}>
        {/* Workflows List */}
        <div className="glass-panel" style={{ padding: "1.25rem", maxHeight: "800px", overflowY: "auto" }}>
          <div style={{ fontSize: "0.875rem", fontWeight: 600, color: "var(--text-muted)", textTransform: "uppercase", marginBottom: "1rem" }}>
            All Workflows ({workflows.length})
          </div>

          {workflows.length === 0 ? (
            <div style={{ textAlign: "center", padding: "2rem", color: "var(--text-muted)", fontSize: "0.875rem" }}>
              No workflows created yet. Click <strong>+ New Workflow</strong> to launch a DAG pipeline!
            </div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
              {workflows.map((wf) => {
                const isSelected = wf.id === selectedWorkflowId;
                return (
                  <div
                    key={wf.id}
                    onClick={() => setSelectedWorkflowId(wf.id)}
                    style={{
                      padding: "1rem",
                      borderRadius: "10px",
                      background: isSelected ? "rgba(6, 182, 212, 0.1)" : "rgba(255, 255, 255, 0.02)",
                      border: isSelected ? "1px solid var(--accent-cyan)" : "1px solid var(--border-glass)",
                      cursor: "pointer",
                      transition: "all 0.15s ease",
                    }}
                  >
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "0.5rem" }}>
                      <span style={{ fontWeight: 600, color: isSelected ? "var(--accent-cyan)" : "var(--text-primary)" }}>
                        {wf.name}
                      </span>
                      <span className={`badge badge-${wf.status.toLowerCase()}`}>
                        {wf.status}
                      </span>
                    </div>

                    <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", display: "flex", justifyContent: "space-between" }}>
                      <span className="mono">{wf.id.slice(0, 8)}...</span>
                      <span>{new Date(wf.created_at).toLocaleTimeString()}</span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* DAG Visualizer Panel */}
        <div className="glass-panel" style={{ padding: "1.5rem" }}>
          {loadingDetails ? (
            <div style={{ padding: "3rem", textAlign: "center", color: "var(--text-muted)" }}>
              Loading DAG state...
            </div>
          ) : selectedWorkflow ? (
            <div>
              {/* Workflow Details Bar */}
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "1.5rem", paddingBottom: "1rem", borderBottom: "1px solid var(--border-glass)" }}>
                <div>
                  <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "0.25rem" }}>
                    <h3 style={{ fontSize: "1.25rem", fontWeight: 700 }} className="brand-font">
                      {selectedWorkflow.name}
                    </h3>
                    <span className={`badge badge-${selectedWorkflow.status.toLowerCase()}`}>
                      {selectedWorkflow.status}
                    </span>
                  </div>
                  <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", display: "flex", gap: "1rem" }}>
                    <span>ID: <span className="mono">{selectedWorkflow.id}</span></span>
                    <span>Started: {selectedWorkflow.started_at ? new Date(selectedWorkflow.started_at).toLocaleTimeString() : "Pending"}</span>
                    <span>Completed: {selectedWorkflow.completed_at ? new Date(selectedWorkflow.completed_at).toLocaleTimeString() : "—"}</span>
                  </div>
                  {selectedWorkflow.error && (
                    <div style={{ marginTop: "0.5rem", fontSize: "0.8125rem", color: "var(--status-failed)" }}>
                      Error: {selectedWorkflow.error}
                    </div>
                  )}
                </div>

                <div style={{ display: "flex", gap: "0.5rem" }}>
                  {selectedWorkflow.status === "RUNNING" || selectedWorkflow.status === "PENDING" ? (
                    <button
                      className="btn btn-danger btn-sm"
                      onClick={() => handleCancel(selectedWorkflow.id)}
                    >
                      Cancel Workflow
                    </button>
                  ) : null}
                  <button
                    className="btn btn-secondary btn-sm"
                    onClick={() => {
                      if (selectedWorkflowId) {
                        fetchWorkflow(selectedWorkflowId).then(setSelectedWorkflow);
                      }
                    }}
                  >
                    ↻ Refresh
                  </button>
                </div>
              </div>

              {/* Interactive Visual DAG Graph */}
              <div style={{ marginBottom: "1.5rem" }}>
                <div style={{ fontSize: "0.875rem", fontWeight: 600, marginBottom: "0.75rem", color: "var(--text-secondary)" }}>
                  DAG Dependency Graph (Left &rarr; Right Execution Flow)
                </div>

                <div className="dag-graph">
                  {computeDAGLevels(selectedWorkflow.tasks || []).map((levelTasks, levelIdx) => (
                    <React.Fragment key={levelIdx}>
                      {levelIdx > 0 && (
                        <div className="dag-arrow">
                          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                            <line x1="5" y1="12" x2="19" y2="12" />
                            <polyline points="12 5 19 12 12 19" />
                          </svg>
                        </div>
                      )}
                      <div className="dag-level">
                        <div style={{ fontSize: "0.6875rem", textTransform: "uppercase", color: "var(--text-muted)", fontWeight: 600, textAlign: "center", marginBottom: "0.25rem" }}>
                          {levelIdx === 0 ? "Root Tasks" : `Stage ${levelIdx + 1}`}
                        </div>
                        {levelTasks.map((task) => (
                          <div key={task.id} className="dag-node">
                            <div className="dag-node-header">
                              <span style={{ fontWeight: 600, fontSize: "0.875rem" }}>
                                {task.name}
                              </span>
                              <span className={`badge badge-${task.status.toLowerCase()}`}>
                                {task.status}
                              </span>
                            </div>

                            <div style={{ fontSize: "0.75rem", color: "var(--text-muted)", marginBottom: "0.5rem" }}>
                              Type: <span style={{ color: "var(--text-primary)" }}>{task.type}</span>
                            </div>

                            {task.dependencies && task.dependencies.length > 0 && (
                              <div style={{ fontSize: "0.6875rem", color: "var(--text-muted)", background: "rgba(255,255,255,0.03)", padding: "4px 8px", borderRadius: "6px", marginBottom: "0.5rem" }}>
                                Depends on: {task.dependencies.join(", ")}
                              </div>
                            )}

                            {task.job_id && (
                              <div style={{ fontSize: "0.6875rem", color: "var(--accent-cyan)" }}>
                                Job ID: <span className="mono">{task.job_id.slice(0, 8)}...</span>
                              </div>
                            )}

                            {task.error && (
                              <div style={{ marginTop: "0.35rem", fontSize: "0.6875rem", color: "var(--status-failed)" }}>
                                {task.error}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    </React.Fragment>
                  ))}
                </div>
              </div>

              {/* Tasks List Table */}
              <div>
                <div style={{ fontSize: "0.875rem", fontWeight: 600, marginBottom: "0.75rem", color: "var(--text-secondary)" }}>
                  Task Breakdown
                </div>
                <div className="table-container">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Task Name</th>
                        <th>Type</th>
                        <th>Status</th>
                        <th>Dependencies</th>
                        <th>Job ID</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(selectedWorkflow.tasks || []).map((t) => (
                        <tr key={t.id}>
                          <td style={{ fontWeight: 600, color: "var(--text-primary)" }}>{t.name}</td>
                          <td>{t.type}</td>
                          <td>
                            <span className={`badge badge-${t.status.toLowerCase()}`}>
                              {t.status}
                            </span>
                          </td>
                          <td style={{ fontSize: "0.8125rem", color: "var(--text-muted)" }}>
                            {t.dependencies && t.dependencies.length > 0 ? t.dependencies.join(", ") : "None (Root)"}
                          </td>
                          <td className="mono" style={{ fontSize: "0.75rem", color: "var(--accent-cyan)" }}>
                            {t.job_id ? `${t.job_id.slice(0, 8)}...` : "—"}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          ) : (
            <div style={{ padding: "3rem", textAlign: "center", color: "var(--text-muted)" }}>
              Select a workflow from the list to view its DAG and execution details.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
