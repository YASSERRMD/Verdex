# Compliance mapping (Phase 082)

This phase draws together five earlier threads -- the durable,
hash-chained audit trail added in Phase 077 (`packages/auditlog`), the
static role/permission model added in Phase 006 (`packages/identity`),
the non-binding-analysis guardrail added in Phase 057
(`packages/guardrail`), the data-residency model added around Phase
078 (`packages/dataresidency`), and the data-subject-rights layer
added in Phase 081 (`packages/privacy`) -- into a single,
tenant-scoped, auditable compliance-mapping layer: `packages/compliance`.

## Goal

Map this platform's controls to the legal/regulatory frameworks a
deployment must answer to: catalogue what a control requires, what
platform feature satisfies it, what evidence proves it is actually
satisfied for a given tenant, which controls apply to which
deployment, and where the gaps are.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store; `Kind` taxonomy | `AuditSink` projects every control registration, evidence addition, and profile change into that same store; no parallel log |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewCompliance`/`PermManageCompliance`: the two fine-grained permissions this package's `Engine` gates every operation on |
| `packages/guardrail` (Phase 057) | `RequireDisclaimer`/`CheckText` -- the only non-binding-disclaimer enforcement in this codebase | The `JRH-03` seed control names `guardrail.RequireDisclaimer`/`CheckText` by `MappedTo` tag as what satisfies it, without importing the package for behavior |
| `packages/dataresidency` / `packages/airgapped` (~Phase 078) | Cross-border transfer / residency enforcement | The `UAE-DP-05` seed control names them by `MappedTo` tag |
| `packages/privacy` (Phase 081) | `SubjectAccessRequest`, `ErasureRequest`, `ConsentRecord`, `RetentionPolicy`/`EnforceRetention` -- data-subject-rights and retention machinery | The `UAE-DP-*`/`INTL-DP-*` seed controls name these by `MappedTo` tag as what satisfies each mapped requirement category |
| `packages/accessgovernance` (Phase 080, by reference only) | `Certify`/`Report` aggregation shape | `Dashboard`/`BuildDashboard` follows the identical counts-by-dimension shape, without importing `accessgovernance` |
| `packages/jurisdiction` (by reference only) | Per-deployment scoping shape | `Profile`/`ApplicableControls` follows the identical "not every deployment needs everything" shape, without importing `jurisdiction` |

## Control catalogue (task 1)

`Control` (types.go) is a catalogue row: a `Code` (a short, stable,
human-referenceable identifier like `"UAE-DP-01"`, distinct from `ID`,
the `uuid.UUID` primary key), a `Title`/`Description`, a `Framework`
(an open string type, not a closed Go enum -- a deployment's
applicable frameworks vary by jurisdiction and customer, and a closed
enum would force a code change every time a tenant needed one more
named framework), a `ControlCategory`, and a `MappedTo` list of string
tags naming, by convention, the platform features this control
conceptually maps to (e.g. `"packages/privacy.SAR"`,
`"packages/auditlog"`) -- reference only, no import of the tagged
packages implied, exactly mirroring
`packages/privacy.DataInventoryEntry.SourceTag`'s convention.

Unlike every tenant-scoped type elsewhere in this package, `Control`
carries no `TenantID`: a catalogued control is shared reference data
describing a requirement that may apply across many tenants, not a
per-tenant record. `ControlRepository`'s implementations
(`InMemoryControlRepository`, `PostgresControlRepository`) reflect
this directly -- there is no tenant argument anywhere on the
interface.

## UAE data protection, judicial records, and international frameworks (tasks 2-4)

`SeedControls` (seed.go) returns three concrete starter sets. Every
`Description` is deliberately framed as a *mapped requirement
category* this platform addresses, never a verbatim statutory
citation this package cannot verify:

- **`UAE-DP-01` .. `UAE-DP-07`** (task 2): PDPL-style UAE
  data-protection controls -- lawful basis, data-subject access rights,
  right to erasure with provenance preservation, consent/withdrawal
  tracking, cross-border transfer restriction, breach detection and
  notification, and retention-limit enforcement. Each `MappedTo`s the
  `packages/privacy` type or `packages/dataresidency`/`packages/auditlog`
  mechanism that actually satisfies it.
- **`JRH-01` .. `JRH-05`** (task 4): judicial-records-handling
  controls -- court record retention schedule, chain-of-custody
  preservation, non-binding disclaimer enforcement on reasoning output
  (tied to `packages/guardrail.RequireDisclaimer`/`CheckText`), sign-off
  and audit trail for case disposition, and role/grant-gated case
  access.
- **`INTL-DP-01` .. `INTL-DP-04`** (task 3): international frameworks
  mapped in as an *applicable/reference overlay* -- the same
  requirement categories as the UAE set (lawful basis, data-subject
  rights, breach notification, data minimization), commonly found
  across international data-protection regimes, layered on top of the
  identical underlying `packages/privacy`/`packages/pii` mechanisms
  rather than a second implementation. A deployment opts this overlay
  in via `Profile.Frameworks` only when its customer base or regulatory
  exposure calls for it -- it is not assumed by default the way the
  UAE and judicial-records sets are.

`Framework` stays open specifically so a future deployment's
compliance officer can catalogue a fourth, fifth, or Nth named
framework via `RegisterControl` without this package needing a new
phase to add a Go constant for it.

## Control evidence (task 5)

`ControlEvidence` (evidence.go) is a tenant-scoped record of what
proves a `Control` is satisfied: an `EvidenceKind`
(`audit_query`/`test_name`/`document`/`configuration`), a `Reference`
string (an `auditlog.Filter` description, a Go test name, a doc
link/path, or a configuration identifier -- reference only, this
package never dereferences it), a `CollectedBy` actor, and
`CollectedAt`/`CreatedAt`/`UpdatedAt` timestamps. `Engine.RecordEvidence`
requires the referenced `ControlID` to already resolve to a catalogued
`Control` before accepting the evidence.

## Gap analysis (task 6)

`EvaluateControl`/`RunGapAnalysis` (gap.go) is real evaluation logic,
not a stub that always reports satisfied:

- `EvaluateControl(control, evidence, now)` counts how many *distinct*
  `EvidenceKind` values are represented among the `ControlEvidence`
  records matching `control.ID` and collected at or before `now`
  (future-dated/clock-skewed evidence is excluded, so it can never
  inflate a status). Two or more distinct kinds resolves to
  `StatusSatisfied`; one or more matching records with fewer than two
  distinct kinds resolves to `StatusPartiallyMet`; zero matching
  records resolves to `StatusGap`.
- The two-distinct-kinds bar for `StatusSatisfied` is deliberate: a
  single test name alone proves the code path exists, not that it is
  actually exercised operationally, so this package does not call a
  control fully satisfied on one piece of evidence alone.
- `Engine.RunGapAnalysis(ctx, tenantID)` resolves the tenant's `Profile`
  (falling back to "every catalogued control applies" if none has been
  set -- the permissive default), filters the catalogue through
  `ApplicableControls`, and evaluates each applicable control against
  the tenant's full `ControlEvidence` set, returning a
  `GapAnalysisReport`.

## Per-deployment compliance profile (task 7)

`Profile` (profile.go) is a per-tenant selection of which `Framework`s
-- and, optionally, which specific `Control`s beyond a selected
framework's full set -- apply. Not every deployment needs every
framework: an air-gapped, UAE-only deployment has no reason to carry
`FrameworkInternationalDataProtection` obligations, while a tenant
serving customers under an international data-protection regime opts
that overlay in explicitly via `Frameworks`. `ExcludedControlIDs`
additionally lets a deployment exclude specific controls even within a
selected framework (e.g. a fully air-gapped installation excluding
`CategoryCrossBorderTransfer` controls despite otherwise following
`FrameworkUAEDataProtection`). `ApplicableControls(catalogue, profile)`
is the single function `RunGapAnalysis` and the dashboard both call to
scope a report to what a specific deployment actually needs to
satisfy.

## Compliance dashboard (task 8)

`Dashboard`/`BuildDashboard` (dashboard.go) aggregates a
`GapAnalysisReport` and a tenant's `ControlEvidence` into counts by
`Framework` and `Status`, plus a trailing-30-day evidence-collection
count giving a viewer a sense of velocity, not just a static snapshot
-- mirroring `packages/accessgovernance.Certify`/`Report`'s aggregation
shape. This is a report/query type, not a UI; `Engine.BuildDashboardReport`
is the permission-gated engine-level entry point composing
`RunGapAnalysis` and `ListAllEvidence`.

## Access control

Two new `identity.Permission` constants gate every `Engine` operation,
added following `permission.go`'s exact
`PermViewPrivacy`/`PermManagePrivacy` precedent from Phase 081:

- `compliance:view` (`identity.PermViewCompliance`): read-only access
  to the control catalogue, a tenant's compliance profile, collected
  evidence, gap-analysis reports, and the compliance dashboard.
- `compliance:manage` (`identity.PermManageCompliance`): register or
  update catalogued controls, record control evidence, and set a
  tenant's compliance profile.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Storage

Two new migration pairs, continuing directly after
`000025_enable_rls_privacy`:

- `packages/persistence/migrations/000026_create_compliance.up.sql` /
  `.down.sql` create three tables: `compliance_controls` (shared
  catalogue reference data -- no `tenant_id` column, deliberately),
  `compliance_control_evidence`, and `compliance_profiles` (both
  tenant-scoped).
- `packages/persistence/migrations/000027_enable_rls_compliance.up.sql`
  / `.down.sql` enable and force row-level security with the standard
  `tenant_isolation` policy on the two tenant-scoped tables.
  `compliance_controls` carries no RLS policy, matching its lack of a
  `tenant_id` column.

Each tenant-scoped table follows the same `Repository` /
`PostgresXRepository` / `TenantScopedXRepository` three-layer pattern
established by `packages/privacy` and `packages/accessgovernance`, with
Row-Level Security enforcing tenant isolation at the database layer in
addition to each repository's own application-level
`requireMatchingTenant` guard. `compliance_controls`'s
`TenantScopedControlRepository` is a thin, unscoped pass-through --
there is no tenant scope to apply, since the underlying table has none.

## What is explicitly reused, not duplicated

- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink`.
- `identity.Role`/`identity.Permission`/`identity.HasPermission` remain
  the coarse RBAC gate every `Engine` method calls through before doing
  anything compliance-specific.
- `packages/guardrail.RequireDisclaimer`/`CheckText` remain the only
  non-binding-disclaimer enforcement mechanism in this codebase; this
  package references them by `MappedTo` tag only.
- `packages/privacy`'s `SubjectAccessRequest`, `ErasureRequest`,
  `ConsentRecord`, and `RetentionPolicy`/`EnforceRetention` remain the
  only data-subject-rights and retention-enforcement machinery; this
  package references them by `MappedTo` tag only.
- `packages/dataresidency`/`packages/airgapped` remain the only
  cross-border-transfer/residency enforcement in this codebase.
- `packages/accessgovernance`'s `Certify`/`Report` aggregation shape
  and `packages/jurisdiction`'s per-deployment scoping shape are the
  references `Dashboard`/`Profile` follow, not dependencies -- this
  package does not import either.
