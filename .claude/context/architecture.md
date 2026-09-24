# Architecture

> **Update 2026-09-10.** Three structural changes: the dead tier-based gateway path
> was deleted (taking the gRPC server with it, since it depended on
> `handler.JobHandler`); `engine_config` is now plumbed end to end; and
> `internal/agent` was added. Sections below that referenced `handler/`,
> `middleware/` or `router.go` describe files that no longer exist.

## The optimization agent (`internal/agent`, 2026-09-10)

```
inferbolt agent "<goal>"
  │
  ▼
internal/agent            campaign loop: plan -> measure -> read -> recommend
  │  planner: Anthropic Messages API (claude-opus-5, adaptive thinking)
  │  tools:   list_workers, classify_workload, historical_results,
  │           run_benchmark (the only side effect), submit_recommendation
  ▼
agent.Platform            interface declared at the consumer
  ▼
internal/cli.AgentPlatform    ordinary authenticated gateway calls
  ▼
POST /v1/jobs -> River -> orchestrator -> worker -> metrics.bench_results
```

Invariants worth not breaking:

- **The model plans; it never measures.** Every figure in a report is read off a
  benchmark result row. `Recommendation.Measured` points at the trial it refers to,
  so the headline numbers never come from the model's prose.
- **The campaign fixes model, GPU profile and workload.** Only engine and engine
  config vary, because trials measured under different workloads are not comparable.
  `run_benchmark` ignores any attempt to change them.
- **Budgets are checked before a trial starts**, never after, so a campaign never
  begins work it cannot afford to finish. Identical configs are deduped free.
- **The agent has no privileged path.** Same bearer token, same tenant-scoped
  endpoints, same rate limit as a human running the CLI. No DB handle, no access to
  the orchestrator's internal port, no shell, no filesystem.
- **A campaign that ends without a conclusion still reports** the cheapest
  error-free trial, explicitly flagged as a mechanical fallback.

**Campaigns are durable (2026-09-10).** The loop runs in the orchestrator as a
River job on its own `campaigns` queue, not in the CLI:

```
POST /v1/campaigns -> public.campaigns -> River "campaigns" queue
  -> campaigns.Worker -> agent loop -> campaigns.Platform
  -> ordinary benchmark jobs -> metrics.bench_results
  -> public.campaign_events written as each step happens
```

- `campaigns.Platform` is a second `agent.Platform` implementation that talks to
  the store and queue directly instead of over HTTP. It fixes the tenant from the
  stored row, so nothing the planner returns can widen its scope.
- The claim is a conditional `pending -> running` update, and `MaxAttempts` is 1:
  a River retry or duplicate delivery must never start a second loop against a
  campaign whose trial budget and planner tokens are already spent.
- Events carry a per-campaign `seq`, so a follower resumes with `?after_seq=`
  rather than replaying from the start or missing what it slept through.
- Cancellation is observed between trials, never mid-benchmark.
- `ANTHROPIC_API_KEY` lives on the orchestrator. No client holds it.

## The `engine_config` path (2026-09-10)

`POST /v1/jobs` -> `jobs.Job.EngineConfig` -> `public.jobs.engine_config` (migration
006) -> `queue.BenchmarkJobArgs` -> the worker's `/run` -> `engine.start(model,
config)`. Zero-valued fields are omitted on the wire, because the Python worker reads
0 as a real value. Results carry the config that produced them
(`BaseEngine.compute_result`), which is what makes a measurement attributable to a
configuration rather than just to an engine.

Before this, the Go side never sent a config and every benchmark ran with worker
defaults.


## System Shape

```
Client
  │
  ▼
cmd/gateway            HTTP :8080 + gRPC :9090
  │  middleware: Recovery → Logger → IPLimiter → Auth → TenantLimiter
  ▼
cmd/orchestrator        job scheduler — dispatches to workers, writes results, owns job lifecycle
  │
  ├── cmd/router        workload router — selects engine + worker for a job
  ├── cmd/collector     OTel + metrics collector, drift detection (internal/drift)
  ├── cmd/operator      k8s controller for the OptimizedInference CRD
  ▼
worker/ (Python)         stateless benchmark execution — vLLM / SGLang / llama.cpp / mock adapters
  ▼
PostgreSQL + TimescaleDB  durable state (jobs, recommendations, audit_log, baselines, api_keys)
                          + metrics.bench_results hypertable
River (on Postgres)       durable async job queue — benchmark dispatch, scheduled drift scans
```

Go owns all orchestration state. Python workers are stateless and horizontally scalable; they talk to the orchestrator over HTTP only — no shared memory or filesystem coupling between the two runtimes.

## Request Flow (job submission)
> **Correction (2026-07-05):** the middleware chain, handler package, and multi-tenancy model described below reflect what `cmd/gateway/main.go` actually wires up (`internal/gateway/handlers.go` + `internal/auth/{apikey.go,middleware.go}`). An earlier version of this doc described `internal/gateway/router.go` + `internal/gateway/handler/` + `internal/auth/tier.go` instead — that path is **dead code**, unreferenced by any `cmd/` binary. See [known-issues.md](known-issues.md).

1. Client `POST /v1/jobs` with `Authorization: Bearer <jwt>` (JWT carries `tenant_id` + scopes — see [dependencies.md](dependencies.md); there is no `X-API-Key` header scheme or tenant-tier concept in the live gateway).
2. Gateway middleware, in order (`cmd/gateway/main.go`): `RequestIDMiddleware` → `LoggerMiddleware` → `OTelMiddleware` → `TimeoutMiddleware(30s)` → `/health` is public; protected routes then go through `AuthMiddleware` (verifies the JWT, sets `tenant_id` + claims in context) → `RateLimitMiddleware` (flat 100 req/min per tenant) → a per-route-group `RequireScope(...)` check.
3. `Handler.CreateJob` (`internal/gateway/handlers.go`) validates the request body (model, engines allowlist, gpu_profile), persists the job via `JobStorer` (`*config.Store`, backed by Postgres), and enqueues dispatch via `JobQueuer` (`*queue.QueueClient`, River) — the gateway writes to Postgres/the queue directly here, it does not proxy through the orchestrator for job creation.
4. River delivers the job to `jobs.BenchmarkWorker.Work` (`internal/jobs/worker.go`), running inside the **orchestrator** process. It selects an idle registered worker for the requested `gpu_profile` from `internal/workers.WorkerRegistry`, marks it busy, and POSTs the job to that worker's `/run`.
5. The Python worker (`worker/main.py`) executes each requested engine adapter (`worker/engines/*.py`, dynamically selected per job — not per worker process) sequentially, computes results, and POSTs them back to the orchestrator's `/internal/results`.
6. The orchestrator writes results to `metrics.bench_results` (TimescaleDB) via `internal/metrics`, updates job state to `completed`, and saves a `recommendations` row. The collector's drift detector (`internal/drift`) separately compares fresh results against `baselines` and fires Slack alerts (`internal/drift/notifier.go`) on significant deviation.

## Worker Registration & Dispatch
Python workers are **not** provisioned by the orchestrator — they self-register: on startup, `worker/main.py` POSTs to the orchestrator's `/internal/workers/register` with its own generated ID, callback URL, and `GPU_PROFILE`, then periodically heartbeats. `internal/workers.WorkerRegistry` tracks status (`idle`/`busy`/`offline`) in memory and evicts stale entries. `jobs.BenchmarkWorker.Work` only ever dispatches to an already-registered **idle** worker matching the job's `gpu_profile` — if none exists, the job fails with "no available worker." `GET /internal/workers` (added alongside `inferbolt run`) lists the current registry snapshot for external callers that need to check availability without dispatching a job — see `memory/completed-features.md`.

`inferbolt run` (see [../commands/deploy.md](../commands/deploy.md)-adjacent CLI behavior, and `cmd/inferbolt/main.go`'s `newRunCmd`) automates what used to require a human to do all of this by hand: it brings up `docker-compose` if the gateway is unreachable, resolves or bootstraps an API key, and — if no idle worker is registered for the requested GPU profile — spawns `worker/main.py` as a local subprocess and waits for it to register before submitting the job.

**Remote GPU worker via `--gpu-host` (2026-07-05):** since worker registration is just an HTTP callback, a worker doesn't have to run on the same machine as the CLI — `spawnRemoteWorkerSSH` (`cmd/inferbolt/main.go`) SSHes into `--gpu-host` and starts `worker/main.py` there, using **one SSH session carrying both a local (`-L`) and reverse (`-R`) port forward** so the local orchestrator and the remote worker can reach each other despite being on different networks: `-L` lets the orchestrator's container reach the remote worker via `host.docker.internal` (hence the `extra_hosts: host-gateway` entry added to the orchestrator's compose service), `-R` lets the remote worker reach back to the local orchestrator via `localhost`. This does **not** provision the remote environment — `--remote-dir` must already have the repo checked out with `worker/`'s dependencies installed for that hardware.

**Dashboard (2026-07-05):** `GET /dashboard` on the gateway serves a single embedded static HTML/JS page (`internal/gateway/static/dashboard.html`, via Go `embed`) — no build step, no framework. It's read-only and calls the existing authenticated `/v1/jobs`, `/v1/jobs/{id}/results`, and the new `/v1/workers` (a gateway-side proxy to the orchestrator's `/internal/workers`, added so browser clients never talk to the orchestrator's unauthenticated internal port directly) using a bearer token entered client-side.

## Key Interfaces
- `agent.Platform` (`internal/agent/platform.go`) — what the agent may do to the
  platform, declared at the consumer, satisfied by `internal/cli.AgentPlatform`.
- `agent.messageCreator` (`internal/agent/agent.go`) — the one SDK method the loop
  needs, so campaigns are testable against a scripted planner with no API key.
- `engine.Engine` (`internal/engine/engine.go`) — a Go-side abstraction over benchmark engines that currently has no confirmed live caller; verify before relying on it (may be part of the same orphaned path as `handler/`/`router.go`, or simply not yet wired up — worth a follow-up check, not yet done).
- `BaseEngine` (`worker/engines/base.py`) — the actual, live Python-side engine contract (`name`, `start`, `teardown`, `get_gpu_memory_mb`, `get_kv_cache_hit_rate`, `_send_request`), implemented by `vllm_engine.py`, `sglang_engine.py`, `llamacpp_engine.py`, `ollama_engine.py`, and `mock_engine.py`.
- `JobStorer` / `JobQueuer` / `MetricsReader` / `DBPinger` (`internal/gateway/interfaces.go`) — defined at the gateway (the consumer), satisfied by `*config.Store`, `*queue.QueueClient`, `*metrics.MetricsWriter`, `*pgxpool.Pool` respectively. Note: this `interfaces.go` file is shared with the dead `handler/` subpackage's `OrchestratorClient` interface, which is *not* what `handlers.go` actually uses — don't confuse the two when reading this file.

## Multi-Tenancy Model
Every authenticated request carries `tenant_id` in context from `AuthMiddleware` onward (via JWT claims) — downstream code reads it with `iauth.MustGetTenantID(ctx)`, never re-derives it. There is **no tenant tier** in the live system — every tenant gets the same flat rate limit; access is governed by JWT **scopes**, not a tier. See [dependencies.md](dependencies.md) for the corrected model (this replaces an earlier, incorrect tier-based description here).

## Deployment Topologies
- **Local dev**: `docker-compose.yml` — Postgres+TimescaleDB, and one instance each of `gateway`, `orchestrator`, `router`, `collector`. (The `gateway` service and its `Dockerfile.gateway` were added 2026-07-05 — previously the compose file didn't include the gateway at all, and it had to be run manually via `go run ./cmd/gateway`.)
- **Production**: Kubernetes via Helm, with the `OptimizedInference` CRD letting users declare a benchmark/optimization run declaratively, reconciled by `cmd/operator`.

## Known Structural Gaps
See [known-issues.md](known-issues.md) for: the two-parallel-gateway-implementations finding (one dead), the README/implementation drift (ClickHouse/Redis mentioned in README, not actually used), and other open architectural questions. See [roadmap.md](roadmap.md) for what's explicitly not yet built (dashboard UI, cost model, Optuna sweep, SGLang/llama.cpp adapters — note the adapter *files* exist, but confirm they're actually functional/tested before assuming "not yet built" is wrong).
