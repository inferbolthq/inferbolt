# /plan — Implementation Planning

## Purpose
Produce a step-by-step implementation plan for a non-trivial task before writing code — surfacing architectural decisions, risks, and sequencing while they're cheap to change.

## Inputs
- A feature, refactor, or system change description.
- Any known constraints (deadline, must-not-break, must-integrate-with-X).

## Outputs
- An ordered list of implementation steps, each small enough to verify independently.
- Called-out decisions with trade-offs (not just one option presented as fact).
- Identified risks and how the plan mitigates them.
- A recommendation on where to pause for user confirmation (usually: after the architecture decision, before mass code generation).

## Workflow
1. Restate the goal in one or two sentences to confirm shared understanding.
2. Identify unknowns and ask clarifying questions rather than assuming.
3. Survey the current state: relevant files, existing patterns (`context/architecture.md`, `context/design-decisions.md`), and any prior related decisions (`memory/project-decisions.md`).
4. Enumerate 2-3 viable approaches only when the choice is genuinely consequential (data model, protocol, sync vs. async) — otherwise pick the obvious approach and move on.
5. Break the chosen approach into ordered, independently verifiable steps (schema → domain → transport → tests → docs is the usual shape here).
6. Flag risk points explicitly: migrations that can't be rolled back easily, changes to shared interfaces, anything touching tenant isolation or auth.
7. Present the plan for confirmation before large-scale code generation on anything architecturally significant.

## Examples
```
/plan add multi-region support to the orchestrator
/plan migrate the job queue from a hand-rolled table to full River usage
/plan design the cost model for cost/ before implementing it
```

## Best practices
- A plan is a communication tool, not busywork — keep it as short as the task allows while still surfacing real decisions.
- Don't plan what's obvious; do plan what's risky, ambiguous, or expensive to reverse.
- Revisit the plan if implementation reveals the initial assumptions were wrong — update it, don't silently deviate.
