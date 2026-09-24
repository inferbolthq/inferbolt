# /interview — Interview Practice Mode

## Purpose
Run a system-design / coding-interview-style Q&A session, tied to real work just completed in this repo or to a general topic, following [../CLAUDE.md](../CLAUDE.md) §12 and [agents/interviewer.md](../agents/interviewer.md).

## Inputs
- Optional topic ("system design", "the orchestrator we just built", "Go concurrency"), or none — defaults to whatever was most recently implemented in the session.
- Optional difficulty ("junior", "senior", "staff") — defaults to progressive difficulty based on `memory/interview-progress.md`.

## Outputs
- One question at a time — never a batch, so the user actually has to think before seeing the next one.
- Silence after the question until the user answers.
- After the answer: a real evaluation (what was right, what was missed, how a FAANG/Stripe-caliber interviewer would push further), then the "model answer" if useful — not just validation.
- An update to `memory/interview-progress.md`.

## Workflow
1. Pick a question rooted in real, recent work when possible ("we just added the tenant rate limiter — how would you scale this to 10M tenants?") — concrete beats abstract.
2. Ask exactly one question. Wait.
3. When the user answers, challenge weak points with a follow-up before giving the answer away — real interviews probe, they don't just accept.
4. Cover the categories from CLAUDE.md §12 across a session: architecture rationale, scaling ("how would this handle 10x/100x"), complexity analysis, CAP-theorem trade-offs, alternative designs, "what would $BigCo do differently."
5. Escalate difficulty as the user demonstrates mastery on a topic; drop back down if they're consistently missing fundamentals — track this in `memory/interview-progress.md`.
6. End each round with a short note on what to review before the next session.

## Examples
```
/interview
/interview system design — staff level
/interview quiz me on what we just built in the drift detector
```

## Best practices
- Don't give the answer in the same breath as the question — that defeats the purpose.
- Reward correct instincts even if the terminology is off; correct terminology issues after substance is confirmed.
- Escalate realistically: "senior" questions probe trade-offs and failure modes; "staff" questions probe org-level and multi-year consequences, not just bigger numbers.
