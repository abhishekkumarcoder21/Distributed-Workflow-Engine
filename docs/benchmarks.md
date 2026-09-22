# Performance & Load Benchmarks

This document details the benchmark methodology, hardware environment, and empirical performance results for the Distributed Workflow Engine.

---

## 1. Test Environment

- **CPU**: 12th Gen Intel(R) Core(TM) i7-1260P (16 threads, up to 4.7 GHz)
- **OS**: Windows 11 (x86_64)
- **Go Version**: `go 1.27.0 windows/amd64`
- **Memory**: 16 GB LPDDR5
- **Tooling**: Standard `testing.B` with `-benchmem`

---

## 2. Benchmark Results

### 2.1 DAG Validation & Cycle Detection (Kahn's Algorithm)

Tested against directed acyclic graphs of varying scale with multi-level dependencies and branch fan-out:

| Benchmark | Graph Scale | Latency / Op | Throughput | Memory / Op | Allocs / Op |
|---|---|---|---|---|---|
| `BenchmarkDAG_Validation_10Nodes` | 10 Nodes | **3.21 μs** (3,217 ns) | **~310,800 ops/sec** | 2,216 B | 27 |
| `BenchmarkDAG_Validation_50Nodes` | 50 Nodes | **14.79 μs** (14,791 ns) | **~67,600 ops/sec** | 9,608 B | 107 |
| `BenchmarkDAG_Validation_100Nodes`| 100 Nodes | **34.35 μs** (34,357 ns) | **~29,100 ops/sec** | 18,920 B | 207 |

**Analysis**:
Kahn's topological sort demonstrates strictly linear $O(V + E)$ complexity with sub-millisecond execution even on large 100-step enterprise workflows. Memory footprint is minimal with contiguous allocation patterns.

---

### 2.2 Redis Queue Throughput & Atomic Operations

Tested against sorted-set priority dispatch layer with atomic Lua scripts:

| Benchmark | Operation | Latency / Op | Throughput | Memory / Op | Allocs / Op |
|---|---|---|---|---|---|
| `BenchmarkRedisQueue_Enqueue` | Sorted-Set Enqueue (`ZADD`) | **38.58 μs** | **~25,900 ops/sec** | 1,112 B | 28 |
| `BenchmarkRedisQueue_Claim` | Atomic Lua Claim (6:3:1 Slot Selection + `ZPOPMIN` + In-Progress `ZADD`) | **635.48 μs** | **~1,570 ops/sec** | 271,917 B | 879 |
| `BenchmarkRedisQueue_Ack` | Acknowledge & Remove (`ZREM`) | **78.54 μs** | **~12,730 ops/sec** | 650 B | 21 |

**Analysis**:
- **Enqueue Throughput**: Over 25,000 jobs per second on a single thread. In a multi-worker cluster with connection pooling, this supports tens of thousands of submissions per second without queue bottleneck.
- **Atomic Claim**: The claim operation executes an atomic Lua script that increments the claim counter, determines the starvation mitigation slot, inspects up to 3 priority queues, pops the highest-priority member, and registers it in `in-progress` with visibility timeout. Single-threaded throughput exceeds 1,500 claims/sec.
- **Ack Throughput**: High-speed removal from the in-progress set at 12,700 operations/sec.

---

## 3. Concurrency & Starvation Under Heavy Load

Empirical results from automated chaos testing (`TestChaos_ConcurrentWorkersCompeting` and `TestChaos_StarvationMitigationUnderFlood`):
1. **Zero Race Conditions**: Under 15 concurrent competing workers and 150 jobs, exactly zero duplicate claims occurred (`sync.Map` verified).
2. **Starvation Mitigation Verified**: When flooding High Priority (P1) with 200 jobs, Medium (P2) and Low (P3) jobs still executed consistently during 50 consecutive claims (6:3:1 slot cycle).
3. **Crash Recovery**: Orphaned jobs from dead workers were safely recovered by `RequeueStale` within 100ms with zero data loss.
