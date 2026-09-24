# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While the project is pre-1.0, breaking changes may land in a MINOR release.

## [Unreleased]

### Added

- **Goal-directed optimization agent** — `inferbolt agent "<goal>"` plans a sequence
  of benchmarks with Claude, submits them through the ordinary authenticated API,
  reads the measurements back, and recommends an engine and configuration citing the
  trials it ran. The model plans; it never measures. Its tool surface is closed:
  four reads and one action, with no shell and no filesystem access.
- **Durable server-side campaigns** — campaigns run in the orchestrator as River
  jobs against `public.campaigns` and `public.campaign_events` (migration 007). They
  survive client disconnect, replay from any cursor, and can be cancelled between
  trials. `inferbolt campaigns list|get|follow|cancel`; Ctrl-C detaches rather than
  killing the work.
- **`engine_config` carried end to end** (migration 006) — the API, queue, store,
  gateway, and CLI now pass an engine configuration through to the Python worker,
  with validation at the boundary. Previously every benchmark ran with worker
  defaults, so no two configurations could be compared.
- **`inferbolt run`** — a one-shot local benchmark that starts the stack, bootstraps
  a credential, spawns a worker, and prints results with no manual setup.
- **Campaign UI** — campaign list and live campaign view in the React dashboard,
  polling the resumable event cursor.
- **Embedded read-only dashboard** at `/dashboard`, plus `GET /internal/workers` for
  worker listing.
- **Mock engine**, usable with no GPU, registered and conforming to the live
  `BaseEngine` contract.
- Contributor documentation: `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`,
  issue and pull request templates, and Dependabot configuration.

### Changed

- The Anthropic API key moved from the client to the **orchestrator**. No client
  needs `ANTHROPIC_API_KEY` any more.
- Campaign JSON is snake_case, and durations serialize as durations.
- The React dashboard adopted a consistent design system.
- CI now runs `go vet`, `golangci-lint`, the race detector, and the Python test
  suite, and verifies `go mod tidy` produces no diff instead of silently rewriting
  `go.mod` during the build.

### Fixed

- **Cross-tenant data exposure in `GET /v1/metrics`** — any authenticated caller
  with `metrics:read` could read another tenant's benchmark measurements, including
  throughput, TTFT, cost per million tokens, and engine configuration, by naming a
  public model. `metrics.bench_results` has no `tenant_id` column and the query
  filtered only on engine, model, and timestamp. Now joined through `public.jobs`
  and filtered by the authenticated tenant. Regression test:
  `TestGetMetrics_ScopesToTheAuthenticatedTenant`.
- `CreateJob` rejected `auto_route: true` without an explicit `engines` list,
  because the empty-engines check ran before the auto-route branch that populates it.
- The dashboard could not reach the gateway at all.
- Compose healthchecks invoked `curl`, which the images do not contain.
- Repo-wide build and vet failures across `cmd/collector`, `cmd/router`,
  `cmd/operator`, and `internal/drift` — including a real compile error in the
  operator's `reconcileDeployment` and a float-equality test bug that had been hidden
  by a package that would not compile.
- Timeout tuning for River jobs, worker dispatch, and inference engine startup.

### Removed

- **The dead tier-based gateway implementation** (13 files) — a complete second
  gateway that no binary ever referenced. The gRPC server went with it, since it
  depended on the dead path's handler. `proto/` and `gen/` remain with no server.
- `ui/node_modules` is no longer tracked.

### Security

- See the cross-tenant `GET /v1/metrics` fix above.
- `JWT_SECRET` no longer has a `change-me-in-production` default; the gateway
  requires the variable and rejects secrets shorter than 32 bytes.

## [0.1.0] - 2026-06-04

Initial tagged release: API gateway with JWT-scope authentication, Go orchestrator
with a River job queue and worker registry, Python worker adapters for vLLM, SGLang,
llama.cpp and Ollama, TimescaleDB metrics storage, and drift detection with Slack
alerting.

[Unreleased]: https://github.com/inferbolthq/inferbolt/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/inferbolthq/inferbolt/releases/tag/v0.1.0
