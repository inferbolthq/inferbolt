# /performance — Performance Analysis & Optimization

## Purpose
Analyze and improve the performance of a specific code path using [../CLAUDE.md](../CLAUDE.md) §4 (Performance Checklist) as the baseline, with measurement before and after — never optimize blind.

## Inputs
- Target: endpoint, query, function, or reported symptom (high latency, high CPU, memory growth).
- Any available measurements (p50/p99 latency, profiling output, `EXPLAIN ANALYZE` output).

## Outputs
- A measured baseline (or an explicit statement that no measurement exists yet and one is needed first).
- Identified bottleneck with reasoning (algorithmic complexity, query plan, allocation pattern, lock contention).
- A fix with expected impact, and a way to verify the impact post-change.

## Workflow
1. Measure first. If there's no baseline, get one (add a benchmark, run `EXPLAIN ANALYZE`, check existing OTel traces) before proposing a fix — don't optimize based on intuition alone.
2. Identify the actual bottleneck: is it algorithmic (Big-O), I/O-bound (DB/network round trips), or resource contention (locks, connection pool exhaustion)?
3. Check the Performance Checklist categories relevant to the target: complexity, DB indexing/query shape, caching opportunity (ristretto), pagination, batching, async offload to River.
4. Propose the smallest change that addresses the actual bottleneck — don't rewrite the whole path when one query needs an index.
5. Re-measure after the change against the same baseline methodology.
6. State the trade-off if the optimization adds complexity (e.g., a cache introduces a staleness window) — perf gains are never free.

## Examples
```
/performance investigate p99 latency spike on POST /v1/jobs
/performance optimize the drift detector's baseline query against TimescaleDB
/performance reduce allocations in the gateway request-handling hot path
```

## Best practices
- "Feels slow" is not a target — get a number, set a target, measure against it.
- Prefer fixing the query/index over adding a cache; caches trade correctness/freshness for speed and should be a deliberate choice, not a first resort.
- Always check whether the bottleneck is actually on the critical path the user cares about — optimizing a cold path is wasted effort.
