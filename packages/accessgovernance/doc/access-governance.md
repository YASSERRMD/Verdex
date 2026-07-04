# Access governance & least privilege (Phase 080)

This is the last of the twenty phases in this run, and this package is
its capstone: it draws together the access-control threads that have
run through the codebase since Phase 006 -- the static role/permission
model in `packages/identity`, per-case lifecycle scoping added in
Phase 063 (`packages/caselifecycle`), the key-specific break-glass
procedure added in Phase 076 (`packages/keymanagement`), and the
hash-chained, queryable audit trail added in Phase 077
(`packages/auditlog`) -- into a single, fine-grained, auditable
governance layer: `packages/accessgovernance`.

## Goal

Move beyond "does this role have this permission" to "should this
specific actor be allowed to perform this specific action, on this
specific resource, right now" -- while keeping every decision,
elevation, and review recorded in the same durable audit trail the
rest of the platform already relies on.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` -- a static role-to-permission matrix | `Policy`/`Request`/`Engine.Evaluate`: attribute-based rules layered *on top of* the matrix, not a replacement for it |
| `packages/caselifecycle` (Phase 063) | `Case`, tenant-scoped access implicit in tenancy alone | `CaseGrant`: an explicit, time-bound, per-case override -- widening access to a specific non-tenant-wide actor, or narrowing it below the role default |
| `packages/keymanagement` (Phase 076) | `BreakGlassGrant`/`GrantBreakGlass`/`UseBreakGlass`: time-bound, justification-required emergency access to a *key* | `Grant`/`Engine.Elevate`: the same shape -- mandatory expiry, mandatory justification, audited regardless of outcome -- generalized to *any* governed `Action`, not just keys |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store; `Kind` taxonomy; CSV/JSON export | `AuditSink` projects every `Evaluate`/`Elevate`/`Attest` call into that same store; `PrivilegedActivity` queries it back for elevation-specific events; no parallel log |
| `packages/signoff` | Explicit human acknowledgement pattern: a reviewer records a decision with required notes | `Review`/`Engine.Attest`: the same "explicit decision, required notes" shape, applied to grants/elevations instead of case sign-off |

### The break-glass overlap, explicitly

`packages/keymanagement`'s break-glass mechanism (`BreakGlassGrant`,
`GrantBreakGlass`, `UseBreakGlass`) is **untouched** by this phase and
remains the correct, sole entry point for emergency access to key
material specifically. This package does not fork a second,
competing break-glass implementation for keys.

What this package does is **generalize the underlying idea** --
time-bound, justification-required, heavily-audited emergency/JIT
access -- from "one mechanism hardcoded to keys" to "any `Action` this
package's `Engine` governs". `accessgovernance.Grant` mirrors
`keymanagement.BreakGlassGrant`'s shape field-for-field (mandatory
`ExpiresAt`, mandatory non-blank `Justification`, `GrantedAt`,
`IsExpired(now)`), and `Engine.Elevate` mirrors `GrantBreakGlass`'s
control flow (authorize, validate justification, default TTL, audit
regardless of outcome). This is a thin, reused pattern, not a
duplicated subsystem -- a caller that specifically needs emergency key
access still goes through `packages/keymanagement`; a caller that
needs temporary elevated access to some *other* governed action (e.g.
`case:delete`, `settings:manage`) uses this package's `Elevate`
instead of inventing a bespoke ad hoc mechanism.

## The decision engine (task 1): `Policy` and `Engine.Evaluate`

`identity.HasPermission(role, perm)` answers a static question: does
this role, in the abstract, ever hold this permission. It cannot
express "only for this case", "only during business hours", or "only
until this grant expires" -- there is no attribute dimension in a bare
role/permission check.

`Policy` adds that dimension. A `Policy` is an ordered list of
`PolicyRule`s; each rule matches on:

- `Actions` -- which `Action`(s) it applies to,
- `Roles` -- which of the actor's roles it applies to,
- `Jurisdictions` -- which `JurisdictionID`(s) it applies to,
- `RequireCaseScope` -- whether the request must be case-scoped,
- `TimeWindow` -- an optional UTC hour-of-day window.

`Engine.Evaluate(ctx, Request) (Decision, error)` is the real decision
function, in this precedence order:

1. An unexpired `CaseGrant` that explicitly **denies** the requested
   permission for this actor+case wins over everything else.
2. An unexpired `Grant` (JIT elevation) for this actor+action(+case)
   allows.
3. An unexpired `CaseGrant` that **grants** the requested permission
   allows.
4. The first matching `PolicyRule` (in registration order) resolves
   the decision.
5. If nothing matches: **deny** (fail closed).

`Evaluate` still requires the caller to be an authenticated
`identity.User` belonging to the request's own tenant -- it adds a
finer-grained layer *after* the ordinary authentication/tenant checks,
it does not replace them.

## Per-case access grants (task 2): `CaseGrant`

A `CaseGrant` ties `CaseID` + `GranteeUserID` + `Permissions` +
mandatory `ExpiresAt` together, optionally as a `Deny` (restriction)
rather than an additive grant. Two uses:

- **Widening**: share one case with an external reviewer who holds no
  ordinary role in the tenant at all (`GranteeUserID` need not map to
  any `identity.Role`).
- **Narrowing**: restrict a role that would otherwise have access
  (e.g. an advocate) on one sensitive case, without touching the
  role's tenant-wide permissions.

`CaseGrant` references `packages/caselifecycle.Case` by `CaseID` only
-- it does not duplicate `Case` or its lifecycle state machine.
`Engine.RevokeCaseAccess` revokes a grant immediately, before its
natural expiry (used by `Attest`'s revoke path -- see below).

## Time-bound, just-in-time access (task 3): `Grant` and `Engine.Elevate`

`Engine.Elevate(ctx, tenantID, granteeUserID, action, caseID,
justification, ttl) (Grant, error)` produces a `Grant`: a temporary
elevation to perform one `Action`, with a **mandatory** expiry
(`DefaultElevationTTL` of one hour if `ttl <= 0`) and a **mandatory**
non-blank `Justification` (`ErrJustificationRequired` otherwise).

Expiry is checked purely at evaluation time -- `Grant.IsExpired(now)`
-- with no background job required, exactly as
`keymanagement.BreakGlassGrant` already does. `Evaluate` rejects an
expired (or explicitly revoked) `Grant` the same way it rejects an
expired `CaseGrant`.

## Access review workflow (task 4): `Review` and `Engine.Attest`

`Engine.ScheduleReview` flags a `CaseGrant` or `Grant` (identified by
`GrantKind` + subject ID) as due for review by a given time.
`Engine.ListDueReviews` lists every pending review whose `DueAt` has
arrived. `Engine.Attest(ctx, tenantID, reviewID, decision, notes)`
records exactly one of:

- `AttestationApprove` -- the grant remains active until its own
  natural expiry;
- `AttestationRevoke` -- the underlying `CaseGrant`/`Grant` is revoked
  immediately.

A `Review` can be attested exactly once (`ErrReviewAlreadyDecided` on
a second attempt).

## Segregation of duties (task 5): `ConflictRule` and `CheckConflict`

Before recording any attestation, `Attest` calls `CheckConflict` with
the review's recorded `RequestedBy` against the attempting actor. Two
built-in rules (`DefaultConflictRules`):

- `RuleRequesterCannotApprove` -- the user a grant was issued to/
  requested by can never also be the one attesting the review over
  it. This is checked unconditionally whenever the acting user equals
  the requester.
- `RuleApproverNotSoleAuthor` -- mirrors `packages/signoff`'s domain:
  a case's sole author cannot be its own approving reviewer. Callers
  supply `ConflictCheck.SoleAuthor` from their own case-authorship
  lookup; this package does not itself know how to determine case
  authorship.

A violation returns `ErrSegregationOfDuties` and the rejection itself
is still recorded via `AuditSink`.

## Privileged-access monitoring (task 6): composing with `auditlog`

Every `Evaluate`, `Elevate`, and `Attest` call is recorded through
`AuditSink` into the exact same `packages/auditlog.Store` the rest of
the platform already writes to and queries -- there is no second,
parallel audit table. The three action verb-phrases this package
appends are `access_governance.evaluate`, `access_governance.elevate`,
and `access_governance.attest`.

`AuditSink.PrivilegedActivity(ctx, tenantID, filter)` surfaces
elevation/break-glass-style access specifically by querying
`auditlog.Store.Query` filtered to `access_governance.elevate` --
itself still gated on `identity.PermAuditRead` and tenant match,
because it is a thin wrapper around `auditlog.Store.Query`, not a
bypass of it.

## Access certification reports (task 7): `Engine.Certify`

`Engine.Certify(ctx, tenantID, period) (Report, error)` aggregates
every `CaseGrant`, `Grant`, and `Review` whose relevant timestamp falls
within `period` into one structured `Report`. `ExportReport(report,
format)` renders it as `ExportFormatJSON` (the `Report` marshaled
directly) or `ExportFormatCSV` (one row per grant/elevation/review,
with a `section` column identifying which), mirroring
`packages/auditlog.Export` and `packages/dataresidency`'s export
conventions exactly.

## Policy testing harness (task 8): `TestPolicy`

`TestPolicy(ctx, policy, scenarios) (Results, error)` dry-runs a
candidate `Policy` -- typically authored with `Active: false` -- against
a set of `Scenario`s, each pairing a `Request` with the `WantEffect`
the policy author expects. It evaluates the exact same
`Policy.evaluate` logic `Engine.Evaluate` uses (forced to `Active` on
an internal copy, so the harness works before a policy is ever
persisted or activated), and reports a pass/fail per scenario plus
aggregate totals -- a real evaluation, not a stub that trivially agrees
with every expectation.

## Storage

Four new tables, one migration pair each style
(`packages/persistence/migrations/000022_create_accessgovernance.up.sql`
/ `000023_enable_rls_accessgovernance.up.sql`, continuing directly
after `auditlog`'s `000021`): `access_policies`, `access_case_grants`,
`access_elevation_grants`, `access_reviews`. Each follows the same
`Repository` / `PostgresXRepository` / `TenantScopedXRepository`
three-layer pattern established by `packages/keymanagement` and
`packages/auditlog`, with row-level security enforcing tenant
isolation at the database layer in addition to each repository's own
application-level tenant-match guard.

## What is explicitly reused, not duplicated

- `identity.Role` / `identity.Permission` / `identity.HasPermission`
  remain the coarse RBAC gate every `Engine` method still calls
  through `authorizeManage`/`authorizeActor` before doing anything
  finer-grained.
- `packages/caselifecycle.Case` is referenced by `CaseID` only.
- `packages/keymanagement`'s break-glass mechanism is untouched and
  remains the key-specific entry point (see above).
- `packages/auditlog.Store` is the only durable event sink this
  package writes to.
- `packages/signoff`'s acknowledgement pattern is the reference this
  package's `Review`/`Attest` workflow follows, not a dependency.
