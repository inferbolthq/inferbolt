# InferBolt

Open-source LLM inference benchmarking and optimization. Run head-to-head benchmarks across vLLM, SGLang, llama.cpp and Ollama — measure TTFT, inter-token latency, throughput, KV cache hit rate and cost per million tokens — then let an agent find the configuration that best fits your workload.

---

## Quick start

```bash
go build -o inferbolt ./cmd/inferbolt

# One command: starts the stack, spawns a worker, runs a benchmark, prints results.
./inferbolt run --model my-model --engines mock --gpu cpu
```

`run` brings up the backing services with `docker compose` if they aren't already up, bootstraps a local dev API key on first use, spawns a Python worker for the requested GPU profile if none is registered, submits the job, and prints the results. It needs Docker running and either `uv` or `python` on `PATH` with the worker's dependencies installed (`cd worker && uv sync`, or `pip install -e .` from the repo root).

`--engines mock --gpu cpu` needs no GPU and no model server. Swap in `--engines vllm --gpu a100-80gb` once you have real inference hardware.

### Let the agent find the configuration

```bash
export ANTHROPIC_API_KEY=...

./inferbolt agent "cheapest engine for chat traffic under 200ms p99 TTFT" \
  --model meta-llama/Llama-3.1-8B --gpu a100-80gb --engines vllm,sglang
```

The agent plans a sequence of benchmarks, submits them through the ordinary API, reads the measurements back, and recommends an engine and configuration citing the trials that support it:

```
[look]   checking registered workers
[run]    vllm     fp8 TP1                  establishing an fp8 baseline
[ok]     vllm     fp8 TP1                  3120 tok/s  142ms p99 TTFT  $0.3105/Mtok
[run]    sglang   fp8 TP1                  RadixAttention should win on shared prefixes
[ok]     sglang   fp8 TP1                  3980 tok/s  118ms p99 TTFT  $0.2810/Mtok
[think]  sglang leads on both; probing int4 for cost headroom
[run]    sglang   int4 TP1                 int4 should cut cost if TTFT holds
[ok]     sglang   int4 TP1                 4210 tok/s  121ms p99 TTFT  $0.1900/Mtok
[done]   campaign complete (18m4s)

Recommendation: sglang int4 TP1
  throughput   4210 tok/s
  p99 TTFT     121 ms
  cost         $0.1900 / Mtok
  confidence   high
```

The model plans; it never measures. Every number in a report comes from a benchmark that actually ran. See [The agent](#the-agent) for the guardrails.

<details>
<summary>Manual, step-by-step path — useful when iterating on one service</summary>

```bash
go mod tidy
cd worker && uv sync && cd ..

docker compose up -d          # postgres+timescale, gateway, orchestrator, router, collector
python -m worker.main         # a worker for the GPU profile you want to benchmark

curl http://localhost:8080/health
# {"status":"ok","version":"0.1.0","postgres":"ok"}
```

Submitting a job by hand needs a bearer token — the gateway writes one to `DEV_TOKEN_FILE` on first startup in development, or use `inferbolt admin apikeys create`:

```bash
curl -X POST http://localhost:8080/v1/jobs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
        "model": "my-model",
        "engines": ["mock"],
        "gpu_profile": "cpu",
        "workload": {"prompt_tokens": 512, "output_tokens": 256, "concurrency": 4, "num_requests": 50},
        "engine_config": {"quantization": "fp8", "tensor_parallel": 1}
      }'
```

</details>

---

## Architecture

```
CLI / API client
  │
  ▼
cmd/gateway            HTTP :8080 — auth, rate limiting, validation, job submission
  │                    also serves the read-only dashboard at /dashboard
  ├── writes ─────────────────────────► PostgreSQL + TimescaleDB
  └── enqueues ───────────────────────► River (durable queue, on Postgres)
                                              │
cmd/orchestrator       :8081 ◄────────────────┘ owns the job lifecycle,
  │                                             dispatches to workers, ingests results,
  │                                             and runs optimization campaigns
  ├── cmd/router       :8082  workload classification, engine selection
  ├── cmd/collector    :8083  metrics + drift detection, Slack alerts
  └── cmd/operator            k8s controller for the OptimizedInference CRD
  │
  ▼
worker/ (Python)       stateless benchmark execution, self-registers with the orchestrator
  └── engines/         vllm · sglang · llamacpp · ollama · mock
```

Go owns all orchestration state. Python workers are stateless, scale horizontally, and talk to the orchestrator over HTTP only. Durable state lives in one place — Postgres, with the TimescaleDB extension for the `metrics.bench_results` hypertable and River for the job queue.

**Job flow.** `POST /v1/jobs` validates and persists the job, then enqueues it on River. The orchestrator picks it up, selects an idle registered worker matching the GPU profile, and POSTs the job to it. The worker runs each requested engine, computes results, and posts them back to `/internal/results`, where they land in TimescaleDB and the job is marked complete. The collector compares fresh results against stored baselines and alerts on drift.

**Workers self-register.** Nothing provisions them. A worker POSTs to `/internal/workers/register` on startup with its callback URL and GPU profile, then heartbeats; stale entries are evicted. A job whose GPU profile has no idle worker fails rather than queueing indefinitely.

---

## The agent

`inferbolt agent "<goal>"` runs a goal-directed optimization campaign.

**What it controls.** The engine and its configuration — quantization, tensor parallelism, batch size, context length, GPU memory fraction. The model, GPU profile and workload are fixed for the whole campaign: trials are only comparable when the configuration is the sole variable.

**What it can touch.** Five tools — four reads (`list_workers`, `classify_workload`, `historical_results`, and reading back results) and one action (`run_benchmark`). No shell, no filesystem. It authenticates with the same bearer token and hits the same tenant-scoped endpoints as any other API client; it holds no database handle.

**Budgets are hard.** Every benchmark occupies a GPU worker for minutes, so a campaign runs against a trial cap and a wall-clock cap, both checked *before* a benchmark starts. Re-requesting a configuration already measured in the campaign returns the earlier result without spending a trial. If the planner ends without concluding, the campaign still reports the cheapest error-free trial, explicitly flagged as a mechanical fallback rather than the model's judgement.

**It's durable.** The loop runs in the orchestrator as a River job on its own queue, writing each step to `campaign_events` as it happens. A campaign survives the client disconnecting, records its progress for replay, and can be cancelled between trials. River retries cannot double-run one: the runner claims the row with a conditional update, and a campaign that fails is recorded as failed rather than retried with a budget it has already spent.

**It plans, it doesn't measure.** Every figure in a report is read off a benchmark result row. A failed trial is handed back to the planner as an error so it can adapt, and is never cited as a measurement.

| Flag | Default | Description |
|---|---|---|
| `--model` | _(required)_ | Model to optimize for |
| `--gpu` | _(required)_ | GPU profile to benchmark on |
| `--engines` | `vllm,sglang` | Candidate engines the agent may try |
| `--max-trials` | `6` | Hard cap on benchmarks run |
| `--max-duration` | `2h` | Wall-clock limit |
| `--planner-model` | `claude-opus-5` | Anthropic model used for planning |
| `--concurrency` / `--prompt-tokens` / `--output-tokens` / `--requests` | `32` / `512` / `256` / `200` | The fixed workload |

Like `run`, `agent` starts whatever the campaign needs and isn't already up — the compose services, a local dev credential on first use, and a worker for the requested GPU profile — and stops a worker it started when the campaign ends (`--keep-worker` to leave it). `--gpu-host user@host` runs the worker on a remote GPU box over SSH.

**The campaign runs on the server, not in your terminal.** `agent` submits it and follows along; Ctrl-C detaches the view and leaves the work running. Reattach any time:

```bash
inferbolt campaigns list
inferbolt campaigns follow <id>     # replays from the start, then follows
inferbolt campaigns get <id>        # the finished report
inferbolt campaigns cancel <id>     # stops before the next trial
```

Planning calls the Anthropic API from the **orchestrator**, so `ANTHROPIC_API_KEY` belongs in its environment — `docker-compose.yml` passes yours through, so `export ANTHROPIC_API_KEY=...` before `docker compose up` is enough. The key never reaches a client. `--output json` emits the full report — every trial, its configuration, its measurements, and token usage.

To exercise the whole loop without a GPU: `--engines mock --gpu cpu`. The mock engine simulates plausible responses to configuration changes so the loop has a gradient to follow; its numbers are explicitly not measurements of anything real.

---

## CLI

| Command | Purpose |
|---|---|
| `inferbolt run <model>` | One-shot: start everything needed, benchmark, print results |
| `inferbolt agent "<goal>"` | Submit a goal-directed optimization campaign and follow it |
| `inferbolt campaigns list \| get \| follow \| cancel` | Manage campaigns |
| `inferbolt benchmark run` | Submit a single benchmark against a running stack |
| `inferbolt benchmark compare` | Compare several engines in one job |
| `inferbolt jobs list \| get \| cancel` | Manage jobs |
| `inferbolt metrics` | Query historical results |
| `inferbolt route` | Classify a workload, get the rules-based engine recommendation |
| `inferbolt admin apikeys create` | Issue a scoped API key |
| `inferbolt configure` | Write `~/.inferbolt/config.yaml` |

---

## API

All routes except `/health` and `/dashboard` require `Authorization: Bearer <jwt>`. The token carries a `tenant_id` and a set of scopes; every tenant-scoped query filters by the tenant in the token.

| Route | Scope | Purpose |
|---|---|---|
| `POST /v1/campaigns` | `jobs:write` | Start an optimization campaign |
| `DELETE /v1/campaigns/{id}` | `jobs:write` | Cancel a pending or running campaign |
| `GET /v1/campaigns` | `jobs:read` | List campaigns (paginated) |
| `GET /v1/campaigns/{id}` | `jobs:read` | Campaign detail and recommendation |
| `GET /v1/campaigns/{id}/events` | `jobs:read` | Campaign progress, resumable via `?after_seq=` |
| `POST /v1/jobs` | `jobs:write` | Submit a benchmark |
| `DELETE /v1/jobs/{id}` | `jobs:write` | Cancel a non-terminal job |
| `GET /v1/jobs` | `jobs:read` | List jobs (paginated) |
| `GET /v1/jobs/{id}` | `jobs:read` | Job detail |
| `GET /v1/jobs/{id}/results` | `jobs:read` | Results for a job |
| `POST /v1/route` | `jobs:read` | Classify a workload |
| `GET /v1/engines` | `jobs:read` | Available engines |
| `GET /v1/workers` | `jobs:read` | Registered workers |
| `GET /v1/metrics` | `metrics:read` | Historical results by engine + model |
| `POST /v1/admin/apikeys` | `admin:all` | Issue a key |
| `GET /health` | _(public)_ | Liveness + Postgres reachability |
| `GET /dashboard` | _(public page, API calls authenticated)_ | Read-only results view |

Rate limiting is a flat 100 requests/minute per tenant, applied after authentication. There is no tenant-tier concept: access is governed by scopes.

`workload` and `engine_config` are validated at the boundary and rejected — never silently clamped — when out of range: concurrency 1–1024, prompt/output tokens 1–1,000,000, requests 1–100,000, tensor parallel 1–8, batch size 1–4096, and quantization from `fp8 · int8 · int4 · gptq · awq`. Omitting an `engine_config` field leaves the engine's own default in place.

---

## Configuration

Everything is environment variables, read once at startup.

### gateway

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | Postgres connection string |
| `JWT_SECRET` | _(required)_ | HMAC secret; must be at least 32 characters |
| `ORCHESTRATOR_URL` | _(required)_ | Orchestrator base URL |
| `PORT` | `8080` | HTTP listen port |
| `ENV` | `development` | `development` enables the dev-key bootstrap |
| `DEV_TOKEN_FILE` | _(unset)_ | Development only: path the bootstrapped dev key is written to |
| `OTEL_ENDPOINT` | _(unset)_ | OTLP endpoint |

### orchestrator

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | Postgres connection string |
| `PORT` | `8081` | HTTP listen port |
| `WORKER_EVICTION_INTERVAL` | `10s` | How often stale workers are evicted |
| `ANTHROPIC_API_KEY` | _(unset)_ | Campaign planning. Benchmarks work without it; campaigns fail on their first turn |

### router

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | Postgres connection string |
| `ORCHESTRATOR_URL` | _(required)_ | Orchestrator base URL |
| `PORT` | `8082` | HTTP listen port |

### collector

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | Postgres connection string |
| `PORT` | `8083` | HTTP listen port |
| `DRIFT_CHECK_INTERVAL` | `5m` | How often baselines are re-checked |
| `DRIFT_LOOKBACK` | `1h` | Window of results compared against baseline |
| `DRIFT_WARNING_PCT` | `15.0` | Deviation that warns |
| `DRIFT_CRITICAL_PCT` | `30.0` | Deviation that alerts |
| `SLACK_WEBHOOK_URL` | _(unset)_ | Where drift alerts go; disabled when empty |

### worker (Python)

| Variable | Default | Description |
|---|---|---|
| `ORCHESTRATOR_URL` | _(unset)_ | Where to register; registration is skipped when empty |
| `PORT` | `8001` | HTTP listen port |
| `WORKER_URL` | `http://127.0.0.1:$PORT` | Callback URL advertised at registration |
| `GPU_PROFILE` | `cpu` | Profile this worker serves |
| `LOG_LEVEL` | `INFO` | Log level |
| `OTEL_ENDPOINT` | _(unset)_ | OTLP endpoint |

### CLI

`~/.inferbolt/config.yaml`, overridden by `INFERBOLT_SERVER_URL`, `INFERBOLT_API_KEY`, `INFERBOLT_TENANT_ID`, `INFERBOLT_OUTPUT`. The agent additionally reads `ANTHROPIC_API_KEY`.

---

## Repo layout

```
cmd/
  gateway/ orchestrator/ router/ collector/ operator/   # services
  inferbolt/                                            # CLI
internal/
  agent/        # the campaign loop: plan, measure, recommend
  campaigns/    # durable campaigns — store, server-side platform, River worker
  auth/         # JWT issue/verify, scopes, tenant context, middleware
  cli/          # typed API client, config, agent platform adapter
  config/       # Postgres-backed job/recommendation store
  drift/        # baselines, detection, Slack notifier
  engine/       # Go-side engine abstraction
  gateway/      # HTTP handlers, interfaces, embedded dashboard
  jobs/         # job lifecycle + the River benchmark worker
  metrics/      # TimescaleDB reader/writer
  queue/        # River client
  router/       # workload classifier, engine selector
  workers/      # worker registry
worker/         # Python: engines/, cost/, search/, tests/
migrations/     # 001…007, applied in order
k8s/            # CRD + Helm chart
proto/, gen/    # gRPC definitions (no server currently wired — see Status)
```

---

## Development

```bash
make build     # compile all cmd/ binaries
make test      # go test ./...
make proto     # regenerate gRPC code
```

Requires Go 1.24+, Python 3.11+, Docker. Python tests: `pytest` from the repo root (`pip install -e '.[dev]'`).

---

## Status

Working: gateway with JWT-scope auth and boundary validation, orchestrator with River dispatch and worker registry, five Python engine adapters, TimescaleDB metrics, drift detection with Slack alerts, the read-only dashboard, `inferbolt run`, and durable server-side optimization campaigns.

Not yet built:

- **Cost model** — `worker/cost/modeler.py` computes cost per million tokens from a hardcoded GPU price table. `cost/` at the repo root is an empty scaffold.
- **Optuna config sweep** — `worker/search/config_search.py` implements a TPE sweep with a Pareto frontier but is not wired to any caller. The agent covers the same ground with a different strategy; whether the deterministic sweep becomes a second campaign mode is undecided.
- **gRPC** — `proto/inferx/v1` and `gen/` exist, but the server that used them was part of a dead code path removed in this branch. Nothing serves gRPC today.
- **Dashboard** — the embedded `/dashboard` page is read-only, with no build step and no framework. A separate React app under `ui/` covers jobs, results and metrics but does not yet know about campaigns.
- **Multi-replica gateway** — the rate limiter keeps buckets in process memory, so running more than one replica multiplies the effective limit. Fix before scaling out.
- **No pre-auth rate limiting** — the only limiter runs after authentication, so unauthenticated requests (`/health`, `/dashboard`, and failed auth attempts) are unbounded. An IP limiter existed only in a code path nothing ever wired up, and went with it.

---

## License

Apache 2.0
