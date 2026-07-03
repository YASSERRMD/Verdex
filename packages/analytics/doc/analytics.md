# Analytics

`packages/analytics` implements Phase 074: operational and
case-portfolio analytics — caseload counts by state/category/
jurisdiction, a creation-date trend, reasoning-quality trends, and
role-gated token/cost views — plus CSV/JSON export, all scoped to a
tenant and gated on `identity` permissions.

This is the last phase of Part 6 (Case Management & Workflow). It is
deliberately a *composition* package: it aggregates case data it does
not own, and reuses two other packages' existing dashboards rather
than re-deriving their aggregation logic.

## What is newly aggregated here vs. composed elsewhere

| Concern | Source | New or composed? |
| --- | --- | --- |
| Caseload by state, category, jurisdiction, creation trend | `packages/caselifecycle.Repository.List` | **New** — `Aggregator.Aggregate` (`aggregate.go`) |
| Reasoning-quality trend by jurisdiction | `packages/reasoningeval.Dashboard.JurisdictionTrend` | **Composed** — `QualityComposer.QualityTrend` (`quality.go`) reshapes the result, no scoring logic re-implemented |
| Token usage / cost by provider, task type, day | `packages/accounting.DashboardAPI.GetTenantDashboard` | **Composed** — `UsageComposer.UsageView` (`usage.go`) returns the result unchanged, after its own access check |
| CSV/JSON export | — | **New** — `Export` (`export.go`), a direct `encoding/csv`/`encoding/json` writer |

Nothing in this package re-reads `reasoningeval`'s `QualityScore` store
or `accounting`'s `UsageRecord` store directly. It only ever calls
those packages' own already-built dashboard types.

## Why compose instead of duplicate

Both `reasoningeval.Dashboard` and `accounting.DashboardAPI` already
describe themselves as "dashboard APIs" from their own phases (062 and
017 respectively). Re-implementing their grouping/averaging logic here
would mean two places could disagree about, say, a jurisdiction's
average quality score. Instead:

- `QualityComposer` holds a `*reasoningeval.Dashboard` and calls
  `JurisdictionTrend`, converting its `map[string]JurisdictionSummary`
  result into a sorted `[]QualityTrendPoint` — the only thing this
  package adds is a stable, displayable ordering.
- `UsageComposer` holds a `*accounting.DashboardAPI` and calls
  `GetTenantDashboard`, returning its `*accounting.TenantDashboard`
  completely unmodified, after enforcing `costPermission` (see below —
  `accounting.DashboardAPI` itself has no permission check, since
  `packages/accounting` does not depend on `packages/identity`).

## Caseload aggregation

`Aggregator.Aggregate(ctx, tenantID, filters)` calls
`caselifecycle.Repository.List(ctx, tenantID, filter)` — the same
method `packages/casesearch`'s query engine reads through — and groups
the returned `[]*caselifecycle.Case` by:

- `State` → `[]StateCount`
- `CategoryID` → `[]CategoryCount`
- `JurisdictionID` → `[]JurisdictionBreakdown`, each further broken
  down by `State` (so "12 active, 3 under review" is visible per
  jurisdiction without a second query)
- `CreatedAt` truncated to a UTC calendar day → `[]DailyCaseCount`
  (`CreatedTrend`), covering every distinct day present in the
  matching cases rather than a fixed trailing window — case creation
  volume is much lower than token usage, so accounting's fixed 7-day
  window would be too narrow here.

`Filters` mirrors `caselifecycle.CaseFilter`/`casesearch.Filter`'s
"zero value means unrestricted" convention and converts directly to a
`caselifecycle.CaseFilter` via `toCaseFilter()`. This package does not
duplicate `casesearch`'s text/semantic query construction — it only
needs `List`'s tenant+filter scoping, which `caselifecycle.Repository`
already exposes.

## Access scoping

Every `Dashboard`/`Aggregator`/`QualityComposer`/`UsageComposer` method
requires `ctx` to carry an authenticated `identity.User` (see
`access.go`):

- **Caseload and quality-trend views** require `identity.PermViewCase`
  — the same permission `caselifecycle`, `casesearch`, and
  `reasoningeval` already gate case-scoped reads on. For quality
  trends this check happens inside the delegated
  `reasoningeval.Dashboard.JurisdictionTrend` call itself; this
  package does not duplicate it.
- **Usage/cost views** require `identity.PermAuditRead`. Per
  `identity.PermissionMatrix`, only `RoleJudge`, `RoleAdmin`, and
  `RoleAuditor` hold this permission — `RoleAdvocate` and `RoleClerk`
  do not. This directly implements the phase's "only certain roles see
  cost/usage views" requirement: an advocate can see caseload and
  quality metrics but gets `ErrForbidden` from `UsageView`.

Tenant isolation is enforced by `Aggregate` requiring a non-nil
`tenantID` and passing it straight through to
`caselifecycle.Repository.List`, which itself refuses cross-tenant
reads (`caselifecycle.ErrCrossTenantAccess`).

## Dashboard facade

`Dashboard` (`dashboard.go`) is a thin, stable facade over an
`*Aggregator`, `*QualityComposer`, and `*UsageComposer`, mirroring
`reasoningeval.Dashboard`'s and `knowledgeapi.KnowledgeAPI`'s "facade
over lower-level stores" style. `NewDashboardFromStores` is a
convenience constructor that wires all three from a
`caselifecycle.Repository`, a `reasoningeval.Store`, and an
`*accounting.InMemoryRepository`. A `Dashboard` built with a nil
`QualityComposer`/`UsageComposer` returns `ErrComposerNotConfigured`
from the corresponding method rather than panicking, so callers that
only need caseload metrics can construct a partial `Dashboard` with
`NewDashboard(aggregator, nil, nil)`.

An HTTP surface over `Dashboard` (mirroring
`packages/knowledgeapi/http.go`'s wrap-and-expose pattern) is left to a
future phase, same as `reasoningeval.Dashboard`'s own doc comment
states for itself.

## Export

`Export(metrics, format)` renders an already-computed `*Metrics` as:

- **JSON** (`FormatJSON`): `json.MarshalIndent` applied to `Metrics`
  directly. Round-trips through `json.Unmarshal` into an equivalent
  `Metrics` value (verified in `export_test.go`).
- **CSV** (`FormatCSV`): a single flat table with columns
  `section, key, sub_key, count`. `section` is one of `total`, `state`,
  `category`, `jurisdiction`, `jurisdiction_state`, or
  `created_trend`; every row is self-describing via its `section`
  value, so the whole `Metrics` fits in one parseable file rather than
  needing one CSV per breakdown.

This does **not** reuse `packages/reportexport`'s PDF/DOCX/Markdown
rendering pipeline. `reportexport` assembles a case narrative document
(facts, issues, analysis, citations); an analytics export is tabular
aggregate data, which a direct `encoding/csv`/`encoding/json` writer
fits more directly — this was an explicit "your call" in the phase
task list, and a case-narrative renderer would be the wrong shape for
a table of counts.

## Web dashboard

`apps/web/src/app/dashboard/page.tsx` fetches from four endpoints this
package's `Dashboard` is designed to back (the HTTP layer itself is a
future phase's concern, same as noted above):

- `GET /api/v1/analytics/caseload` → `Dashboard.Caseload`
- `GET /api/v1/analytics/quality-trend` → `Dashboard.QualityTrend`
- `GET /api/v1/analytics/usage` → `Dashboard.UsageView` (only rendered
  client-side for `admin`/`judge` roles, matching `costPermission`'s
  server-side gate — the client-side role check is a UX nicety, not
  the enforcement boundary, since the server still gates on
  `identity.PermAuditRead` regardless of what the client sends)

See `apps/web/src/components/dashboard/CaseloadPanel.tsx`,
`CategoryBreakdownPanel.tsx`, `JurisdictionBreakdownPanel.tsx`,
`QualityTrendPanel.tsx`, and `UsageCostPanel.tsx` for the UI, and
`apps/web/__tests__/DashboardPanels.test.tsx` for their tests.
