# Dependencies & Auth Model Reference

For the full dependency list, see [tech-stack.md](tech-stack.md). This file describes the gateway's **actual, live** auth/config surface — i.e. what `cmd/gateway/main.go` really wires up, not the orphaned tier-based implementation.

**Correction (2026-07-05):** an earlier version of this file documented a tier-based rate-limit model (`oss`/`cloud_free`/`cloud_paid`/`enterprise`, `internal/auth/tier.go`, `internal/config/gateway.go`'s `GatewayConfig`). That system exists in the repo but **is not wired into any running binary** — it's only referenced by `internal/gateway/router.go` + `internal/gateway/handler/`, which nothing in `cmd/` imports. See [known-issues.md](known-issues.md) for the full finding. What's below is the model that's actually live.

## Auth Model — JWT with Scopes (`internal/auth/apikey.go`, `middleware.go`)

Auth is a signed JWT (HMAC-SHA256, `JWT_SECRET`) carrying a `tenant_id` and a list of scopes — not a tenant tier:

| Scope | Grants |
|---|---|
| `jobs:write` | `POST /v1/jobs` |
| `jobs:read` | `GET /v1/jobs`, `GET /v1/jobs/{id}`, `GET /v1/jobs/{id}/results`, `POST /v1/route`, `GET /v1/engines` |
| `metrics:read` | `GET /v1/metrics` |
| `configs:write` | (reserved — not yet consumed by a route) |
| `admin:all` | Everything, including `POST /v1/admin/apikeys` (implicitly satisfies any other scope check — see `KeyManager.HasScope`) |

Tokens are issued via `KeyManager.Issue(tenantID, scopes, expiry)`; only the SHA-256 hash is ever persisted (`public.api_keys.key_hash`) — the plaintext exists only at issuance time (API response body, or the dev-token file below), matching the security posture in [../knowledge/security-notes.md](../knowledge/security-notes.md).

There is **no tenant tier and no per-tier rate limit**. `RateLimitMiddleware` enforces a flat **100 requests/minute per tenant**, full stop — see `internal/auth/middleware.go`.

## Getting Your First API Key

- **Local dev (default, `ENV=development`)**: the gateway auto-bootstraps an all-scopes key for a `dev` tenant on first startup (`cmd/gateway/main.go`'s `bootstrapDevKey`) and, if `DEV_TOKEN_FILE` is set (as it is in `docker-compose.yml`, pointed at a bind-mounted `.inferbolt-dev/dev-token`), writes the plaintext there so it's recoverable across restarts — not just visible once in the startup log. `inferbolt run` reads this automatically when no key is configured and the target server is local; see `memory/completed-features.md`.
- **Anywhere else**: `POST /v1/admin/apikeys` (requires an existing `admin:all`-scoped token — chicken-and-egg by design; the dev bootstrap is the only way to get the first one without direct DB access) or `inferbolt admin apikeys create --tenant-id <id> --scopes <scopes>`.

## Config Surface — What `cmd/gateway/main.go` Actually Reads

| Variable | Required? | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres connection string |
| `JWT_SECRET` | yes | — | HMAC secret; **must be ≥ 32 characters** or the gateway refuses to start |
| `ORCHESTRATOR_URL` | yes | — | Orchestrator base URL (job cancel notifications) |
| `PORT` | no | `8080` | HTTP listen address |
| `ENV` | no | `development` | `development` enables `bootstrapDevKey` |
| `OTEL_ENDPOINT` | no | _(empty)_ | If set, logs that OTel SDK env vars should be configured (exporter wiring itself is minimal here) |
| `DEV_TOKEN_FILE` | no | _(empty)_ | If set, the bootstrapped dev key's plaintext is written here (0600) and reused across restarts instead of being re-issued/lost |

There is **no `GRPC_ADDR`, `API_KEYS`, `ENTERPRISE_LIMITS`, `IP_RATE_LIMIT_*`, or `HTTP_ADDR`** in the live gateway — those belong to the dead `internal/config/gateway.go` path.

Orchestrator-side (`cmd/orchestrator/main.go`): `DATABASE_URL` (required), `PORT` (default `8081`), `WORKER_EVICTION_INTERVAL` (default `10s`).

Worker-side (`worker/main.py`, the long-running FastAPI service — not the older standalone `worker/benchmark.py`): `ORCHESTRATOR_URL`, `PORT` (default `8001`), `WORKER_URL` (defaults to `http://127.0.0.1:<PORT>`), `GPU_PROFILE` (default `cpu`), `OTEL_ENDPOINT`, `LOG_LEVEL`. The engine to run is selected **per job** (`BenchmarkJob.engines`), not via a worker-level env var — one worker process serves any engine adapter registered in `worker/engines/__init__.py`'s `ENGINES` dict.

Collector-side (`docker-compose.yml`): `DRIFT_CHECK_INTERVAL`, `DRIFT_LOOKBACK`, `DRIFT_WARNING_PCT`, `DRIFT_CRITICAL_PCT`, `SLACK_WEBHOOK_URL`.

## Rule of Thumb
Every authenticated request context carries `tenant_id` (from JWT claims, set by `AuthMiddleware`) from the auth middleware onward — downstream code reads it via `iauth.MustGetTenantID(ctx)`, never re-derives or re-parses it. There is no "tier" in context to read in the live system — don't add code that expects one without first confirming which gateway implementation it's meant to run against (see [known-issues.md](known-issues.md)).
