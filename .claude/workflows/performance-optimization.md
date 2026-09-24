# Workflow: Performance Optimization

## Inputs
- A hot path, reported symptom (high latency/CPU/memory), or a proactive performance review request.

## Outputs
- Measured baseline, identified bottleneck, applied fix with measured impact.

## Step-by-Step Process
1. **Measure first.** Get a baseline (OTel trace data, `EXPLAIN ANALYZE`, a quick benchmark) — never optimize from intuition alone.
2. **Classify the bottleneck**: algorithmic (Big-O), I/O-bound (DB/network round trips), or contention-based (locks, pool exhaustion).
3. **Walk the Performance Checklist** ([../CLAUDE.md](../CLAUDE.md) §4): complexity, DB indexing/query shape, caching (Ristretto, staleness-tolerant data only), pagination, batching, async offload to River, rate limiting.
4. **Apply the smallest change addressing the actual bottleneck** — index before query rewrite before caching before architectural change, in that order of preference.
5. **Re-measure against the same baseline methodology.**
6. **State any trade-off introduced** (e.g., a cache's staleness window) explicitly — performance gains are never free.

## Review Checklist
- [ ] Baseline measured before any change
- [ ] Bottleneck classified correctly (not guessed)
- [ ] Fix addresses the actual bottleneck, not a plausible-sounding one
- [ ] Re-measured after the fix
- [ ] Trade-offs of the fix stated explicitly

## Success Criteria
A measurable, targeted improvement with before/after numbers — not a qualitative "should be faster" claim.
