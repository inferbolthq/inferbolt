# Deployment

## Local Development

**Zero-manual-intervention path (added 2026-07-05):**
```bash
inferbolt run --model my-model --engines mock --gpu cpu
```
This brings up `docker compose` if the gateway isn't reachable, bootstraps/reuses a local dev API key, spawns a Python worker for the requested GPU profile if none is registered, and submits/polls/reports the benchmark — see `memory/completed-features.md` and [architecture.md](architecture.md)'s "Worker Registration & Dispatch" section for how this actually works under the hood. Requires Docker running locally; `--project-dir` if not invoked from the repo root.

**Manual/step-by-step path** (useful when iterating on a single service):
```bash
go mod tidy                 # Go deps
cd worker && uv sync         # Python deps (or: pip install -e . from repo root)
docker compose up -d         # postgres+timescaledb, gateway, orchestrator, router, collector
python -m worker.main        # from repo root; registers with the orchestrator over ORCHESTRATOR_URL
```
`make build` compiles all `cmd/` binaries; `make test` runs the Go test suite; `make proto` regenerates gRPC code from `proto/inferx/v1/` (note: no gRPC server is currently started by `cmd/gateway` — see [api-contracts.md](api-contracts.md)).

## Docker Images
One Dockerfile per binary: `Dockerfile.gateway` (added 2026-07-05), `Dockerfile.orchestrator`, `Dockerfile.router`, `Dockerfile.collector`, `Dockerfile.operator`, `Dockerfile.inferbolt`. Each should stay multi-stage (build stage with full Go toolchain, minimal runtime stage) and never bake secrets into a layer.

`docker-compose.yml` wires: `postgres` (TimescaleDB image, migrations mounted at `/docker-entrypoint-initdb.d`) → `gateway` + `orchestrator` → `router` → `collector`, each gated by the previous service's healthcheck (`condition: service_healthy`). The `gateway` service mounts `./.inferbolt-dev:/var/lib/inferbolt` and sets `DEV_TOKEN_FILE` so its bootstrapped dev API key survives container restarts instead of only ever appearing once in the startup log (see [dependencies.md](dependencies.md)).

## Kubernetes
- Helm chart: `k8s/helm/`.
- CRD: `k8s/crds/optimized_inference_crd.yaml` (Go types in `optimized_inference_types.go`) — lets users declare an `OptimizedInference` resource that the operator (`cmd/operator`) reconciles into actual benchmark jobs.
- Every deployed service needs: resource requests/limits, liveness/readiness probes hitting `/health`, and secrets sourced from Kubernetes Secrets (or an external secret manager), never inlined in Helm values.

## CI/CD
- `.github/workflows/ci.yaml` — build + test gate on every PR.
- `.github/workflows/release.yaml` + `.goreleaser.yaml` — tagged releases build and publish binaries.
- No merge to `master` should bypass the CI gate; no release skips semantic versioning (§7 of `CLAUDE.md`).

## Secrets
`JWT_SECRET`, Postgres credentials, `SLACK_WEBHOOK_URL`, API keys — environment variables at runtime only. The `change-me-in-production` default for `JWT_SECRET` is a footgun if forgotten in a real deployment; flag its presence in any production-readiness check (see [known-issues.md](known-issues.md)).

## Observability
- OpenTelemetry wired via `middleware/otel.go` (gateway) and the Python `opentelemetry-instrumentation-fastapi` (worker) — new services must propagate trace context across the HTTP boundary, not start a fresh, disconnected trace.
- Health checks: every service in `docker-compose.yml` already has one; new services must match this pattern before being added to the compose file or Helm chart.
- Alerts: drift detection's Slack notifier (`internal/drift/notifier.go`) is the template for "actionable, not noisy" alerting — new alerts should meet the same bar (a human should be able to act on every alert that fires).

## Migration Sequencing
Migrations are additive-first. During a rolling deploy, the schema change must land in a way that both the old and new code versions can run against it simultaneously — add columns/tables before deploying code that requires them; never drop a column the currently-running old version still reads.
