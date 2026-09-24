# Product Requirements

## What InferX/InferBolt Is
An open-source LLM inference benchmarking and optimization platform. Core value proposition (per README): run head-to-head benchmarks across inference engines (vLLM, SGLang, llama.cpp), measure the metrics that actually matter for inference cost/performance (TTFT, inter-token latency, throughput, KV cache hit rate, cost per million tokens), and recommend the best engine/config for a given model and workload. Self-hostable for free; cloud tier for hosted usage.

## Primary User Segments
1. **OSS / self-hosted users** — run the full stack themselves, unlimited usage, no billing relationship. Highest priority: reliability and clarity of self-host docs (currently a gap — see [known-issues.md](known-issues.md)).
2. **Cloud free tier** — evaluating the hosted product, low rate limits (10 jobs/hr, 100 API calls/hr).
3. **Cloud paid tier** — production usage of the hosted product, higher limits (100 jobs/hr, 1000 API calls/hr).
4. **Enterprise** — custom limits, likely dedicated infra/SLAs in the future (not yet built — no dedicated-tenant infrastructure exists today, only a config-level rate-limit override).

## Core Jobs-to-be-Done
- Submit a benchmark job for a model + engine + workload shape, get back timing/cost/throughput metrics.
- Get a recommendation for the best engine/config combination for a given model and workload (`recommendations` table, `cost/` model — the latter not yet implemented).
- Detect performance drift over time for a given (engine, model) pair and get alerted (Slack) when it crosses a threshold.
- (Planned) Visualize results and history via a dashboard.
- (Planned) Run an automated config sweep (Optuna) rather than manually trying configurations.

## Explicit Non-Goals (unless this changes)
- Not a general-purpose ML training platform — scope is inference benchmarking/optimization specifically.
- Not managing GPU provisioning itself beyond what the k8s `OptimizedInference` CRD/operator declares — assumes GPU infrastructure already exists in the cluster.

## Requirement Clarification Protocol
Any new feature request should be checked against: which tenant tier(s) it affects, whether it changes the rate-limit/quota model, and whether it fits the core jobs-to-be-done above or is scope creep that should be called out explicitly before building. See [../agents/product-manager.md](../agents/product-manager.md).
