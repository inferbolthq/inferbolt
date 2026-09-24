# /explain — Code & Concept Explanation

## Purpose
Explain existing code, a concept, or a design decision clearly and at the right depth for the asker — teaching mode by default (see [../CLAUDE.md](../CLAUDE.md) §13 and [agents/mentor.md](../agents/mentor.md)).

## Inputs
- Target: a file/function/flow to explain, or a concept (e.g., "how does River guarantee at-least-once delivery", "what is CAP theorem in context of our Postgres failover").

## Outputs
- A layered explanation: what it does, why it's built this way, when the pattern applies elsewhere, common pitfalls.
- For code: trace the actual execution path in this repo, not a generic textbook description divorced from the real implementation.

## Workflow
1. Identify the actual depth needed — a quick "what does this function do" vs. a full concept teach-in are different asks; match effort to the question.
2. For code explanations: read the real code and trace real call paths (e.g., "here's what happens from `POST /v1/jobs` through middleware to `internal/jobs/state.go`"), don't paraphrase from the function name alone.
3. For concept explanations: cover what it is, why it exists, when to use it, when not to, alternatives, common mistakes, and how it typically comes up in interviews — per Learning Mode.
4. Tie the explanation back to this codebase's real usage wherever possible (e.g., explain idempotency using the River job handlers already in `internal/jobs/worker.go`, not a hypothetical).
5. Update `memory/learning-progress.md` when a new concept is introduced this way.

## Examples
```
/explain how does the tenant rate limiter enforce per-tier limits
/explain what is a hypertable and why does TimescaleDB use one for metrics
/explain the trade-off between River (Postgres-backed) and a dedicated queue like SQS/Kafka
```

## Best practices
- Ground abstract concepts in this repo's concrete instances whenever one exists — it's more memorable and immediately useful.
- Don't over-explain what's already well understood — read the room (or the question's specificity) to calibrate depth.
- If the explanation reveals the code doesn't do what its name/comment implies, say so — that's a bug or a doc gap, not just a teaching moment.
