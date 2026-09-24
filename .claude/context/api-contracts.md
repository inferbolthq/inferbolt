# API Contracts

Source of truth for HTTP contracts. Update this file in the same change that alters a handler.

> **Update 2026-09-10 (campaigns).** New tenant-scoped surface:
> `POST /v1/campaigns` (goal, model, engines, gpu_profile, workload, optional
> max_trials / max_duration / planner_model; ceilings 50 trials and 24h),
> `GET /v1/campaigns`, `GET /v1/campaigns/{id}`,
> `GET /v1/campaigns/{id}/events?after_seq=&limit=` (returns `events`, `last_seq`,
> `state`, `done` — poll with the cursor to follow a run), and
> `DELETE /v1/campaigns/{id}` to cancel. A campaign belonging to another tenant is
> 404 on every one of these, including the event stream.
>
> **Update 2026-09-10.**
> - `POST /v1/jobs` accepts an **`engine_config`** object: `quantization`
>   (`fp8|int8|int4|gptq|awq`), `tensor_parallel` (1-8), `max_batch_size` (1-4096),
>   `max_model_len` (1-1048576), `gpu_memory_utilization` (0-1). Every field is
>   optional; omitting one leaves the engine's own default. Zero is "unset", not a
>   value, and is omitted on the wire to the worker.
> - **`workload` is now validated**: concurrency 1-1024, prompt/output tokens
>   1-1,000,000, num_requests 1-100,000. Previously unvalidated — `concurrency: 0`
>   was accepted and hung the worker. Out-of-range values are rejected with 400 and a
>   field-specific message; nothing is silently clamped.
> - **`GET /v1/metrics` is now tenant-scoped.** It previously returned every tenant's
>   results. See `known-issues.md`.
> - **`GET /v1/workers`** (scope `jobs:read`) proxies the orchestrator's registry.
> - **gRPC is gone.** The `:9090` server described below was deleted with the dead
>   gateway path. `proto/inferx/v1/` still exists; nothing serves it.

> **Correction (2026-07-05):** this file previously described `internal/gateway/handler/`'s `SubmitJobRequest`/`Job` shapes — that package is dead code (see [known-issues.md](known-issues.md)). Everything below is verified directly against `cmd/gateway/main.go`'s route table and `internal/gateway/handlers.go`, which is what actually runs.

## HTTP (Gateway — `:8080`)

### `GET /health`
Public, unauthenticated. Returns `{"status":"ok","version":"0.1.0","postgres":"ok"|"error"}`.

### `POST /v1/jobs` — scope `jobs:write`
Request (`Handler.CreateJobRequest`):
```json
{
  "model": "meta-llama/Llama-3-8B",
  "engines": ["vllm"],
  "workload": {"concurrency": 32, "prompt_tokens": 512, "output_tokens": 256, "num_requests": 200},
  "gpu_profile": "a100-80gb",
  "auto_route": false
}
```
`engines` must be non-empty and every entry must be in the allowlist: `vllm`, `sglang`, `tensorrt`, `llamacpp`, `ollama`, `mock` (`mock` added 2026-07-05 for no-GPU testing — see [known-issues.md](known-issues.md)). `gpu_profile` is required (used both for cost lookup and worker selection — see [architecture.md](architecture.md)'s worker registry section). If `auto_route` is true, `engines[0]` is overwritten by the classifier's recommendation.

Response (`202 Accepted`):
```json
{"job_id": "...", "state": "pending", "auto_routed": false, "recommended_engine": ""}
```
Errors: `400` (missing model/engines/gpu_profile, unknown engine).

### `GET /v1/jobs` — scope `jobs:read`
Query: `state`, `limit` (default 20, max 100), `offset`. Response: `{"jobs": [...], "total": N, "limit": N, "offset": N}` — always tenant-scoped (only the caller's own jobs).

### `GET /v1/jobs/{jobID}` — scope `jobs:read`
Returns `jobs.Job` (see `internal/jobs/state.go`). `404` if not found *or* owned by another tenant (never distinguishes the two — see the IDOR-prevention note in [../knowledge/security-notes.md](../knowledge/security-notes.md)).

### `GET /v1/jobs/{jobID}/results` — scope `jobs:read`
Returns `[]jobs.Result` (TTFT/ITL/throughput/GPU-mem/KV-cache/cost per engine tried). `404` if the job doesn't exist or isn't owned by the caller; `200` with `[]` if the job exists but has no results yet.

### `DELETE /v1/jobs/{jobID}` — scope `jobs:write`
Cancels a non-terminal job; `409` if already terminal. Best-effort notifies the orchestrator (`POST {ORCHESTRATOR_URL}/internal/jobs/{jobID}/cancel`) — fire-and-forget, not guaranteed delivered.

### `POST /v1/route` — scope `jobs:read`
Request: `router.ClassificationInput` (prompt/output tokens, concurrency, structured-output/tool-calls flags, shared-prefix ratio). Response: `router.ClassificationResult` (recommended engine + reasoning + confidence). Stateless — no job is created.

### `GET /v1/engines` — scope `jobs:read`
Returns the static list of supported engines with descriptions (not tied to what's actually registered/available — see `GET /internal/workers` below for live worker availability).

### `GET /v1/metrics` — scope `metrics:read`
Query: `engine`, `model`, `since` (RFC3339, default now-24h). Returns `[]jobs.Result` from `metrics.bench_results`.

### `POST /v1/admin/apikeys` — scope `admin:all`
Request: `{"tenant_id": "...", "scopes": ["jobs:write", ...], "expiry_days": 30}`. Response: `{"token": "<jwt, shown once>", "expires_at": "..."}`. See [dependencies.md](dependencies.md) for how to get your *first* key (dev bootstrap, since this endpoint itself requires an existing `admin:all` token).

### `GET /v1/workers` — scope `jobs:read` (added 2026-07-05)
Relays the orchestrator's `GET /internal/workers` (below) so browser/CLI clients only ever need the gateway, never the orchestrator's unauthenticated internal port. `502` if the orchestrator is unreachable. Backs the dashboard's worker panel and `internal/cli.OrchestratorClient` (which actually still calls the orchestrator directly for `inferbolt run`'s spawn-check loop — this endpoint is for browser use, see [architecture.md](architecture.md)).

### `GET /dashboard` — public (added 2026-07-05)
Serves a single embedded static HTML/JS page (`internal/gateway/static/dashboard.html`, no build step) — a minimal read-only view of workers and jobs. The page itself is public; it calls the authenticated endpoints above client-side using a bearer token the user pastes in (persisted to the browser's `localStorage`). No server-side session, no cookies.

## Orchestrator Internal Routes (`:8081`, unauthenticated by design — same-host/cluster callers only)
- `GET /health`
- `POST /internal/workers/register`, `POST /internal/workers/heartbeat` — used by `worker/main.py`.
- `GET /internal/workers` (added 2026-07-05) — returns the current `WorkerRegistry` snapshot (`[]workers.WorkerEntry`). Used by `inferbolt run` to check availability before spawning a worker, and by `internal/cli.OrchestratorClient`.
- `POST /internal/results` — used by workers to report completed benchmark results.
- `GET /internal/jobs/{jobID}/state`.

## gRPC (`proto/inferx/v1/`)
`.proto` definitions exist and generated code lives in `gen/inferx/` (regenerate via `make proto`, never hand-edit generated code) — but **no gRPC server is started in `cmd/gateway/main.go`** as currently written. Treat gRPC as not-currently-live until confirmed otherwise; don't assume `:9090` is listening.

## Auth Scheme
`Authorization: Bearer <jwt>` only — HMAC-SHA256, claims `tid` (tenant ID) and `scp` (scopes). There is no `X-API-Key` header scheme in the live gateway (that belongs to the dead `internal/gateway/router.go`/`handler/` path). See [dependencies.md](dependencies.md) for the full scope table.

## Versioning Policy
- Additive, backward-compatible fields may land on `/v1/` directly.
- Any breaking change (removed/renamed field, changed semantics, changed status codes) requires `/v2/` — never a silent contract change on `/v1/`.

## Open Gaps (update as these are implemented)
- No streaming/polling contract for live per-request benchmark progress — the dashboard and CLI both poll `GET /v1/jobs/{id}` on an interval, there's no push/streaming mechanism.
- No cost-report endpoint (`cost/` is still an empty scaffold — see [roadmap.md](roadmap.md)).
