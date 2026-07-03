// Package analytics provides operational and case-portfolio analytics
// over a tenant's case load: counts by lifecycle state, category, and
// jurisdiction, trends over time, and — where the caller is authorized
// — reasoning-quality and token/cost views.
//
// # What this package aggregates itself
//
// Caseload metrics (Metrics, StateCount, CategoryCount,
// JurisdictionBreakdown, DailyCaseCount) are new aggregations computed
// here, in Aggregate, by walking packages/caselifecycle.Repository.List
// results (the same repository method packages/casesearch's query
// engine reads through) and grouping by State, CategoryID, and
// JurisdictionID. This is genuinely new logic: neither caselifecycle
// nor casesearch expose a "count cases by state/category/jurisdiction"
// view of their own.
//
// # What this package composes rather than duplicates
//
//   - Reasoning-quality trends (QualityTrend) call directly into an
//     already-constructed packages/reasoningeval.Dashboard —
//     specifically its JurisdictionTrend method — and reshape the
//     result into a []QualityTrendPoint. No scoring, rubric, or
//     aggregation logic is reimplemented; this package only adds the
//     "one point per jurisdiction, sorted for display" framing that
//     reasoningeval.Dashboard itself does not provide.
//   - Usage and cost views (UsageView) call directly into an
//     already-constructed packages/accounting.DashboardAPI's
//     GetTenantDashboard method and return its TenantDashboard
//     unmodified. No token/cost aggregation is reimplemented here.
//
// # Access scoping
//
// Every Dashboard method requires ctx to carry an authenticated
// identity.User (see access.go). Caseload, category, jurisdiction, and
// quality-trend views require identity.PermViewCase — the same
// permission packages/caselifecycle, packages/casesearch, and
// packages/reasoningeval already gate case-scoped reads on. The
// usage/cost view additionally requires identity.PermAuditRead, so
// only roles with audit/financial oversight (judge, admin, auditor —
// see packages/identity's PermissionMatrix) can see token spend;
// advocate and clerk can see caseload and quality metrics but not cost.
//
// # Export
//
// Export renders an already-computed Metrics as CSV or JSON (see
// export.go). It writes directly with encoding/csv and encoding/json
// rather than reusing packages/reportexport's PDF/DOCX rendering
// pipeline, since analytics exports are tabular data, not a
// case-narrative document — see export.go's doc comment for the fuller
// rationale.
//
// See doc/analytics.md for the full design, including which existing
// package accounts for each dashboard section.
package analytics
