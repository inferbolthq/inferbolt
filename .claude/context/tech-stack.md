# Tech Stack

## Languages & Runtimes
- **Go 1.22** — all orchestration services (`gateway`, `orchestrator`, `router`, `collector`, `operator`, `inferbolt` CLI). Module: `github.com/inferbolthq/inferbolt`.
- **Python 3.11+** — stateless benchmark workers (`worker/`), managed with `uv`.

## Go Dependencies (from `go.mod`)
| Package | Purpose |
|---|---|
| `go-chi/chi/v5` | HTTP router for the gateway |
| `golang-jwt/jwt/v5` | JWT auth (HMAC-signed, `tenant_id`/`tier` claims) |
| `jackc/pgx/v5` | Postgres driver (parameterized queries only — never string-built SQL) |
| `riverqueue/river` + `riverdriver/riverpgxv5` | Postgres-backed durable job queue for async work (benchmark dispatch, drift scans) |
| `dgraph-io/ristretto` | In-memory cache for hot, staleness-tolerant reads |
| `spf13/cobra` + `spf13/viper` | CLI (`cmd/inferbolt`) and config |
| `go.opentelemetry.io/otel` (+ `trace`) | Distributed tracing across services |
| `google.golang.org/grpc` | Internal gRPC service (`proto/inferx/v1/`) |
| `k8s.io/client-go`, `k8s.io/api`, `k8s.io/apimachinery`, `sigs.k8s.io/controller-runtime` | k8s operator (`cmd/operator`, `k8s/crds`) |
| `golang.org/x/time` | Rate limiting (token bucket, IP + tenant) |
| `stretchr/testify` | Test assertions |
| `olekukonko/tablewriter`, `schollz/progressbar/v3` | CLI output formatting |

## Python Dependencies (from `pyproject.toml`)
| Package | Purpose |
|---|---|
| `fastapi` + `uvicorn[standard]` | Worker HTTP surface |
| `pydantic` | Request/config validation |
| `httpx` | Worker → orchestrator HTTP calls |
| `optuna` | Config sweep / hyperparameter search (planned: `search/`) |
| `numpy` | Numeric processing for benchmark stats |
| `opentelemetry-sdk`, `opentelemetry-exporter-otlp`, `opentelemetry-instrumentation-fastapi` | Worker-side tracing, consistent with the Go side |
| Optional extras: `vllm`, `sglangrt`, `llama-cpp-python` | Engine-specific dependencies, installed per deployment need |
| `pytest`, `pytest-asyncio` (dev) | Testing |

## Data Stores
- **PostgreSQL** (via `timescale/timescaledb:latest-pg16` image) — durable state: `jobs`, `recommendations`, `audit_log`, `baselines`, `api_keys`, plus River's internal queue tables.
- **TimescaleDB** (extension on the same Postgres instance) — `metrics.bench_results` hypertable for benchmark time-series, with compression policy (7-day) and segment-by `engine,model`.

## Infra & Ops
- **Docker Compose** (`docker-compose.yml`) — local dev stack: postgres, orchestrator, router, collector.
- **Kubernetes** — Helm chart (`k8s/helm/`), CRD `OptimizedInference` (`k8s/crds/optimized_inference_crd.yaml`) reconciled by `cmd/operator`.
- **GitHub Actions** — `ci.yaml` (build/test gate), `release.yaml`; releases built via `.goreleaser.yaml`.
- **OpenTelemetry** — tracing/metrics standard across all services; do not introduce a second observability mechanism.

## Notably Not Used (despite README mentions)
The README describes ClickHouse and Redis as part of the original target architecture. The implementation that actually exists uses **TimescaleDB** for metrics and **River (Postgres-backed)** for the job queue instead. See [known-issues.md](known-issues.md). Don't reintroduce ClickHouse/Redis without an explicit decision — the current stack already satisfies the same needs with one fewer moving part (single Postgres instance for both durable state and time-series).
