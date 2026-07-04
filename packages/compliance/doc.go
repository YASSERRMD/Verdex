// Package compliance is Phase 082: mapping this platform's controls to
// the legal/regulatory frameworks a deployment must answer to. It
// draws on the durable, hash-chained audit trail added in Phase 077
// (packages/auditlog), the role/permission model added in Phase 006
// (packages/identity), the non-binding-analysis guardrail added in
// Phase 057 (packages/guardrail), the data-residency model added
// around Phase 078 (packages/dataresidency), and the
// data-subject-rights layer added in Phase 081 (packages/privacy),
// composing them into a tenant-scoped, auditable compliance-mapping
// layer rather than duplicating any of them.
//
// # What is new here
//
//   - Framework / ControlCategory / Control (types.go): an open,
//     extensible named-framework type (not a closed enum -- a
//     deployment's applicable frameworks vary by jurisdiction and
//     customer) plus a catalogued Control: ID, Code, Title,
//     Description, Framework, Category, and a MappedTo list linking it
//     conceptually to platform features by string tag (e.g.
//     "packages/privacy.SAR", "packages/auditlog") -- reference only,
//     no import of the tagged packages implied (task 1).
//   - SeedControls (seed.go): a real starter catalogue -- seven
//     concrete UAE-DP-* controls (PDPL-style lawful basis,
//     data-subject rights, cross-border transfer, breach notification,
//     retention), five JRH-* judicial-records-handling controls
//     (retention, chain-of-custody, non-binding disclaimer
//     enforcement, sign-off/audit, case access), and four INTL-DP-*
//     controls mapped in as an applicable/reference overlay. Every
//     Description is framed as a mapped requirement category, never a
//     verbatim statutory citation this package cannot verify (tasks
//     2-4).
//   - ControlEvidence (evidence.go): a tenant-scoped evidence record
//     attached to a Control -- what proves it is satisfied (an
//     auditlog query, a test name, a doc link, a configuration
//     artifact), with tenant scoping and timestamps (task 5).
//   - Status / EvaluateControl / RunGapAnalysis (gap.go): real
//     per-control gap-analysis evaluation logic -- given a Control
//     catalogue and a tenant's collected ControlEvidence, each control
//     resolves to StatusSatisfied (>=2 distinct evidence kinds),
//     StatusPartiallyMet (some evidence, below that bar), or StatusGap
//     (none), not a stub that always reports satisfied (task 6).
//   - Profile / ApplicableControls (profile.go): a per-deployment
//     (tenant) selection of which Frameworks and Controls apply -- not
//     every deployment needs every framework, mirroring how
//     packages/jurisdiction and packages/reasoningprofile scope by
//     deployment (task 7).
//   - Dashboard / BuildDashboard (dashboard.go): an aggregation
//     report -- counts by framework and Status, plus a trailing
//     evidence-collection-velocity figure -- mirroring
//     packages/accessgovernance's Certify/Report shape. A report/query
//     type, not a UI (task 8).
//   - AuditSink (audit.go): records every control registration,
//     evidence addition, and profile change into
//     packages/auditlog.Store -- the same durable, hash-chained sink
//     the rest of the platform already writes to and queries. No
//     second audit table.
//   - identity.PermViewCompliance / identity.PermManageCompliance
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewPrivacy/PermManagePrivacy precedent from Phase
//     081.
//
// # What is explicitly reused, not duplicated
//
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/privacy's and packages/accessgovernance's own
//     AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything compliance-specific.
//   - packages/guardrail.RequireDisclaimer / CheckText (Phase 057)
//     remain the only non-binding-disclaimer enforcement mechanism in
//     this codebase; the JRH-03 seed control names them by MappedTo
//     tag as what satisfies it, rather than this package
//     reimplementing any disclaimer or verdict-language check.
//   - packages/privacy's SubjectAccessRequest, ErasureRequest,
//     ConsentRecord, and RetentionPolicy/EnforceRetention (Phase 081)
//     remain the only data-subject-rights and retention-enforcement
//     machinery; the UAE-DP-* and INTL-DP-* seed controls name them by
//     MappedTo tag as what satisfies each mapped requirement category,
//     rather than this package re-implementing SAR handling, erasure,
//     consent tracking, or retention evaluation.
//   - packages/dataresidency and packages/airgapped remain the only
//     cross-border-transfer/residency enforcement in this codebase;
//     UAE-DP-05's MappedTo names them by reference.
//   - packages/accessgovernance's Certify/Report aggregation shape is
//     the reference Dashboard/BuildDashboard follow, not a dependency
//     -- this package does not import packages/accessgovernance.
//   - packages/jurisdiction's per-deployment scoping shape is the
//     reference Profile follows, not a dependency -- this package does
//     not import packages/jurisdiction.
//
// See doc/compliance.md for the full write-up.
package compliance
