# /frontend — Frontend / Dashboard Work

## Purpose
Build or review frontend code for the planned dashboard surface (currently pre-implementation — see `context/roadmap.md`). Applies standard senior frontend practice so the dashboard starts on solid footing rather than accumulating debt from day one.

## Inputs
- Target: a UI feature/component/page, or a review of existing frontend code once it exists.

## Outputs
- Component code with clear state boundaries (local vs. shared/global state).
- Accessibility considered by default (semantic HTML, keyboard nav, ARIA where semantic HTML isn't enough, color contrast).
- Data-fetching code that handles loading/error/empty states explicitly — never just the happy path.

## Workflow
1. Clarify the data source: which gateway endpoint(s) back this view, what the auth/tenant-scoping requirement is for the calling user.
2. Decide state ownership: server state (fetched data, cached appropriately) vs. UI state (local component state) — don't mix them in one store.
3. Build the loading/empty/error states as first-class, not an afterthought bolted on at the end.
4. Apply accessibility basics unconditionally: semantic elements, labeled inputs, focus management on route change/modal open, sufficient color contrast — see `dataviz` skill guidance for any charts/metrics visualizations.
5. Keep components small and composable; a component that both fetches data and renders a complex layout should usually be split into a container + presentational pair.
6. Write component tests for interaction logic and critical rendering states, not just snapshot tests.

## Examples
```
/frontend design the job list + recommendation view against GET /v1/jobs
/frontend build a live benchmark progress view (polling or streaming)
/frontend review accessibility of the cost report table
```

## Best practices
- Never fetch data with tenant-sensitive info directly from the browser without going through the authenticated gateway API — no direct DB/service access from the client.
- Prefer boring, well-supported patterns (standard fetch/query libraries, semantic HTML) over cutting-edge frontend tech for a dashboard that needs to be maintained by a small team.
- For any chart/metric visualization, load the `dataviz` skill before writing chart code — it defines the palette and accessibility bar for this project's visuals.
