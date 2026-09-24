# /backend — Backend Service Work

## Purpose
Implement or review backend logic across the Go services (`gateway`, `orchestrator`, `router`, `collector`, `operator`) and the Python `worker`, following the layering conventions already established.

## Inputs
- Target: a service, package, or specific piece of business logic.

## Outputs
- Implementation respecting the `cmd/` (wiring) vs. `internal/` (logic) split.
- Interfaces defined at the consumer for cross-package/service dependencies.
- Tests colocated with the code.

## Workflow
1. Identify which service owns this responsibility — don't duplicate logic across `orchestrator` and `router` when one should own it and the other should call it.
2. Respect statelessness: no service-local state that isn't safely reconstructable from Postgres/TimescaleDB on restart.
3. For anything long-running or retryable (benchmark dispatch, drift scans), use River (`internal/jobs`, `internal/queue`) rather than a bespoke goroutine/background loop.
4. Propagate `context.Context` through the full call chain for cancellation and trace propagation (OTel) — never drop it or substitute `context.Background()` mid-chain.
5. For the Python worker: keep engine adapters (`worker/engines/*.py`) conforming to the same interface/protocol as the existing `vllm_engine.py` and mock engine, so the orchestrator can treat them interchangeably.
6. Handle partial failure explicitly: what happens if the orchestrator dies mid-dispatch, if a worker crashes mid-benchmark, if Postgres is briefly unavailable.

## Examples
```
/backend implement the SGLang engine adapter
/backend add a River-based scheduled job for baseline recomputation
/backend wire drift detector alerts through the existing Slack notifier
```

## Best practices
- New cross-service calls go through a defined interface, mirroring `OrchestratorClient` in `internal/gateway/interfaces.go`.
- Idempotency matters for anything River will retry — a job handler must be safe to run twice.
- Keep Python and Go boundaries HTTP-only, as documented in the architecture — no shared memory or filesystem coupling between the two runtimes.
