# memory/

Working state for whoever is developing this repo — human or assistant. The
charter ([../CLAUDE.md](../CLAUDE.md) §14) asks that it be read at the start of
a significant piece of work and updated at the end of one.

**The contents of this directory are deliberately not committed.** They are one
contributor's running notes, not project documentation. Anything here that
becomes true of the project itself belongs in [`../context/`](../context/), the
README, or [`CHANGELOG.md`](../../CHANGELOG.md), which are committed.

If you are starting fresh, create the files as you need them:

| File | Holds |
| --- | --- |
| `current-state.md` | Where things stand right now: branch, what shipped, what is in flight. Read first, written last. |
| `active-tasks.md` | Work underway, and anything blocked on a decision. |
| `project-decisions.md` | Decisions with real trade-offs, and why they went the way they did. Check before contradicting one. |
| `technical-debt.md` | Debt as it is introduced or discovered: what it is, why it was accepted, what paying it down costs. |
| `known-bugs.md` | Confirmed bugs — symptom, root cause, fix, regression-test status. Kept after they are fixed, as precedent. |
| `completed-features.md` | What has actually shipped, as opposed to what was planned. |
| `architecture-history.md` | Structural changes over time. |
| `future-improvements.md` | Ideas not yet committed to. Distinct from `active-tasks.md`. |

The charter also references `interview-progress.md` and `learning-progress.md`,
used by the `/interview` and `/explain` workflows. Those track one person's
practice and are of no use to anyone else.
