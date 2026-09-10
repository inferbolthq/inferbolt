# InferX

Open-source LLM inference benchmarking platform. Run head-to-head benchmarks across vLLM, SGLang, and llama.cpp — measure TTFT, inter-token latency, throughput, KV cache hit rate, and cost per million tokens. Self-host for free or use the cloud tier.

---

## Architecture

```
Client
  │
  ▼
cmd/gateway          ← HTTP :8080 + gRPC :9090 (auth, rate limiting, routing)
  │
  ▼
cmd/orchestrator     ← job scheduler, dispatches to workers, writes results
  │
  ├── cmd/collector  ← OTel + Prometheus metrics collector
  ├── cmd/router     ← workload router (selects engine + worker)
  │
  ▼
worker/              ← Python benchmark workers (stateless, scale horizontally)
  └── engines/
        ├── vllm_engine.py
        ├── sglang_engine.py   (coming soon)
        └── llamacpp_engine.py (coming soon)
  │
  ▼
ClickHouse           ← columnar metrics store (TTFT, throughput, cost)
PostgreSQL           ← job state + tenant config
Redis                ← job queue
```

Go owns all orchestration state. Python workers are stateless — multiple can run in parallel. Workers communicate with the orchestrator via HTTP only, no shared memory.

---

## Repo Layout

```
inferx/
├── cmd/
│   ├── gateway/        # API gateway binary
│   ├── orchestrator/   # Job scheduler binary
│   ├── collector/      # OTel + Prometheus binary
│   ├── router/         # Workload router binary
│   └── operator/       # k8s CRD controller binary
├── internal/
│   ├── auth/           # Tenant context + tier definitions
│   ├── config/         # Config structs (env-based)
│   ├── engine/         # Go Engine interface + Result types
│   ├── gateway/        # HTTP router, middleware, gRPC server
│   │   ├── handler/    # HTTP handlers + OrchestratorClient interface
│   │   └── middleware/ # Auth, rate limiting, logging, recovery
│   ├── metrics/        # ClickHouse writer
│   ├── queue/          # Redis job queue
│   └── drift/          # Drift detection logic
├── worker/
│   ├── benchmark.py    # Worker entrypoint
│   └── engines/        # Engine adapters (vLLM, mock, …)
├── k8s/
│   ├── crds/           # OptimizedInference CRD YAML
│   └── helm/           # Helm chart for full deploy
├── proto/inferx/v1/    # gRPC service definition
├── gen/                # protoc-generated Go code (run `make proto`)
├── search/             # Optuna config sweep
├── cost/               # Cost model + report gen
├── ui/                 # React dashboard
└── docs/benchmarks/    # Published benchmark results
```

---

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.22+ | `winget install GoLang.Go` |
| Python | 3.11+ | |
| uv | latest | `pip install uv` |
| protoc | 3.x | Only needed to regenerate gRPC code |
| Docker | any | For ClickHouse + Postgres + Redis |

---

## Quick Start

### Fastest path: one command, no manual setup

```bash
go build -o inferbolt ./cmd/inferbolt
./inferbolt run --model my-model --engines mock --gpu cpu
```

This single command starts the backing services (`docker compose up -d`) if they aren't already running, bootstraps a local dev API key on first run, spawns a Python worker for the `cpu` GPU profile if none is registered yet, submits the benchmark, and prints results. Requires Docker running locally and either `uv` or `python` on `PATH` (with `worker/`'s dependencies installed — `cd worker && uv sync`, or `pip install -e .` from the repo root). `--engines mock --gpu cpu` needs no real GPU or model server — it's the fastest way to confirm the whole pipeline works end to end; swap in `--engines vllm --gpu a100-80gb` (etc.) once you have real inference infrastructure.

### Manual / step-by-step path

<details>
<summary>Useful when iterating on a single service instead of the whole stack</summary>

```bash
# 1. Fetch Go dependencies
go mod tidy

# 2. Install Python dependencies
cd worker && uv sync

# 3. Start backing services (Postgres+TimescaleDB, gateway, orchestrator, router, collector)
docker compose up -d

# 4. Start a worker (from repo root) for the GPU profile you want to benchmark against
python -m worker.main
```

```bash
# Health check
curl http://localhost:8080/health
# {"status":"ok","version":"0.1.0","postgres":"ok"}

# Submit a job (requires a bearer token — see the dev API key logged/written by the
# gateway on first startup, or run `inferbolt admin apikeys create` once you have one)
curl -X POST http://localhost:8080/v1/jobs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"model":"my-model","engines":["mock"],"workload":{"prompt_tokens":512,"output_tokens":256,"concurrency":4,"num_requests":50},"gpu_profile":"cpu"}'
```

</details>

> Note: this section previously described a ClickHouse/Redis/`X-API-Key` architecture that doesn't match the current implementation (TimescaleDB, River, JWT-with-scopes auth) — see `.claude/context/known-issues.md` for the full list of corrections; a broader README rewrite is tracked there but not yet done.

---

## Configuration

All config is via environment variables.

### Gateway

| Variable | Default | Description |
|---|---|---|
| `HTTP_ADDR` | `:8080` | HTTP listen address |
| `GRPC_ADDR` | `:9090` | gRPC listen address |
| `JWT_SECRET` | `change-me-in-production` | HMAC secret for JWT validation |
| `API_KEYS` | _(empty)_ | Comma-separated key:tenantID:tier entries |
| `ENTERPRISE_LIMITS` | _(empty)_ | Per-tenant overrides: tenantID:jobsPerHour:apiPerHour |
| `IP_RATE_LIMIT_RPS` | `50` | Global IP-based requests/sec (pre-auth) |
| `IP_RATE_LIMIT_BURST` | `10` | Burst size for IP limiter |
| `ORCHESTRATOR_URL` | `http://localhost:8081` | Internal orchestrator address |

**API_KEYS format:**
```
API_KEYS=key1:tenant-abc:cloud_free,key2:tenant-xyz:enterprise
```
Short form (OSS tier, tenantID = key): `API_KEYS=mydevkey`

**Enterprise limits format** (0 = unlimited):
```
ENTERPRISE_LIMITS=tenant-xyz:500:5000
```

### Worker

| Variable | Default | Description |
|---|---|---|
| `ORCHESTRATOR_URL` | `http://localhost:8080` | Orchestrator base URL |
| `JOB_ID` | _(required)_ | Job ID to execute |
| `ENGINE` | `mock` | Engine name: `vllm` \| `mock` |
| `MODEL` | `mock-model` | Model name passed to vLLM |

---

## Rate Limits

| Tier | Jobs / hour | API calls / hour |
|---|---|---|
| OSS / self-hosted | unlimited | unlimited |
| Cloud free | 10 | 100 |
| Cloud paid | 100 | 1000 |
| Enterprise | configurable | configurable |

Burst = full hourly allowance (tokens accumulate, can be spent at once). Every request context carries `tenant_id` and `tier` — all downstream services read from context, never re-derive it.

---

## Development

```bash
make build        # compile all cmd/ binaries
make test         # run all Go tests
make proto        # regenerate gRPC code from proto/inferx/v1/inferx.proto
make run/gateway  # run gateway with go run
```

### Middleware stack (HTTP)

```
Recovery → Logger → IPLimiter → [public routes]
                              → Auth → TenantLimiter(API) → [protected routes]
                                                          → TenantLimiter(Job) → POST /v1/jobs
```

### Auth

Supports two schemes on all protected routes:
- `X-API-Key: <key>` — looked up against `API_KEYS` config
- `Authorization: Bearer <jwt>` — HMAC-SHA256 signed, carries `tenant_id` and `tier` claims

---

## Roadmap

- [x] Repo scaffold
- [x] API gateway — routing, auth, rate limiting, gRPC
- [x] Python worker — vLLM engine adapter + mock engine
- [ ] Go orchestrator — job queue, dispatch, result ingestion
- [ ] ClickHouse schema + metrics writer
- [ ] SGLang + llama.cpp engine adapters
- [ ] Drift detection
- [ ] Cost model
- [ ] Optuna config sweep
- [ ] React dashboard
- [ ] Helm chart + k8s operator
- [ ] First public benchmark report

---

## License

Apache 2.0
