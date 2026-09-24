# Design Decisions

Running log of consequential technical decisions and their rationale, inferred from the codebase and to be extended going forward via `templates/adr.md` + `memory/project-decisions.md`. Treat entries without an explicit ADR as reconstructed rationale (best-effort, based on code evidence) rather than a recorded decision — confirm with the user before treating them as immutable.

## TimescaleDB over ClickHouse for metrics
**Decision (inferred):** Use a TimescaleDB extension on the primary Postgres instance for `metrics.bench_results`, rather than a separate ClickHouse cluster as the README originally described.
**Why (inferred):** One fewer operational moving part (single Postgres instance handles both transactional and time-series data), simpler local dev (`docker-compose.yml` only needs one datastore), and TimescaleDB's hypertable + compression policy covers the compaction/retention need ClickHouse would otherwise serve.
**Trade-off:** Postgres/TimescaleDB won't match a dedicated columnar store's raw analytical query throughput at very large scale. Acceptable for current scale; revisit if metrics volume/query patterns outgrow it (see [../knowledge/database-notes.md](../knowledge/database-notes.md)).

## River over Redis-backed queue
**Decision (inferred):** Use River (`riverqueue/river`, Postgres-backed) for durable async job dispatch, rather than a Redis-backed queue as the README originally described.
**Why (inferred):** Durability (jobs survive a Postgres restart without a separate persistence concern), one less datastore to operate, and River's built-in retry/scheduling semantics match the "benchmark dispatch + scheduled drift scans" use case well.
**Trade-off:** Lower raw throughput ceiling than a dedicated broker (Kafka/SQS/Redis Streams) at extreme scale; Postgres becomes a shared bottleneck for both transactional data and queue traffic under very high load.

## Interfaces defined at the consumer
**Decision (observed pattern):** `OrchestratorClient` is defined in `internal/gateway/interfaces.go` (the gateway, which consumes it), not in the orchestrator package that implements it.
**Why:** Keeps the gateway decoupled from orchestrator internals — the gateway only depends on the shape of interaction it needs, and the orchestrator has no import-time dependency on the gateway. Standard Go idiom ("accept interfaces, return structs") applied at a package/service boundary.
**Implication:** Any new cross-service dependency in this codebase should follow the same pattern — define the interface where it's consumed.

## Tenant tiers as a closed enum with a config-driven enterprise override
**Decision (observed):** `auth.Tier` is a fixed Go enum (`oss`, `cloud_free`, `cloud_paid`, `enterprise`) with default limits in code, and only `enterprise` supports per-tenant overrides via the `ENTERPRISE_LIMITS` env var.
**Why (inferred):** Keeps the common case (three fixed tiers) simple and compile-time-checked, while giving the one tier that genuinely needs customization (enterprise) a config escape hatch instead of a full database-backed tier system.
**Trade-off:** Adding a new tier requires a code change and redeploy, not a config change — acceptable given tiers change rarely; revisit if tier proliferation becomes frequent.

## How to Add to This File
When a real architectural decision is made going forward, write an ADR via `templates/adr.md`, then add a summary entry here and in `memory/project-decisions.md` so both the historical record (memory) and the reference doc (context) stay in sync.
