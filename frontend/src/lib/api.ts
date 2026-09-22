export type JobStatus =
  | "QUEUED"
  | "RUNNING"
  | "SUCCEEDED"
  | "FAILED"
  | "RETRYING"
  | "CANCELLED"
  | "TIMEOUT"
  | "DEAD_LETTER";

export interface Job {
  id: string;
  type: string;
  payload: any;
  status: JobStatus;
  priority: number;
  max_attempts: number;
  attempt: number;
  idempotency_key?: string;
  error?: string;
  worker_id?: string;
  queue: string;
  run_at: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export type WorkflowStatus =
  | "PENDING"
  | "RUNNING"
  | "SUCCEEDED"
  | "FAILED"
  | "CANCELLED";

export type WorkflowTaskStatus =
  | "PENDING"
  | "RUNNING"
  | "SUCCEEDED"
  | "FAILED"
  | "CANCELLED";

export interface WorkflowTask {
  id: string;
  workflow_id: string;
  job_id?: string;
  name: string;
  type: string;
  payload: any;
  status: WorkflowTaskStatus;
  dependencies: string[];
  error?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  updated_at: string;
}

export interface Workflow {
  id: string;
  name: string;
  status: WorkflowStatus;
  error?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  updated_at: string;
  tasks?: WorkflowTask[];
}

export interface Worker {
  id: string;
  status: "ACTIVE" | "DRAINING" | "OFFLINE";
  last_heartbeat: string;
  started_at: string;
  current_job_id?: string;
  completed_count: number;
  failed_count: number;
}

export interface QueueStats {
  queue_name: string;
  priority_counts: Record<string, number>;
  in_progress_count: number;
}

export interface SystemStats {
  total_jobs: number;
  queued_jobs: number;
  running_jobs: number;
  succeeded_jobs: number;
  failed_jobs: number;
  retrying_jobs: number;
  dead_letter_jobs: number;
  cancelled_jobs: number;
  active_workers: number;
}

const API_BASE = "";

interface ApiEnvelope {
  data?: any;
  error?: string;
  meta?: { total: number; limit: number; offset: number };
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let errMessage = `HTTP ${res.status}: ${res.statusText}`;
    try {
      const errObj = await res.json();
      if (errObj.error) errMessage = errObj.error;
    } catch {
      // fallback to statusText
    }
    throw new Error(errMessage);
  }
  const json: ApiEnvelope = await res.json();
  return (json.data !== undefined ? json.data : json) as T;
}

async function handleResponseWithMeta(res: Response): Promise<{ data: any; meta?: { total: number; limit: number; offset: number } }> {
  if (!res.ok) {
    let errMessage = `HTTP ${res.status}: ${res.statusText}`;
    try {
      const errObj = await res.json();
      if (errObj.error) errMessage = errObj.error;
    } catch {
      // fallback
    }
    throw new Error(errMessage);
  }
  const json: ApiEnvelope = await res.json();
  return { data: json.data, meta: json.meta };
}

export async function fetchStats(): Promise<SystemStats> {
  const res = await fetch(`${API_BASE}/api/v1/stats`, { cache: "no-store" });
  return handleResponse<SystemStats>(res);
}

export async function fetchJobs(params?: {
  status?: string;
  priority?: number;
  limit?: number;
  offset?: number;
}): Promise<{ jobs: Job[]; total: number; limit: number; offset: number }> {
  const query = new URLSearchParams();
  if (params?.status && params.status !== "ALL") query.set("status", params.status);
  if (params?.priority) query.set("priority", params.priority.toString());
  if (params?.limit) query.set("limit", params.limit.toString());
  if (params?.offset) query.set("offset", params.offset.toString());

  const res = await fetch(`${API_BASE}/api/v1/jobs?${query.toString()}`, { cache: "no-store" });
  const { data, meta } = await handleResponseWithMeta(res);
  return {
    jobs: Array.isArray(data) ? data : [],
    total: meta?.total ?? 0,
    limit: meta?.limit ?? 50,
    offset: meta?.offset ?? 0,
  };
}

export async function fetchJob(id: string): Promise<Job> {
  const res = await fetch(`${API_BASE}/api/v1/jobs/${id}`, { cache: "no-store" });
  return handleResponse<Job>(res);
}

export async function createJob(data: {
  type: string;
  payload?: any;
  priority?: number;
  max_attempts?: number;
  idempotency_key?: string;
  queue?: string;
}): Promise<Job> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (data.idempotency_key) {
    headers["Idempotency-Key"] = data.idempotency_key;
  }
  const res = await fetch(`${API_BASE}/api/v1/jobs`, {
    method: "POST",
    headers,
    body: JSON.stringify(data),
  });
  return handleResponse<Job>(res);
}

export async function cancelJob(id: string): Promise<{ status: string }> {
  const res = await fetch(`${API_BASE}/api/v1/jobs/${id}/cancel`, { method: "POST" });
  return handleResponse(res);
}

export async function retryJob(id: string): Promise<{ status: string }> {
  const res = await fetch(`${API_BASE}/api/v1/jobs/${id}/retry`, { method: "POST" });
  return handleResponse(res);
}

export async function fetchWorkflows(params?: {
  status?: string;
  limit?: number;
  offset?: number;
}): Promise<{ workflows: Workflow[]; total: number; limit: number; offset: number }> {
  const query = new URLSearchParams();
  if (params?.status && params.status !== "ALL") query.set("status", params.status);
  if (params?.limit) query.set("limit", params.limit.toString());
  if (params?.offset) query.set("offset", params.offset.toString());

  const res = await fetch(`${API_BASE}/api/v1/workflows?${query.toString()}`, { cache: "no-store" });
  // The workflows list endpoint returns { data: { workflows: [...], total, limit, offset } }
  const result = await handleResponse<any>(res);
  // Handle both shapes: if result has .workflows, use it; otherwise treat result as the array
  if (result && result.workflows) {
    return {
      workflows: result.workflows,
      total: result.total ?? result.workflows.length,
      limit: result.limit ?? 20,
      offset: result.offset ?? 0,
    };
  }
  // Fallback: result is the array directly
  const arr = Array.isArray(result) ? result : [];
  return { workflows: arr, total: arr.length, limit: 20, offset: 0 };
}

export async function fetchWorkflow(id: string): Promise<Workflow> {
  const res = await fetch(`${API_BASE}/api/v1/workflows/${id}`, { cache: "no-store" });
  return handleResponse<Workflow>(res);
}

export async function createWorkflow(data: {
  name: string;
  tasks: Array<{
    name: string;
    type: string;
    payload?: any;
    priority?: number;
    dependencies?: string[];
  }>;
}): Promise<Workflow> {
  const res = await fetch(`${API_BASE}/api/v1/workflows`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  return handleResponse<Workflow>(res);
}

export async function cancelWorkflow(id: string): Promise<{ status: string }> {
  const res = await fetch(`${API_BASE}/api/v1/workflows/${id}/cancel`, { method: "POST" });
  return handleResponse(res);
}

export async function fetchWorkers(): Promise<Worker[]> {
  const res = await fetch(`${API_BASE}/api/v1/workers`, { cache: "no-store" });
  // The backend returns { data: [workers] }, handleResponse unwraps to the array
  const result = await handleResponse<Worker[] | any>(res);
  return Array.isArray(result) ? result : [];
}

export async function fetchQueueStats(queue = "default"): Promise<QueueStats> {
  const res = await fetch(`${API_BASE}/api/v1/queues?queue=${encodeURIComponent(queue)}`, { cache: "no-store" });
  return handleResponse<QueueStats>(res);
}

export async function checkBackendHealth(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/health`, { cache: "no-store" });
    return res.ok;
  } catch {
    return false;
  }
}

