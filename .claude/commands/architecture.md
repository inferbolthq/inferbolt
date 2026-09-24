# /architecture — System Design Discussion

## Purpose
Reason about system architecture: new components, scaling a subsystem, evaluating a technology choice. Produces a design with explicit trade-offs, not just a diagram.

## Inputs
- A design question ("how should X scale to Y", "should we use Z for this") or a proposed change to system structure.

## Outputs
- A recommended design with explicit trade-offs against at least one alternative.
- Scalability, security, and performance implications spelled out.
- A recommendation on whether this warrants an ADR (`templates/adr.md`).

## Workflow
1. Clarify the actual requirement: expected scale (requests/sec, data volume, tenant count), consistency needs, latency budget.
2. Survey existing architecture (`context/architecture.md`) — prefer extending established patterns (River for async, pgx/Postgres for durable state, OTel for observability) over introducing a new technology without cause.
3. Propose a design; explicitly discuss CAP-theorem trade-offs where relevant (e.g., job state consistency vs. availability during a Postgres failover).
4. Call out failure modes: what happens when a downstream dependency is slow/down, what the blast radius of a bad deploy is.
5. Discuss cost implications (infra cost, operational complexity) alongside technical merit — the "best" design is the one that's sustainable for this team's size.
6. If the discussion surfaces a real decision, write it up with `templates/adr.md` and record it in `memory/project-decisions.md`.

## Examples
```
/architecture how should the orchestrator handle a River outage
/architecture should the drift detector read from TimescaleDB continuous aggregates or raw tables
/architecture design multi-tenant isolation for a future dedicated-GPU tier
```

## Best practices
- Never present a single design as the only option when real alternatives exist — name the trade-off, then recommend.
- Ground scale discussions in actual numbers where possible (current tenant tiers/rate limits in the README config tables), not hypothetical "web scale."
- A good architecture answer explains what breaks first under load, not just what works at the target load.
