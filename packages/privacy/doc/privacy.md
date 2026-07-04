# Privacy & data subject controls (Phase 081)

This phase draws together four earlier threads -- the PII detection
and redaction primitives added in Phase 025
(`packages/pii`), the hash + chain-of-custody model added in Phase 020
(`packages/provenance`), the durable, hash-chained audit trail added
in Phase 077 (`packages/auditlog`), and the static role/permission
model added in Phase 006 (`packages/identity`) -- into a single,
tenant-scoped, auditable data-subject-rights layer:
`packages/privacy`.

## Goal

Honor data-subject rights (access, erasure, consent) and enforce data
minimization (retention limits, anonymization for analytics) across
every tenant's stored personal data, while never weakening the
provenance guarantees the rest of the platform depends on: erasing a
data subject's personal content must never erase, mutate, or
invalidate the chain-of-custody record that content was tracked under.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/pii` (Phase 025) | `Detector`/`RuleBasedDetector`/`ClassifyMatches`/`Redactor`/`RedactionMode` -- deterministic PII detection and redaction over free text | `AnonymizeForAnalytics`: an aggregated/pseudonymized projection of a record's free-text fields for analytics use, calling straight through to `pii`'s existing pipeline for every span-level decision |
| `packages/provenance` (Phase 020) | `ProvenanceRecord`'s `ContentHash`/`ChainHash`, an append-only, tamper-evident chain-of-custody model | `ErasureRequest`/`ErasureResult`: an explicit model of "personal content scrubbed, chain-of-custody hash intact and queryable", referencing a `ProvenanceRecord` by ID + hash only -- never importing `provenance`'s store |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store; `Kind` taxonomy | `AuditSink` projects every SAR transition, erasure execution, consent change, and inventory registration into that same store; no parallel log |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewPrivacy`/`PermManagePrivacy`: the two fine-grained permissions this package's `Engine` gates every operation on |
| `packages/caselifecycle` (Phase 063, by reference only) | Guarded state-transition shape: an `allowedTransitions` map plus a `CanTransition` guard function | `SARStatus`/`allowedSARTransitions`/`CanTransitionSAR`/`TransitionSAR` follow the identical shape for subject-access-request status, without importing `caselifecycle` |

## Data inventory and minimization (tasks 1-3)

`DataInventoryEntry` (types.go) is a registry row: a `DataCategory`
(a system-level classification -- `case_party`, `identity`,
`contact`, `identifier`, `financial`, `transcript`, `behavioral`,
`other` -- distinct from `pii.PIICategory`, which classifies a single
detected text span rather than a whole class of stored data), a
`SourceTag` naming where it lives by convention (`"case.parties"`,
`"identity.user"`, `"ingestion.transcript"`, etc. -- reference only, no
package import), a `Sensitivity` tier, a `LegalBasis`, and a
`RetentionPeriod`.

`RetentionPolicy` (retention.go) binds a `DataCategory` to a retention
`Window` and a `DeletionAction` (`ActionHardDelete` or
`ActionAnonymize`). `EnforceRetention(policy, recordCreatedAt, now)` is
the real evaluation function: it reports `ActionRetain` while a record
is within its window, or the policy's prescribed action once the
window has elapsed -- exercised directly by unit tests at, before, and
past the exact boundary.

## Consent and legal-basis tracking (task 6)

`ConsentRecord` (consent.go) captures a subject identifier (a plain
string, since a data subject -- a case party or witness, say -- is not
always a registered `identity.User`), a tenant, a processing
`Purpose`, a `LegalBasis`, `GrantedAt`, and a nilable `WithdrawnAt`.
`HasValidConsent(records, subjectID, purpose, now)` evaluates a
subject's *entire* consent history for that purpose, so:

- a subject with only a withdrawn record (no subsequent re-grant)
  correctly has no valid consent, and
- a subject who withdrew and was later re-granted correctly does.

This is real logic exercised directly by tests, not a lookup that
trusts a single record.

## Subject access requests (task 4)

`SubjectAccessRequest` (sar.go) tracks a data subject's access request
through a guarded `SARStatus` state machine: `received` ->
`in_progress` -> `fulfilled`/`rejected`. `allowedSARTransitions` (a map
of permitted destination statuses per origin status) and
`CanTransitionSAR`/`TransitionSAR` (the guard function) mirror
`packages/caselifecycle`'s `allowedTransitions`/`CanTransition` shape
by reference -- this package does not import `caselifecycle`. An
illegal transition returns `ErrIllegalSARTransition` and leaves the
request unmutated; a move into a terminal status stamps `ResolvedAt`
and records `ResolutionNotes`.

## Right to erasure with provenance preservation (task 5)

This is the phase's hard, non-negotiable constraint, and
`ErasureRequest`/`ErasureResult`/`Engine.ExecuteErasure` (erasure.go)
model it explicitly rather than as an implicit convention:

- `ErasureRequest` may carry a `ProvenanceRecordID` and
  `ProvenanceHash` pointing at the `packages/provenance.ProvenanceRecord`
  describing the content being erased. The two fields are required
  together (`ErrProvenanceHashRequired`) -- a request can never
  reference a provenance record without also carrying the hash that
  must survive.
- `Engine.ExecuteErasure` copies `ProvenanceRecordID`/`ProvenanceHash`
  from the request into the returned `ErasureResult`
  **before** invoking the caller-supplied `ScrubFunc`, and never
  subsequently overwrites them. This is what makes "content scrubbed,
  chain-of-custody hash intact" a property of this method's control
  flow, not a promise that depends on `ScrubFunc` behaving correctly
  or on this package remembering to leave some other record alone.
- `ErasureResult.ProvenancePreserved` is `true` whenever there *was* a
  provenance record to preserve (asserting it survived), and remains
  vacuously `true` when there was none to begin with (nothing to have
  broken).
- This package has **no dependency on `packages/provenance`'s store at
  all** -- it never reads, writes, or even resolves a
  `ProvenanceRecord`; it only threads the ID and hash string a caller
  supplies straight through. The actual chain-of-custody record lives
  entirely in `packages/provenance` and is never touched by this
  package's code, which is a stronger guarantee than "we promise not
  to modify it" would be.
- `TestExecuteErasure_ProvenanceHashSurvives` (erasure_test.go) is the
  test that asserts this directly: it files an `ErasureRequest`
  referencing a simulated provenance record, executes erasure against
  a fake downstream content store, and asserts the returned
  `ErasureResult` -- and the persisted `ErasureRequest` itself --
  still carry the exact original hash and record ID, while the
  personal content is actually gone.

## Anonymization for analytics (task 8)

`AnonymizeForAnalytics(ctx, subjectID, category, records, mode)`
(anonymize.go) produces an `AnonymizedRecord`: one `AnonymizedField`
per input free-text field, with every detected PII span redacted.
Every detection and redaction decision is delegated to
`packages/pii.NewRuleBasedDetector`, `pii.ClassifyMatches`, and
`pii.NewRedactor` -- this package does not reimplement NER, regex
detection, or redaction logic. `mode` defaults to
`pii.ModeIrreversibleRedact` (an analytics projection has no
legitimate need to ever reverse a redaction), but a caller may request
`pii.ModePseudonymize` explicitly.

## Privacy audit (task 7)

`AuditSink` (audit.go) records every `SubjectAccessRequest` status
transition, `ExecuteErasure` call (success or failure, including
whether the provenance record was preserved), `ConsentRecord`
grant/withdrawal, and `DataInventoryEntry` registration into
`packages/auditlog.Store` -- the same durable, hash-chained sink the
rest of the platform already writes to and queries. There is no second
audit table. `PrivacyActivity` is a thin, still-permission-gated
wrapper around `auditlog.Store.Query` for a compliance dashboard's read
side.

## Access control

Two new `identity.Permission` constants gate every `Engine` operation,
added following `permission.go`'s exact `PermViewKeys`/`PermManageKeys`
precedent from Phase 076:

- `privacy:view` (`identity.PermViewPrivacy`): read-only access to the
  data inventory, retention report, and privacy audit trail.
- `privacy:manage` (`identity.PermManagePrivacy`): process
  subject-access, erasure, and consent-change requests.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Storage

Two new migration pairs, continuing directly after
`000023_enable_rls_accessgovernance`:

- `packages/persistence/migrations/000024_create_privacy.up.sql` /
  `.down.sql` create four tenant-scoped tables:
  `privacy_data_inventory`, `privacy_consent_records`,
  `privacy_subject_access_requests`, and `privacy_erasure_requests`.
  `privacy_erasure_requests` carries a
  `privacy_erasure_provenance_hash_required` CHECK constraint
  mirroring `ErasureRequest.Validate`'s `ErrProvenanceHashRequired`
  guard at the database layer -- a row can never reference a
  provenance record without also carrying its hash.
- `packages/persistence/migrations/000025_enable_rls_privacy.up.sql` /
  `.down.sql` enable and force row-level security with the standard
  `tenant_isolation` policy on all four tables.

Each table follows the same `Repository` / `PostgresXRepository` /
`TenantScopedXRepository` three-layer pattern established by
`packages/accessgovernance` and `packages/keymanagement`, with
Row-Level Security enforcing tenant isolation at the database layer in
addition to each repository's own application-level
`requireMatchingTenant` guard.

## What is explicitly reused, not duplicated

- `packages/pii`'s `Detector`/`RuleBasedDetector`/`ClassifyMatches`/
  `Redactor`/`RedactionMode` remain the only PII detection and
  redaction pipeline in this codebase; `AnonymizeForAnalytics` calls
  through them rather than re-detecting or re-redacting.
- `packages/provenance`'s `ProvenanceRecord` hash/chain-of-custody
  model is preserved, never mutated, by erasure. This package has no
  dependency on `packages/provenance`'s store at all.
- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink`.
- `identity.Role`/`identity.Permission`/`identity.HasPermission`
  remain the coarse RBAC gate every `Engine` method calls through
  before doing anything privacy-specific.
- `packages/caselifecycle`'s guarded state-transition shape
  (`allowedTransitions` map + `CanTransition` guard function) is the
  reference `SARStatus`/`CanTransitionSAR`/`TransitionSAR` follow, not
  a dependency -- this package does not import `packages/caselifecycle`.
