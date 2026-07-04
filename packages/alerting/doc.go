// Package alerting is Phase 096: production visibility and on-call
// alerting for this platform. It draws on the structured logging,
// metrics registry, tracing, and health/readiness endpoints already
// built in Phase 003 (packages/observability), the SLO/error-budget
// evaluation added in Phase 093 (packages/reliability), the
// reasoning-quality regression detector added in Phase 062
// (packages/reasoningeval), the token/cost budget-threshold machinery
// added in Phase 017 (packages/accounting), and the notification
// delivery/inbox mechanism added in Phase 072 (packages/notifications),
// composing them into one alert-rule-evaluation, on-call-escalation,
// and runbook layer rather than duplicating any of them.
//
// # What is new here
//
//   - Catalogue / RegisterBusinessMetrics (metrics.go): a small,
//     named catalogue of business-relevant counters and gauges
//     ("cases_ingested_total", "opinions_signed_off_total",
//     "sar_requests_pending", etc.) registered through
//     observability.Registry -- this package never constructs its own
//     Prometheus registry or counter type; every metric it names is a
//     real Counter/Gauge obtained from the caller's existing
//     observability.Registry (task 1).
//   - DashboardDefinition / BuildDashboard (dashboard.go): a
//     structured, named-panel data model for a handful of named "key
//     flows" ("ingestion", "reasoning", "sign-off") -- each panel
//     names a metric from Catalogue by string, mirroring
//     packages/compliance.Dashboard/BuildDashboard's aggregation shape
//     and packages/accounting.DashboardAPI's per-flow summary idea. A
//     report/query type, not a UI; this package does not import
//     packages/analytics (task 2).
//   - Severity / AlertRule / AlertEvent / Engine.Evaluate (rule.go,
//     engine.go): a named rule with a condition (a threshold over a
//     named metric value, an SLO-breach signal, or a
//     quality-regression signal) and a Severity.
//     Engine.Evaluate(ctx, rule, currentValue) returns an AlertEvent
//     only when the rule's condition is met -- real threshold
//     comparison logic, tested against both firing and non-firing
//     inputs, not a stub that always fires (task 1, core mechanism
//     tasks 3-5 build on).
//   - SLOAlertRule / EvaluateSLOAlert (slo_alert.go): wraps
//     reliability.SLO/EvaluateSLO/ComputeErrorBudget/ErrorBudgetPolicy
//     directly -- feeds real Observations through
//     reliability.EvaluateSLO, computes an ErrorBudget from the
//     result, and produces an AlertEvent when the SLO is breached or
//     its error budget is exhausted per the wrapped
//     ErrorBudgetPolicy. This package does not redefine SLO or
//     error-budget math; it only decides when that already-computed
//     signal should become an AlertEvent (task 3).
//   - QualityAlertRule / EvaluateQualityAlert (quality_alert.go):
//     wraps reasoningeval.RegressionDetector/RegressionResult
//     directly -- runs the same baseline/current QualityScore
//     comparison packages/reasoningeval.QualityAlertChecker already
//     performs, and converts a Regressed RegressionResult into this
//     package's own AlertEvent (rather than reusing
//     reasoningeval.Alert's shape verbatim), so a
//     quality-regression alert can flow through the same
//     AlertRule/Severity/EscalationPolicy pipeline as every other
//     alert kind in this package. See "Alert vs AlertEvent" below for
//     why a second, structurally different type is warranted here
//     rather than reusing reasoningeval.Alert directly (task 4).
//   - CostAlertRule / EvaluateCostAlert (cost_alert.go): wraps
//     accounting.BudgetConfig/TokenUsage/evalLimits's public
//     equivalent directly -- calls into an accounting.BudgetChecker
//     and converts a "usage crossed the alert threshold" or
//     "hard-stop exceeded" signal into this package's own AlertEvent.
//     Does not duplicate budget tracking or threshold percentages;
//     accounting.InMemoryBudgetChecker remains the only place token
//     and cost usage is actually accumulated (task 5).
//   - EscalationPolicy / EscalationTier / Responder / Route
//     (escalation.go): an ordered list of responder tiers, each with
//     an escalation delay before the next tier is paged, plus a
//     Route(alert, policy, now) function that walks the tiers based
//     on how long the alert has been open -- real time-based
//     escalation logic, not a static "always page tier 1" stub
//     (task 6). Route names the Responder to hand the alert to;
//     actually delivering that hand-off is
//     packages/notifications.Service.Notify's job -- see
//     NotificationRecipientSink below.
//   - Runbook / RunbookStep (runbook.go): an ordered remediation
//     procedure attached to an AlertRule by name, structurally
//     identical in shape to packages/backupdr.Runbook/RunbookStep
//     (Phase 085) -- Order, Description, OwnerRole -- because a
//     remediation procedure and a DR procedure are the same kind of
//     artifact (an ordered, role-assigned checklist), just for a
//     different trigger. This package defines its own Runbook type
//     rather than importing packages/backupdr's, since the two
//     packages' Runbooks attach to unrelated parent concepts
//     (AlertRule vs a DataClass/DR scenario) and importing backupdr
//     solely for a two-field struct would add a dependency for no
//     shared behavior (task 7).
//   - SyntheticCheck / Prober / Engine.RunSynthetic (synthetic.go): a
//     named, on-demand (or externally scheduled) probe wrapping a
//     caller-supplied Prober function -- typically one that composes
//     with observability.Checker/ReadinessHandler by calling the same
//     health endpoint a caller's own /readyz already exposes -- with
//     pass/fail and latency recording. Real logic (a real clock, a
//     real timeout), tested with a fake probe function rather than a
//     live network call (task 8).
//   - AlertRuleRepository / AlertEventRepository /
//     EscalationPolicyRepository plus InMemory*/Postgres*/TenantScoped*
//     implementations (repository.go, inmemory_repository.go,
//     postgres_repository.go, tenant_scoped_repository.go): the usual
//     three-layer, tenant-scoped, RLS-backed persistence for AlertRule
//     definitions, fired AlertEvent history, and EscalationPolicy
//     configuration -- see "Persistence" below for why this phase adds
//     one, unlike Phase 093 (task on persistence, see below).
//   - identity.PermViewAlerting / identity.PermManageAlerting
//     (packages/identity/permission.go): the two fine-grained
//     permissions this package's Engine gates every write/read
//     operation on, following the exact PermViewBackupDR/
//     PermManageBackupDR precedent from Phase 085.
//
// # Alert vs AlertEvent: why this package does not just reuse
// reasoningeval.Alert or accounting.AlertEvent's type directly
//
// packages/reasoningeval.Alert and packages/accounting.AlertEvent are
// each scoped to exactly one alert kind (a quality regression, a
// budget crossing) and carry that kind's domain-specific payload
// (RegressionResult, token/cost totals) as first-class fields. This
// package's own AlertEvent is deliberately more general: it carries a
// RuleName, Severity, TriggerValue, and a free-form Detail string, so
// that an SLO breach, a quality regression, a cost overage, and a
// plain metric-threshold breach can all flow through the exact same
// Engine.Evaluate -> AlertEvent -> Route -> Runbook pipeline uniformly.
// QualityAlertRule and CostAlertRule each *compose with* (call into)
// reasoningeval's and accounting's existing detection logic -- neither
// re-implements regression detection or budget-limit math -- and then
// translate the already-computed domain result into this package's
// uniform AlertEvent shape, exactly as packages/notifications'
// existing ReasoningEvalAlertSink/AccountingAlertSink adapters
// translate those same upstream events into a Notification. This
// package's AlertEvent is one further translation step upstream of
// that: from "a domain package detected something" to "an alert rule
// fired", before notifications.Service ever gets involved.
//
// # Persistence: why this package has a SQL migration (unlike Phase 093)
//
// packages/reliability (Phase 093) explicitly skipped a migration
// because every type it defines -- CircuitBreaker state,
// IdempotencyGuard's cache, TrafficShifter's backend health, SLO
// Observations -- is live, in-process operational state with no
// tenant-facing historical value (see packages/reliability/doc.go).
// This package is different: an AlertRule definition, a fired
// AlertEvent, and an EscalationPolicy are exactly the kind of
// tenant-facing, queryable-after-the-fact record
// packages/notifications.Notification and
// packages/reasoningeval.Alert-consuming dashboards already are --
// an operator or auditor legitimately wants to ask "what alerts fired
// last week" or "what is our current escalation policy" days or
// months later, not just "what is happening right now in this
// process". That is the same reasoning packages/compliance (082),
// packages/privacy (081), and packages/backupdr (085) each gave for
// adding tenant-scoped, RLS-backed storage, and this phase follows it:
// alert_rules, alert_events, and escalation_policies each carry a
// tenant_id column and an RLS policy exactly like
// compliance_control_evidence.
//
// # What is explicitly reused, not duplicated
//
//   - observability.Registry/Counter/Gauge/Histogram (Phase 003)
//     remain the only metrics-registration machinery in this
//     codebase. RegisterBusinessMetrics takes a caller-supplied
//     Registry and registers named Counters/Gauges through it; this
//     package never constructs a *prometheus.Registry or a competing
//     Counter/Gauge interface of its own.
//   - observability.Checker/NamedChecker/ReadinessHandler (Phase 003)
//     remain the only HTTP liveness/readiness probe machinery in this
//     codebase. SyntheticCheck's Prober uses the identical
//     "func(ctx) error, nil means healthy" convention by reference
//     (the same convention packages/reliability.TrafficShifter's
//     HealthCheckFunc already follows), named independently in this
//     package rather than imported, so a caller can wrap its existing
//     /readyz checker as a Prober without this package depending on
//     packages/observability's HTTP types.
//   - reliability.SLO/Observation/SLOStatus/EvaluateSLO/ErrorBudget/
//     ComputeErrorBudget/ErrorBudgetPolicy (Phase 093) remain the only
//     SLO and error-budget computation in this codebase.
//     SLOAlertRule/EvaluateSLOAlert calls directly into these types;
//     this package does not recompute a success rate, a P95 latency,
//     or a consumed-budget fraction itself.
//   - reasoningeval.RegressionDetector/RegressionResult/QualityScore
//     (Phase 062) remain the only reasoning-quality regression
//     detection in this codebase. QualityAlertRule/EvaluateQualityAlert
//     calls directly into RegressionDetector.Compare; this package does
//     not recompute an average Overall score or a per-dimension drop
//     itself.
//   - accounting.BudgetConfig/BudgetChecker/TokenUsage (Phase 017)
//     remain the only token/cost budget tracking in this codebase.
//     CostAlertRule/EvaluateCostAlert calls directly into a
//     BudgetChecker.Check result; this package does not track daily
//     or monthly token/cost totals itself.
//   - notifications.Service/NotifyInput (Phase 072) remains the only
//     notification delivery and inbox mechanism in this codebase.
//     Route (escalation.go) only *decides* which Responder an
//     AlertEvent should be handed to next; actually delivering that
//     hand-off to a human is notifications.Service.Notify's job --
//     see NotificationRecipientSink, an adapter mirroring
//     packages/notifications/adapters.go's existing
//     ReasoningEvalAlertSink/AccountingAlertSink shape, converting an
//     AlertEvent+Responder into a NotifyInput. This package does not
//     import packages/notifications to avoid a two-way dependency
//     (packages/notifications already exists and does not import this
//     new package); NotificationRecipientSink is instead an interface
//     this package defines and packages/notifications (or any other
//     caller) can implement, exactly mirroring how
//     packages/reasoningeval.AlertSink and packages/accounting.AlertSink
//     are interfaces the *upstream* package defines for a downstream
//     notifier to implement.
//   - backupdr.Runbook/RunbookStep's Order/Description/OwnerRole shape
//     (Phase 085) is the structural precedent this package's own
//     Runbook/RunbookStep follows, not a dependency -- this package
//     does not import packages/backupdr.
//   - identity.Role/identity.Permission/identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything alerting-specific.
//
// See doc/monitoring.md for the full write-up, including a
// composition table and worked examples.
package alerting
