# Security Policy

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
discussions, or pull requests.**

Report them privately through
[GitHub's private vulnerability reporting](https://github.com/inferbolthq/inferbolt/security/advisories/new),
which notifies the maintainers without disclosing the issue publicly.

Please include as much of the following as you can:

- The type of issue (for example: cross-tenant data access, authentication bypass,
  SQL injection, secret exposure in logs).
- The affected component — gateway, orchestrator, router, collector, operator, CLI,
  or Python worker.
- Paths of the source files involved, and the commit or tag you tested against.
- Step-by-step reproduction instructions, ideally with the exact HTTP request or CLI
  invocation.
- The impact: what an attacker gains, and what access they need to start.

You should get an initial response within a few days. We will keep you informed as
we work on a fix and will credit you in the advisory unless you would rather stay
anonymous.

## Supported versions

InferBolt is pre-1.0 and under active development. Security fixes land on the
default branch and in the next tagged release. There is no long-term support branch
yet.

| Version | Supported |
| ------- | --------- |
| `master` (unreleased) | ✅ |
| Tagged `0.x` releases | Latest tag only |

## Scope

The areas most worth your attention, because they carry the security properties this
project depends on:

- **Tenant isolation.** Every tenant-scoped query must filter by `tenant_id`, and a
  tenant must never be able to reference another tenant's `job_id` or campaign. A
  cross-tenant read is a high-severity finding here.
- **Authentication and authorization.** JWT-scope handling in `internal/auth/`, and
  API key issuance and verification.
- **Injection.** All SQL goes through `pgx` parameterized queries; a
  string-concatenated query is a finding even if it is not currently reachable.
- **Secret handling.** `JWT_SECRET`, database credentials, `SLACK_WEBHOOK_URL`, and
  `ANTHROPIC_API_KEY` are environment-injected. Any path that logs one, bakes one
  into a Docker layer, or returns one in an API response is a finding.
- **The optimization agent.** Its tool surface is deliberately closed — four reads
  and one action, with no shell and no filesystem access. Anything that widens that
  surface, or that lets campaign input reach a tool argument unvalidated, is worth
  reporting.

## Known gaps

These are already documented and are **not** something you need to report — they are
tracked work, not undiscovered vulnerabilities:

- **No pre-authentication rate limiting.** The only limiter runs after
  authentication, so unauthenticated requests and failed auth attempts are unbounded.
- **Drift baselines are global, not per-tenant.** `public.baselines` has no tenant
  dimension, so tenants benchmarking the same model share a baseline and can
  influence each other's alerting.
- **The rate limiter is per-process.** Running more than one gateway replica
  multiplies the effective limit.

## Deployment guidance

If you are running InferBolt yourself:

- `JWT_SECRET` must be at least 32 bytes and unique per deployment. The gateway
  refuses to start otherwise.
- Never expose the orchestrator's internal endpoints (`/internal/*`) outside your
  cluster network.
- `ANTHROPIC_API_KEY` belongs to the orchestrator only. No client needs it.
- Run the stack behind a reverse proxy that terminates TLS. Nothing in this project
  serves HTTPS directly.
