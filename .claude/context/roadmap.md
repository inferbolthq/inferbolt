# Roadmap

> **Rewritten 2026-09-14.** Replaces the 2026-09-10 flat "Next" list with a phased
> plan. Three entries from that list were already done and have been moved to Done.
> The README's Status section is the same claim in prose — keep them in sync.

Sizing is relative, not calendar: **S** = an afternoon, **M** = a day or two,
**L** = a week or more, **?** = blocked on a decision, size unknowable until made.

---

## Done

- [x] Repo scaffold
- [x] API gateway — JWT-scope auth, flat per-tenant rate limit, boundary validation
- [x] Python workers — vllm, sglang, llamacpp, ollama, mock adapters
- [x] Go orchestrator — River queue, worker registry, dispatch, result ingestion
- [x] Drift detection — baselines, detector, Slack notifier, collector loop
- [x] TimescaleDB metrics (superseding the original ClickHouse plan)
- [x] `inferbolt run` — one-shot local benchmark with no manual setup
- [x] Read-only dashboard at `/dashboard`
- [x] **`engine_config` end to end** (2026-09-10) — the prerequisite for comparing
      configurations at all
- [x] **Optimization agent** (2026-09-10) — `inferbolt agent "<goal>"`
- [x] **Durable server-side campaigns** (2026-09-10) — migration 007,
      `internal/campaigns`, `inferbolt campaigns list|get|follow|cancel`
- [x] **UI campaign list + live campaign view** (2026-09-10) — `ui/src/pages/Campaigns.tsx`,
      `CampaignDetail.tsx`, `hooks/useCampaign.ts`, bindings in `api/client.ts`.
      *Was listed as Next on 2026-09-10; it had already landed in `3c8d291`.*

---

## Phase 0 — Prove it, then preserve it

Nothing below this line is worth building until Phase 0 is done. The agent has
never executed against a real platform, and none of it is on `master`.

- [ ] **Run one real campaign end to end** — S, but it is the whole ballgame.
      No Docker daemon has ever been reachable in a build session, so
      `docker compose up` → worker registers → agent plans → real benchmark →
      recommendation has not happened once. The 19 tests in `internal/agent` run
      against a scripted planner and a fake platform: they prove the loop's control
      flow and nothing about whether the prompt survives real tool output.
      Use the mock engine — no GPU needed, exercises the entire loop:
      `docker compose down -v && docker compose up -d`, export `ANTHROPIC_API_KEY`,
      then `./inferbolt agent "find the best throughput config" --model test-model
      --gpu cpu --engines mock --max-trials 4`.
      **Expect to find prompt bugs here** — that is the point of running it.
- [ ] **Merge and push `feat/optimization-agent`** — S. 19 commits ahead of
      `master`, 0 behind, never pushed. Do this after the campaign above passes,
      not before: merging an unproven agent is how it becomes permanent.
- [ ] **Migration runner** — M. `docker-compose` mounts `migrations/` at
      `/docker-entrypoint-initdb.d`, which Postgres runs *only* when initializing a
      fresh data directory. 006 and 007 therefore do not apply to any existing
      volume, and the failure is silent until a query hits a missing table. Every
      future migration inherits this bug. `golang-migrate`, or a small
      `schema_migrations` table in the orchestrator's startup path — but stop
      hand-applying DDL.

---

## Phase 1 — Multi-tenant correctness

These are wrong *today* for more than one tenant. Cheap now, expensive once
anyone depends on the current behavior.

- [ ] **Per-tenant drift baselines** — ? then M. `public.baselines` is
      `PRIMARY KEY (engine, model)` with no tenant dimension, and the detector reads
      `metrics.bench_results` unscoped. Two tenants benchmarking the same model on
      different hardware write into one shared baseline, so one tenant's results can
      fire *or suppress* another's Slack alerts. **Needs a product decision first:**
      per-tenant, per `(tenant, engine, model, gpu_profile)`, or deliberately global?
      Then a migration plus a detector change. Open in `known-bugs.md`.
- [ ] **Integration tests for `internal/campaigns/store.go`** — M. ~300 lines of SQL
      at 17.4% package coverage. Those queries carry both the tenant filter and the
      conditional claim that prevents double-running a campaign — exactly the logic
      the charter's integration-test rule exists for. Use the `docker-compose.yml`
      Postgres, not mocks.
- [ ] **Pre-auth (IP) rate limiting** — S. The only limiter runs *after* auth, so
      `/health`, `/dashboard` and failed auth attempts are unbounded. An IP-keyed
      `golang.org/x/time/rate` limiter ahead of the auth group; the dependency is
      already present.
- [ ] **Shared rate-limiter state** — M. Buckets live in a process-local `sync.Map`,
      so N gateway replicas enforce `100/min × N`, silently. This contradicts
      CLAUDE.md §1 ("every service must be horizontally scalable"). Fix **before**
      any Helm change sets `replicas > 1`.

---

## Phase 2 — Finish the benchmarking product

- [ ] **Cost model** — L. Root `cost/` is a `.gitkeep`. `worker/cost/modeler.py` is
      wired into `worker/main.py` but prices off a hardcoded GPU table, and
      `internal/agent/budget.go` prices planning off a second hardcoded table. Both
      drift. A benchmark platform that cannot say "this config is 30% cheaper" is
      leaving its main claim on the table.
- [ ] **Decide the Optuna sweep's fate** — ?. `worker/search/config_search.py` is
      complete and imported by nothing. The agent covers the same ground. Either
      wire it as a second, cheaper campaign mode (a deterministic sweep costs no
      tokens — a real choice, not just cleanup) or delete it.
- [ ] **Decide gRPC's fate** — ?. `proto/inferx/v1/` and `gen/` still define a
      service and `make proto` still works, but nothing serves it — the server was
      part of the deleted gateway path. It looks live and is not. Reimplement
      against `internal/gateway.Handler` (real work; that handler is HTTP-shaped) or
      delete `proto/`, `gen/`, and the `make proto` target.
- [ ] **Pick one dashboard** — ? then M. The embedded Go page
      (`internal/gateway/static/dashboard.html`) and the React app under `ui/`
      overlap. Keeping both means maintaining both. The React app has the campaign
      view; the embedded page needs no build step.
- [ ] **Browser auth** — ? then M. `ui/src/components/TokenGate.tsx` has the user
      paste an API key into `localStorage`. Fine for a dev tool, not for anything
      shared. CLAUDE.md §5 says settle CSRF before introducing cookie sessions — so
      decide the session model first.
- [ ] **First public benchmark report** — M. `docs/benchmarks/` is structure only.
      This is the artifact that makes the project legible to anyone outside it.
- [ ] **Coverage in the untested core** — L, ongoing. `internal/jobs`,
      `internal/queue`, `internal/metrics`, `internal/config` are at 0.0%;
      `internal/auth` at 15.0%. `jobs/state.go` is the declared single source of
      truth for job lifecycle and has no test at all.

---

## Phase 3 — Agentic platform

**A direction, not yet a commitment.** Written down because the stated intent is
that InferBolt be "mainly an agentic platform," and none of it exists today. What
exists is one agent with a deliberately closed five-tool surface — four reads, one
action, no shell, no filesystem — that optimizes benchmark configurations. That is
a feature of a benchmarking product, not a platform.

Sequenced so each step is useful on its own:

- [ ] **Agent audit trail** — M. Every tool call, its arguments, and its result,
      persisted per campaign and queryable. `campaign_events` is close, but it
      records steps for a human follower, not a reviewable record of what the agent
      did. Prerequisite for trusting an agent with anything beyond reads.
- [ ] **Per-tenant agent budgets and quotas** — M. Today `Budget` is per-campaign
      and client-supplied. A platform needs a tenant-level ceiling on tokens and
      trials that a campaign cannot exceed regardless of what it requests.
- [ ] **Scheduled / recurring campaigns** — M. "Re-optimize this model weekly, alert
      me if the recommendation changes" is the feature that connects the agent to
      the drift detector that already exists.
- [ ] **A second agent type** — L. One agent is a feature; the second one forces the
      real abstraction (shared loop, tool registry, per-agent prompts and tool sets).
      Do not build that abstraction before the second agent exists — per the
      charter's DRY rule, wait for the real duplication.
- [ ] **User-defined goals and tool sets** — L, and the point of no return. This is
      where the closed tool surface opens up, and where the security model has to be
      rewritten rather than extended. Do not start before the audit trail and the
      budgets are in place.
- [ ] **SSE or websocket event push** — S. Followers poll `?after_seq=` every 2s.
      Fine for one campaign, not for a dashboard watching many. The cursor design
      makes this additive rather than breaking.

---

## Decisions needed from the user

Six items above are blocked on a call only the user can make. Cheapest thing on
this page to unblock, most expensive to guess wrong:

1. Are drift baselines per-tenant, per `(tenant, engine, model, gpu_profile)`, or
   deliberately global?
2. Does the Optuna sweep become a second campaign mode, or get deleted?
3. Does gRPC come back, or do `proto/` and `gen/` go?
4. Which dashboard is the product — embedded Go, or the React app?
5. What is the browser session model, and therefore the CSRF posture?
6. Is Phase 3 the actual direction, or is InferBolt a benchmarking product with one
   good agent in it? Phases 0–2 are correct either way; Phase 3 is not.

---

## Before the repo goes public

Done in the 2026-09-25 open-source readiness pass: community health files,
issue/PR templates, Dependabot, golangci-lint + ruff, a CI that actually gates
(vet, lint, race, Python, UI, compose), the InferBolt rename, the MIT/Apache
license contradiction, and a history rewrite that dropped `.git` from 38 MB to
1.1 MB. What is left needs a decision or an account:

- [ ] **Decide the canonical remote.** `filter-repo` dropped `origin`, which was
      `github.com/karthikkay07/inferx` — matching neither `go.mod`
      (`github.com/inferbolthq/inferbolt`) nor the README badges. Either create
      `inferbolthq/inferbolt`, or change `go.mod` and every import to match the
      real home. **Nothing should be pushed until this is settled.**
- [ ] **Set the Code of Conduct enforcement contact.** `CODE_OF_CONDUCT.md`
      carries an explicit placeholder; a Contributor Covenant with no reporting
      address does not function.
- [ ] **Force-push the rewritten history**, once the remote is decided. Every
      SHA changed. Backup bundle:
      `~/inferbolt-backup-20260925-005208.bundle`.
- [ ] **Enable on GitHub:** private vulnerability reporting (SECURITY.md links
      to it), branch protection on the default branch requiring the CI jobs,
      and Discussions (the issue template config links to it).
- [ ] **Decide whether `master` or `feat/optimization-agent` is trunk.** Master
      is 19 commits stale. Publishing with master as the default branch would
      show a repo without the agent, campaigns, or the current README.

## Housekeeping

- [ ] CLAUDE.md §2 cites `internal/gateway/handler/response.go` as the shared
      response envelope; that file was deleted 2026-09-10. The charter is the
      user's file — needs their say-so to edit. Related: every handler now
      hand-rolls its own JSON shape, tracked in `memory/technical-debt.md`.
- [ ] Ratify or reject the `revive` doc-comment exclusion for `internal/`, which
      deviates from CLAUDE.md §2. See `memory/technical-debt.md`.
- [ ] Regression test asserting `"mock"` is in the gateway's engine allowlist
      (`known-bugs.md` records this as a `fixed-no-regression-test` gap).

---

## How to Use This File

Verify against the code before trusting any line here — `git log`,
`go test ./... -cover`, directory contents. This file has been wrong before: the
2026-09-10 revision listed three things as Next that were already committed.
Update this file and the README's Status section in the same change.
