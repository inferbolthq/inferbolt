# Workflow: System Design

## Inputs
- A design question or a proposed new component/system change.

## Outputs
- A recommended design with explicit trade-offs, and (if consequential) an ADR.

## Step-by-Step Process
1. **Get real numbers** — expected scale (rps, data volume, tenant count), latency budget, consistency requirements. Don't design against a vague "web scale" target.
2. **Survey existing architecture** (`context/architecture.md`) — prefer extending established patterns (River, Postgres/TimescaleDB, OTel, consumer-defined interfaces) over introducing new infrastructure without cause.
3. **Generate 2-3 candidate designs** when the choice is genuinely consequential; pick the obvious one and move on when it isn't.
4. **Analyze failure modes** for each candidate — what happens when each dependency is slow/down; what's the blast radius of a bad rollout.
5. **Reason explicitly about CAP-theorem trade-offs** where relevant — does this data need strong consistency or can it tolerate eventual consistency (see [../knowledge/system-design-notes.md](../knowledge/system-design-notes.md)).
6. **Recommend, with stated trade-offs** — not just one option presented as the only choice.
7. **Write an ADR** (`templates/adr.md`) if the decision is expensive to reverse or cross-cutting; log it in `memory/project-decisions.md`.

## Review Checklist
- [ ] Real scale numbers used, not assumed
- [ ] At least one real alternative considered and explained
- [ ] Failure modes and blast radius discussed
- [ ] CAP/consistency trade-offs addressed where relevant
- [ ] ADR written for consequential decisions

## Success Criteria
The design survives a "what breaks at 10x" question, and the trade-offs are explicit enough that a future engineer can understand why this design was chosen over the alternatives.
