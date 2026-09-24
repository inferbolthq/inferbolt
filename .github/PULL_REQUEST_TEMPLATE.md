## What this changes

<!-- What does this do, and why? Link the issue it closes, if there is one. -->

Closes #

## How it was tested

<!--
Which tests you added or ran, and anything you verified by hand. If a change
touches SQL, say whether it was tested against a real Postgres — mocks cannot
verify query correctness.
-->

## Checklist

- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
- [ ] `make test` and `make lint` pass
- [ ] New logic has tests; any bug fix has a regression test that would have caught it
- [ ] Every new tenant-scoped query filters by `tenant_id`
- [ ] Any new endpoint is authenticated and rate-limited (`/health` is the only exception)
- [ ] No secrets, API keys, or tokens in code, logs, or fixtures
- [ ] Errors are wrapped with context; none are silently swallowed
- [ ] New environment variables are documented in the README and `context/deployment.md`
- [ ] Docs updated in this PR, not deferred — README, `context/`, `docs/` as applicable
- [ ] `CHANGELOG.md` updated under `## [Unreleased]` if this is user-facing

## Edge cases considered

<!--
Briefly: empty input, maximum-size input, concurrent access, partial failure
(orchestrator down, DB timeout), malformed payload. Say which apply and how they
behave.
-->

## Performance and scalability

<!--
Only if this touches a hot path — gateway request handling, orchestrator dispatch,
drift queries. State the time complexity if it is not obvious, note any new query
and its index, and say what load test would validate the change.

Does this assume a single instance? Does it hold state that is unsafe to lose on
restart?
-->
