"use client";

import React, { useState, useEffect, useCallback } from "react";
import Navbar from "@/components/Navbar";
import OverviewTab from "@/components/OverviewTab";
import WorkflowsTab from "@/components/WorkflowsTab";
import JobsTab from "@/components/JobsTab";
import WorkersTab from "@/components/WorkersTab";
import QueuesTab from "@/components/QueuesTab";
import CreateJobModal from "@/components/CreateJobModal";
import CreateWorkflowModal from "@/components/CreateWorkflowModal";
import {
  SystemStats,
  QueueStats,
  Job,
  Workflow,
  Worker,
  fetchStats,
  fetchQueueStats,
  fetchJobs,
  fetchWorkflows,
  fetchWorkers,
  checkBackendHealth,
} from "@/lib/api";

export default function DashboardPage() {
  const [activeTab, setActiveTab] = useState<
    "overview" | "workflows" | "jobs" | "workers" | "queues"
  >("overview");

  const [isOnline, setIsOnline] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState(2000); // 2s default
  const [loading, setLoading] = useState(true);

  const [stats, setStats] = useState<SystemStats | null>(null);
  const [queueStats, setQueueStats] = useState<QueueStats | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [totalJobs, setTotalJobs] = useState(0);
  const [statusFilter, setStatusFilter] = useState("ALL");
  const [priorityFilter, setPriorityFilter] = useState<number | undefined>(undefined);

  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [workers, setWorkers] = useState<Worker[]>([]);

  const [isCreateJobOpen, setIsCreateJobOpen] = useState(false);
  const [isCreateWorkflowOpen, setIsCreateWorkflowOpen] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const healthy = await checkBackendHealth();
      setIsOnline(healthy);

      const [s, qs, jData, wfData, wData] = await Promise.allSettled([
        fetchStats(),
        fetchQueueStats("default"),
        fetchJobs({
          status: statusFilter,
          priority: priorityFilter,
          limit: 50,
        }),
        fetchWorkflows({ limit: 50 }),
        fetchWorkers(),
      ]);

      if (s.status === "fulfilled") setStats(s.value);
      if (qs.status === "fulfilled") setQueueStats(qs.value);
      if (jData.status === "fulfilled") {
        setJobs(jData.value.jobs || []);
        setTotalJobs(jData.value.total || 0);
      }
      if (wfData.status === "fulfilled") {
        setWorkflows(wfData.value.workflows || []);
      }
      if (wData.status === "fulfilled") {
        setWorkers(Array.isArray(wData.value) ? wData.value : []);
      }
    } catch (err) {
      console.error("Failed to load dashboard data:", err);
    } finally {
      setLoading(false);
    }
  }, [statusFilter, priorityFilter]);

  // Initial load & polling loop
  useEffect(() => {
    loadData();

    if (refreshInterval <= 0) return;
    const timer = setInterval(() => {
      loadData();
    }, refreshInterval);

    return () => clearInterval(timer);
  }, [loadData, refreshInterval]);

  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <Navbar
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        isOnline={isOnline}
        refreshInterval={refreshInterval}
        setRefreshInterval={setRefreshInterval}
        onRefresh={loadData}
        onOpenCreateJob={() => setIsCreateJobOpen(true)}
        onOpenCreateWorkflow={() => setIsCreateWorkflowOpen(true)}
      />

      <main style={{ flex: 1, maxWidth: "1440px", width: "100%", margin: "0 auto", padding: "2rem" }}>
        {loading && !stats ? (
          <div style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "300px", color: "var(--text-muted)" }}>
            Connecting to control plane...
          </div>
        ) : (
          <>
            {activeTab === "overview" && (
              <OverviewTab
                stats={stats}
                queueStats={queueStats}
                recentJobs={jobs}
                onSelectJob={(j) => {
                  setActiveTab("jobs");
                }}
                onNavigateTab={setActiveTab}
              />
            )}

            {activeTab === "workflows" && (
              <WorkflowsTab
                workflows={workflows}
                onOpenCreateWorkflow={() => setIsCreateWorkflowOpen(true)}
                onRefresh={loadData}
              />
            )}

            {activeTab === "jobs" && (
              <JobsTab
                jobs={jobs}
                total={totalJobs}
                statusFilter={statusFilter}
                setStatusFilter={setStatusFilter}
                priorityFilter={priorityFilter}
                setPriorityFilter={setPriorityFilter}
                onRefresh={loadData}
                onOpenCreateJob={() => setIsCreateJobOpen(true)}
              />
            )}

            {activeTab === "workers" && (
              <WorkersTab workers={workers} onRefresh={loadData} />
            )}

            {activeTab === "queues" && (
              <QueuesTab queueStats={queueStats} onRefresh={loadData} />
            )}
          </>
        )}
      </main>

      <CreateJobModal
        isOpen={isCreateJobOpen}
        onClose={() => setIsCreateJobOpen(false)}
        onJobCreated={loadData}
      />

      <CreateWorkflowModal
        isOpen={isCreateWorkflowOpen}
        onClose={() => setIsCreateWorkflowOpen(false)}
        onWorkflowCreated={loadData}
      />
    </div>
  );
}
