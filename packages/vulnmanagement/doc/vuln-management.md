# Vulnerability & dependency management (Phase 084)

This phase draws together two earlier threads -- the durable,
hash-chained audit trail added in Phase 077 (`packages/auditlog`) and
the static role/permission model added in Phase 006
(`packages/identity`) -- into a single, tenant-scoped, auditable
vulnerability-management layer: `packages/vulnmanagement`.

## Goal

Continuous detection of vulnerable components: dependencies (SCA),
this platform's own source code (SAST), container images, and
non-compliant open-source licenses, funneled into one Finding record
that a human triages through a guarded remediation state machine,
tracked against a severity-based SLA, and rolled up into a reporting
dashboard.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store; `Kind` taxonomy | `AuditSink` projects every finding-record and triage-decision call into that same store; no parallel log |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewVulnmanagement`/`PermManageVulnmanagement`: the two fine-grained permissions this package's `Engine` gates every operation on |
| `packages/caselifecycle` (by reference only) | `allowedTransitions` map + `CanTransition` guard shape | `Finding.Status`'s guarded state machine follows the identical map-of-slices shape, without importing `caselifecycle` |
| `packages/threatmodel` (by reference only) | `allowedMitigationTransitions` + terminal-state shape | Same shape again for `Finding.Status`, without importing `threatmodel` |
| `packages/accessgovernance` / `packages/signoff` (by reference only) | `Attest` / `AcknowledgementConfirmation` explicit-decision pattern | `Engine.Triage` requires a non-blank `Notes` and a recorded `Actor`, without importing either package |
| `packages/compliance` / `packages/accessgovernance` (by reference only) | `Dashboard`/`BuildDashboard` and `Certify`/`Report` aggregation shape | `Report`/`BuildReport` follows the identical counts-by-dimension shape, without importing either package |

## Finding: the unit of detection (tasks 1-3)

`Finding` (types.go) is what every scanner category funnels into:

- `Source` (`ScannerSource`): `sca`, `sast`, `container`, or `license`
  -- a closed enum, since the set of scanner categories this platform
  integrates with is a deliberate, curated list, not an open taxonomy.
- `Package`/`Version`: the affected dependency (or source file, for a
  SAST finding with no package version).
- `Severity`: `low`/`medium`/`high`/`critical`.
- `AdvisoryID`: the CVE/GHSA/advisory identifier, or a scanner-internal
  rule ID when no CVE has been assigned (a SAST finding).
- `Status`: the remediation state machine (see below).
- `DiscoveredAt`: when the scanner first reported it -- the anchor
  `SLADeadline` computes a remediation deadline from.

`Engine.RecordFinding` is the single, permission-gated, tenant-scoped,
audited ingestion point every scanner's output is written through. A
freshly recorded `Finding` always starts at `StatusOpen` regardless of
what the caller supplies -- a newly-scanned finding has not yet been
triaged by definition.

### Status state machine

```
Open -> Triaged -> Remediating -> Resolved
  |        |            |
  v        v            v
FalsePositive/AcceptedRisk (both terminal)
```

`CanTransition(from, to)` (types.go) is the single authoritative source
of truth, mirroring `packages/caselifecycle`'s `allowedTransitions` map
+ `CanTransition` guard shape and `packages/threatmodel`'s
`allowedMitigationTransitions` shape by reference -- this package
imports neither. Every terminal state (`Resolved`, `AcceptedRisk`,
`FalsePositive`) maps to an empty transition set: there is no
sanctioned "reopen a resolved finding" path in this phase. A
rediscovered vulnerability is a new scanner `Finding`, not a reopened
old one.

## Triage workflow (task 5)

`Engine.Triage(ctx, tenantID, findingID, decision, notes, actor)`
records a `TriageDecision` -- who decided (`Actor`), what they decided
(`ToStatus`), and why (`Notes`, required non-blank) -- and applies the
resulting guarded `Status` transition to the `Finding` itself, mirroring
`packages/accessgovernance`'s `Attest` and `packages/signoff`'s
`AcknowledgementConfirmation`/explicit-acknowledgement pattern by
reference: a finding's disposition always carries an accountability
trail, never a bare status flip. An illegal transition attempt (e.g.
skipping `Open` straight to `Remediating`, or moving out of a terminal
state) is rejected with `ErrIllegalStatusTransition` before any state
changes, and is still recorded via `AuditSink` as a denied attempt --
the audit trail shows attempted-and-rejected triage just as clearly as
successful triage.

## SLA tracking (task 6)

`remediationSLA` (sla.go) maps `Severity` to a real remediation window:

| Severity | Deadline |
|---|---|
| Critical | 7 days |
| High | 30 days |
| Medium | 90 days |
| Low | 180 days |

`SLADeadline(finding)` computes the absolute deadline from
`DiscoveredAt + RemediationDeadlineFor(Severity)`. `IsSLABreached(finding,
now)` is real, testable-with-injected-clock time logic: a `Finding`
already in a terminal `Status` is never considered breached (the SLA
clock only matters for outstanding work), and a `Severity` with no
configured SLA is never considered breached (no SLA means no deadline
to miss, not an implicit immediate breach). `FindingsPastSLA`/
`Engine.ListSLABreaches` are the batch query and permission-gated
engine-level entry point, respectively.

## License compliance (task 7)

`LicenseCheck`/`EvaluateLicense` (license.go) classify a dependency's
declared SPDX license identifier against a seeded allow-list (`MIT`,
`Apache-2.0`, `BSD-2-Clause`, `BSD-3-Clause`, `ISC`, `MPL-2.0`,
`Unlicense`, `0BSD`) and deny-list (`GPL-2.0`, `GPL-3.0`, `AGPL-3.0`,
`LGPL-2.1`, `LGPL-3.0`, `SSPL-1.0`, `CC-BY-NC-4.0`), rather than
requiring a real SPDX database. Matching is deliberately
case-sensitive to canonical SPDX casing (`"MIT"`, not `"mit"`), so a
scanner emitting non-canonical casing does not silently slip past the
allow-list. Any blank or unrecognized license classifies as
`LicenseNeedsReview` -- an unrecognized license is a compliance
question for a human, never a silent pass. `EvaluateLicenses` supports
batch sweeps over a dependency map; `NeedsReview` filters down to the
actionable (denied or needs-review) subset.

## Reporting (task 8)

`BuildReport`/`Report` (report.go) aggregates a tenant's `Finding` set
into counts by `Severity`/`Status`/`ScannerSource` plus the current
SLA-breach list, mirroring `packages/compliance`'s `Dashboard` and
`packages/accessgovernance`'s `Certify`/`Report` shape -- a real report
type + generator, not a UI. `Report.OpenCount()` sums the non-terminal
statuses (`Open` + `Triaged` + `Remediating`) into the headline
"how much outstanding work" figure. `Report.SLABreachesBySeverityDesc()`
sorts the breach list by descending severity, so the most serious
overdue finding surfaces first. `Engine.BuildDashboard` is the
permission-gated, tenant-scoped generator.

## CI wiring (tasks 1-3)

`.github/workflows/ci.yml` gains three jobs:

- **`sca-scan`** (`govulncheck`): real, call-graph-aware Go
  vulnerability scanning across every `go.mod` in the repository.
  `govulncheck` cross-references each module's actual reachable code
  paths against the Go vulnerability database, so it only flags
  dependencies (or standard-library APIs) whose vulnerable code path
  this repository's own code actually calls -- not every CVE anywhere
  in the dependency tree regardless of reachability. Verified locally
  with a full-repo run before writing this job: it currently finds
  real, pervasive standard-library findings (`crypto/tls`,
  `crypto/x509`, `net/url` fixes shipped in Go 1.25.6-1.25.9) across
  nearly every module, since the repository's pinned `go-version:
  '1.25'` currently resolves to a patch release older than those
  fixes. Bumping the pinned toolchain repo-wide is a real fix but is
  out of scope for this phase, so this job runs with
  `continue-on-error: true` -- its real signal stays visible in the PR
  checks list without blocking every unrelated merge on a pre-existing
  condition this phase did not introduce.
- **`sast-scan`** (`gosec`, standalone): `.golangci.yml` already
  enables `gosec` as a golangci-lint linter gating `lint-go`, but
  golangci-lint's `gosec` integration understands this repository's
  `//nolint:gosec` suppression comments, while the standalone `gosec`
  binary run in this job only recognizes its own native `//#nosec`
  comments. Running it bare therefore re-surfaces a handful of
  already-reviewed findings (see e.g.
  `packages/adapters/*/adapter.go`'s `//nolint:gosec` G104
  suppressions) that are not real regressions. This job also runs with
  `continue-on-error: true` so it stays informational -- a security
  reviewer can read its log for anything new -- without blocking
  merges on pre-existing, already-accepted findings.
- **`container-scan`** (placeholder): this repository ships no
  Dockerfile today, verified with `find . -iname Dockerfile*`
  immediately before writing this job (see
  `packages/threatmodel/doc/Dockerfile.hardened`, Phase 083's
  reference hardened template for when one is added). The job is a
  documented TODO rather than a scan of a container image that does
  not exist, naming Trivy/Grype as the intended tool once a real image
  exists to scan. Unlike the two jobs above, this one is a real gate
  dependency (see the `gate` job's own comment) since it is a
  no-op that always succeeds.

`sca-scan` and `sast-scan` are deliberately not wired into the `gate`
job's pass/fail decision while they carry pre-existing,
not-yet-remediated findings unrelated to this phase's own changes --
see the `gate` job's comment in `ci.yml` for the exact reasoning. Both
remain visible, real jobs in the PR checks list; only their influence
on the hard merge gate is deferred.

## Automated dependency updates (task 4)

`.github/dependabot.yml` configures three ecosystems, each opening a PR
per outdated dependency that then runs through the same CI workflow
(including the scanning jobs above) as any other change:

- `gomod`, using the `directories` glob (`/packages/*`, `/services/*`)
  rather than hand-listing 70+ individual module paths.
- `npm`, at the workspace root (`package.json`'s `"workspaces"` field
  already covers `apps/*`, `packages/*`, `services/*/frontend`).
- `github-actions`, since a pinned Action version can itself carry a
  vulnerability -- part of the same continuous-detection surface as
  the Go/npm ecosystems, not a separate concern.

## Access control

Two new `identity.Permission` constants gate every `Engine` operation,
added following `permission.go`'s exact
`PermViewThreatmodel`/`PermManageThreatmodel` precedent from Phase 083:

- `vulnmanagement:view` (`identity.PermViewVulnmanagement`): read-only
  access to findings, triage decision history, SLA-breach reports, and
  the vulnerability-management dashboard.
- `vulnmanagement:manage` (`identity.PermManageVulnmanagement`):
  record findings, triage them, and transition a finding's remediation
  status.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Storage

Two new migrations, continuing directly after
`000027_enable_rls_compliance`:

- `packages/persistence/migrations/000028_create_vulnmanagement.up.sql`
  / `.down.sql` create `vulnmanagement_findings` and
  `vulnmanagement_triage_decisions`. Unlike `packages/compliance`'s
  shared `compliance_controls` catalogue, both tables here are
  genuinely per-tenant operational data -- a finding belongs to one
  tenant's deployment -- so both carry `tenant_id`.
- `packages/persistence/migrations/000029_enable_rls_vulnmanagement.up.sql`
  / `.down.sql` enable and force row-level security with the standard
  `tenant_isolation` policy on both tables.

Each table follows the same `Repository` / `PostgresXRepository` /
`TenantScopedXRepository` three-layer pattern established by
`packages/compliance` and `packages/privacy`, with Row-Level Security
enforcing tenant isolation at the database layer in addition to each
repository's own application-level `requireMatchingTenant` guard.

## What is explicitly reused, not duplicated

- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink` -- exactly the composition
  pattern `packages/compliance`'s and `packages/threatmodel`'s own
  `AuditSink` established.
- `identity.Role` / `identity.Permission` / `identity.HasPermission`
  (Phase 006) remain the coarse RBAC gate every `Engine` method calls
  through `authorizeManage`/`authorizeView` before doing anything
  vulnerability-management-specific.
- `packages/caselifecycle`'s and `packages/threatmodel`'s
  guarded-transition-map shapes are the reference `Finding.Status`
  follows, not a dependency -- this package imports neither.
- `packages/accessgovernance`'s `Attest` and `packages/signoff`'s
  explicit-acknowledgement pattern are the reference `Engine.Triage`
  follows, not a dependency -- this package imports neither.
- `packages/compliance`'s `Dashboard`/`BuildDashboard` and
  `packages/accessgovernance`'s `Certify`/`Report` aggregation shape
  are the reference `Report`/`BuildReport` follows, not a dependency --
  this package does not import either.
- `.golangci.yml`'s `gosec` linter (already gating `lint-go`) remains
  the primary in-repo SAST signal for day-to-day development; the
  standalone `sast-scan` CI job is a second, broader-but-informational
  pass, not a replacement.
