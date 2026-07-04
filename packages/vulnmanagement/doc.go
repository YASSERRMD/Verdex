// Package vulnmanagement is Phase 084: continuous detection,
// triage, and remediation-SLA tracking of vulnerable components --
// dependencies (SCA), this platform's own source (SAST), container
// images, and license compliance. It draws on the durable,
// hash-chained audit trail added in Phase 077 (packages/auditlog) and
// the role/permission model added in Phase 006 (packages/identity),
// composing them into a tenant-scoped, auditable vulnerability-
// management layer rather than duplicating either.
//
// # What is new here
//
//   - ScannerSource / Severity / Status / Finding (types.go): a
//     tenant-scoped Finding record -- scanner source (SCA/SAST/
//     container/license), affected package+version, Severity, a
//     CVE/advisory ID, discovered-at, and a guarded Status state
//     machine (Open -> Triaged -> Remediating ->
//     Resolved/AcceptedRisk/FalsePositive) via CanTransition, mirroring
//     packages/caselifecycle's and packages/threatmodel's
//     transition-guard shape by reference -- this package imports
//     neither (tasks 1-3).
//   - Engine.RecordFinding (engine.go): the ingestion side every
//     SCA/SAST/container/license scanner funnels its output through, a
//     single permission-gated, tenant-scoped, audited entry point
//     rather than a separate write path per scanner (tasks 1-3).
//   - TriageDecision / Engine.Triage (triage.go): a triage workflow
//     recording who decided what and why -- required non-blank Notes,
//     the deciding Actor, and the guarded Status transition applied --
//     mirroring packages/accessgovernance's Attest and
//     packages/signoff's explicit-acknowledgement pattern by reference
//     (task 5).
//   - RemediationDeadlineFor / IsSLABreached / FindingsPastSLA
//     (sla.go): severity-to-remediation-deadline SLA tracking (7
//     days for Critical, 30 for High, 90 for Medium, 180 for Low) with
//     real, injected-clock time logic -- a finding already in a
//     terminal Status is never considered breached, and a Severity
//     with no configured SLA is never considered breached (task 6).
//   - LicenseCheck / EvaluateLicense (license.go): a seeded
//     allow-list (MIT, Apache-2.0, BSD, ISC, MPL-2.0, etc.) /
//     deny-list (GPL, AGPL, LGPL, SSPL, etc.) classifier for a
//     dependency's declared SPDX license, defaulting any blank or
//     unrecognized license to LicenseNeedsReview rather than a silent
//     pass (task 7).
//   - Report / BuildReport / Engine.BuildDashboard (report.go): an
//     aggregation report -- counts by Severity/Status/ScannerSource
//     plus the current SLA-breach list -- mirroring
//     packages/compliance's Dashboard and packages/accessgovernance's
//     Certify/Report shape (task 8).
//   - AuditSink (audit.go): records every finding recorded and every
//     triage decision into packages/auditlog.Store -- the same
//     durable, hash-chained sink the rest of the platform already
//     writes to and queries. No second audit table.
//   - identity.PermViewVulnmanagement / PermManageVulnmanagement
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewThreatmodel/PermManageThreatmodel precedent from
//     Phase 083.
//   - .github/workflows/ci.yml: sca-scan (govulncheck, real,
//     call-graph-aware Go vulnerability scanning) and sast-scan
//     (gosec) both run continue-on-error -- see each job's own comment
//     for why this phase does not wire them into the hard CI Gate yet
//     -- and container-scan (a documented placeholder, since this
//     repository ships no Dockerfile yet -- see
//     packages/threatmodel/doc/Dockerfile.hardened for the reference
//     template a future phase would build against, and which does
//     gate the CI Gate, since it is a no-op that always succeeds)
//     (tasks 1-3).
//   - .github/dependabot.yml: weekly automated dependency-update PRs
//     across every packages/*/go.mod and services/*/go.mod, the npm
//     workspace root, and the GitHub Actions used in this repository's
//     workflows (task 4).
//
// # Persistence
//
// Unlike packages/compliance's shared Control catalogue,
// vulnmanagement_findings and vulnmanagement_triage_decisions are both
// genuinely per-tenant operational data -- a finding belongs to one
// tenant's deployment, not a shared reference catalogue -- so both
// tables carry tenant_id and get a tenant_isolation Row-Level-Security
// policy (migrations 000028/000029), following the exact
// Repository/PostgresXRepository/TenantScopedXRepository three-layer
// pattern packages/compliance and packages/privacy established.
//
// # What is explicitly reused, not duplicated
//
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/compliance's and packages/threatmodel's own
//     AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything vulnerability-management-specific.
//   - packages/caselifecycle's allowedTransitions-map + CanTransition
//     guard shape and packages/threatmodel's
//     allowedMitigationTransitions shape are the reference Finding's
//     Status state machine follows, not a dependency -- this package
//     imports neither.
//   - packages/accessgovernance's Attest and packages/signoff's
//     AcknowledgementConfirmation/explicit-acknowledgement pattern are
//     the reference Engine.Triage follows (an actor, a reason, a
//     recorded decision), not a dependency -- this package imports
//     neither.
//   - packages/compliance's Dashboard/BuildDashboard and
//     packages/accessgovernance's Certify/Report aggregation shape are
//     the reference Report/BuildReport follows, not a dependency --
//     this package does not import packages/compliance or
//     packages/accessgovernance.
//   - .golangci.yml's gosec linter (already gating lint-go) remains
//     the primary in-repo SAST signal for day-to-day development; the
//     standalone sast-scan CI job is a second, broader-but-
//     informational pass, not a replacement.
//
// See doc/vuln-management.md for the full write-up.
package vulnmanagement
