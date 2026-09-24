# /debug — Root-Cause Debugging

## Purpose
Find and fix the root cause of a bug — not the symptom. Every fix ships with a regression test and an entry in `memory/known-bugs.md` (resolved) or `memory/technical-debt.md` (if a full fix is deferred).

## Inputs
- A failure description: error message, stack trace, failing test, or reproduction steps.
- Any relevant logs, request/response payloads, or environment details (which service, which tenant tier, which engine).

## Outputs
- Root cause identified and explained in plain terms (what, where, why it manifests).
- A minimal, targeted fix — not a defensive rewrite of surrounding code.
- A regression test that fails before the fix and passes after.
- An update to `memory/known-bugs.md`.

## Workflow
1. Reproduce first. If you can't reproduce it, say so explicitly and ask for more detail (exact request, tenant/tier, timing) rather than guessing at a fix.
2. Form a hypothesis from the evidence (error message, code path, recent commits touching that area — `git log -p -- <path>`).
3. Trace the actual execution path: read the handler → domain logic → data layer chain rather than pattern-matching on the error string alone.
4. Isolate the root cause with the narrowest possible test or manual repro — a single failing assertion beats "the whole flow seems off."
5. Fix at the root cause. If the bug is a missing validation, add validation — don't add a downstream null-check that papers over invalid state entering the system.
6. Write the regression test first (it should fail on the old code), then confirm the fix makes it pass.
7. Check whether the same class of bug exists elsewhere (same pattern in a sibling handler/engine adapter) — mention it even if out of scope to fix now.

## Examples
```
/debug jobs stuck in "pending" state after river dispatch
/debug 500 on POST /v1/jobs when workload_config.concurrency is 0
/debug drift detector flags false positive at low sample counts
```

## Best practices
- Resist the urge to fix by adding a `try/except` or a defensive `if nil` around the symptom — that hides the bug, it doesn't fix it.
- A bug with an unclear repro is a research task, not a coding task — say so and propose how to gather the missing signal (add logging, add a metric, ask for a payload).
- If the root cause reveals a design flaw, name it and propose the fix as a separate, explicit follow-up rather than smuggling a redesign into a "bug fix."
