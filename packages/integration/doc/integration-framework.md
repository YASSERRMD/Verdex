# External system integration framework (Phase 087)

This phase draws together the adapter/registry pattern added in
Phase 011 (`packages/provider`), the durable, hash-chained audit trail
added in Phase 077 (`packages/auditlog`), the static role/permission
model added in Phase 006 (`packages/identity`), and the by-reference-only
conventions established by `packages/accessgovernance` (Phase 080,
referencing `packages/caselifecycle` by ID only) and `packages/compliance`
(Phase 082, referencing platform features by string tag only) into a
single, tenant-scoped, auditable integration layer: `packages/integration`.

## Goal

Connect to external court case-management systems where permitted --
importing cases inbound and delivering reports (opinions, compliance
exports) outbound -- through one `Connector` adapter interface rather
than a bespoke client per court system, with the same tenant isolation,
permission gating, and audit-trail discipline every other phase in
this platform already applies.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/provider` (Phase 011) | `LLMProvider` interface, `Registry`, `Capability` descriptor, `NoOpProvider` | `Connector` interface, `Registry`, `ConnectorCapability` descriptor, `SandboxConnector` -- the identical adapter/registry/mock-implementation pattern applied to court case-management systems instead of model providers |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store; `Kind` taxonomy | `AuditSink` projects every connector registration, credentials change, import run, delivery run, and reconciliation attempt into that same store; no parallel log |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewIntegration`/`PermManageIntegration`: the two fine-grained permissions this package's `Engine` gates every operation on |
| `packages/caselifecycle` (Phase 063, by field name only) | `Case`'s `ID`/`TenantID`/`JurisdictionID`/`CategoryID`/`Title`/`Reference`/`State`/`Metadata` fields | `InboundCase`/`FieldMapping` reference these field names in doc comments only -- no import |
| `packages/reportexport` (Phase 073, by field name only) | `Report`'s `CaseID`/`TenantID`/`CaseTitle`/`CaseReference`/`JurisdictionKey` fields, and its PDF/DOCX/Markdown render functions | `OutboundReport` carries a caller-rendered `Payload` addressed by external case ID, naming the source `Report` fields by convention only -- no import |
| `packages/keymanagement` (Phase 076, by reference only) | `Provider`/`KeyMetadata`, the only place raw key material is stored | `ConnectorCredentials.SecretRef` names a key/secret by handle only -- this package never stores or transmits the referenced secret's raw bytes |
| `packages/compliance` (Phase 082, shape only) | `Engine`'s authenticate/check-tenant/check-permission/mutate/audit-regardless-of-outcome shape | `Engine` follows the identical shape for connector configs, credentials, field mappings, and import/delivery/reconciliation runs -- no import |

## The Connector adapter interface (task 1)

`Connector` (connector.go) is the contract every concrete external
court case-management system adapter must satisfy:

```go
type Connector interface {
    ID() string
    Capabilities() ConnectorCapability
    ImportCases(ctx context.Context, since time.Time) ([]InboundCase, error)
    DeliverReport(ctx context.Context, report OutboundReport) (DeliveryReceipt, error)
    Ping(ctx context.Context) error
}
```

This mirrors `packages/provider.LLMProvider`'s interface shape
precisely -- `ID`/`Capabilities`/the two substantive calls/a health
check -- just applied to court case-management systems instead of
model providers. `Registry` (registry.go) is the identical
thread-safe `Register`/`Get`/`List`/`Unregister`/`DefaultRegistry`
map `packages/provider.Registry` establishes.

`InboundCase` carries an external system's raw field representation
(`ExternalID`, `ExternalUpdatedAt`, and a `Fields` map keyed by the
external system's own field names) -- deliberately not
`packages/caselifecycle.Case` itself, since the external schema is not
this platform's schema. `OutboundReport` carries a rendered report
`Payload` addressed by `CaseExternalID` and `ReportKind` --
deliberately not `packages/reportexport.Report` itself. Both types
reference their platform counterparts by field name in comments only;
neither package is imported, keeping this package's dependency
footprint thin, exactly as `packages/accessgovernance.CaseGrant`
references `packages/caselifecycle.Case` by `CaseID` only.

`ConnectorConformanceTest` (conformance_test.go) is the
`Connector`-contract counterpart to
`packages/provider.ProviderConformanceTest`: any adapter, including
`SandboxConnector`, can call it from its own test suite to confirm it
satisfies the interface.

## Inbound case import (task 2)

`ImportRun` (importrun.go) is the durable, tenant-scoped outcome
record of one `Connector.ImportCases` attempt: the connector used, the
`since` window, per-run counts (`ImportedCount`/`MappedCount`), and
which external IDs (if any) failed mapping (`FailedExternalIDs`).
`summarizeImportRun` derives a real
`ImportRunStatusSucceeded`/`Partial`/`Failed` from the actual outcome
of every case in the run -- not a status hardcoded to "succeeded".
`Engine.RunImport` resolves the tenant's `ConnectorConfig` to a live
`Connector` via `Registry`, wraps the call in `WithRetry`, invokes an
optional per-case `mapFn` (typically a `FieldMapping.Apply` followed
by a `packages/caselifecycle` case-creation call the caller supplies),
and persists the `ImportRun` regardless of outcome. Like the rest of
this platform's transcribe-and-discard convention for transient
ingestion artifacts, the raw `InboundCase` payloads themselves are
never persisted by this package once an `ImportRun` is recorded --
only the summary.

## Outbound report delivery (task 3)

`DeliveryRun` (deliveryrun.go) is the outbound counterpart: one
`Connector.DeliverReport` attempt's outcome
(`DeliveryRunStatusAccepted`/`Rejected`/`Failed`, derived by
`statusFromReceipt` from the actual `DeliveryReceipt` the connector
returned) plus attempt count and receipt detail. `Engine.RunDelivery`
follows the identical resolve-connector/retry/persist-regardless-of-
outcome shape as `RunImport`.

## Field mapping configuration (task 4)

`FieldMapping`/`FieldRule` (fieldmapping.go) is a tenant's
configurable, ordered set of field-level translations between this
platform's case/party/document field names and an external system's
own schema. `Apply(externalRecord) (MappedFields, error)` is a real
translation function: it enforces `Required` fields (returning
`ErrUnmappedField` when absent), applies `DefaultValue` for optional
ones, and surfaces any external field no rule claimed
(`UnmappedSourceFields`) so a stale mapping becomes visible rather than
silently dropping data. `Reverse(values)` is the exact mirror image,
translating this platform's field values back into the external
system's own field names for outbound delivery metadata. Neither is a
type with no logic -- `TestFieldMappingRoundTrip` exercises
`Apply` -> `Reverse` fidelity directly.

## Integration auth and security (task 5)

`ConnectorCredentials` (credentials.go) describes one of four
authentication shapes (`CredentialKindAPIKey`,
`CredentialKindOAuthClientCredentials`, `CredentialKindMutualTLS`,
`CredentialKindNone`) but never stores raw secret material -- only a
`SecretRef` string naming a handle/reference into `packages/keymanagement`
or `packages/encryption` (or an external secrets manager), resolved by
the caller at call time. `Validate` enforces per-kind structural
requirements (an OAuth credential needs a non-blank `ClientID` and
`TokenURL`; anything but `CredentialKindNone` needs a non-blank
`SecretRef`). `AuthorizeCredentials(ctx, creds, verify, now)` performs
that structural check plus an optional live `VerifyFunc` round-trip
against the external system -- the per-connector auth validation task
5 requires before any `Connector` call is attempted -- and records
`LastVerifiedAt` on success. `Engine.SetCredentials` calls this before
persisting, and `AuditSink.RecordCredentialsSet` never logs `SecretRef`
or any secret material, only the non-secret `Kind`/`ClientID` fields
(`TestEngineSetCredentialsNeverLogsSecretRef` asserts this directly).

## Retry and reconciliation (task 6)

`WithRetry(ctx, policy, shouldRetry, fn)` (retry.go) is a real
retry-with-backoff wrapper around any `Connector` call: bounded
`MaxAttempts`, jittered exponential backoff
(`BaseDelay`/`MaxDelay`/`Jitter`), honouring context cancellation, with
an optional `IsRetryable` veto letting a caller mark certain errors as
non-retryable even with attempts remaining. Every attempt that
exhausts `MaxAttempts` returns a wrapped `ErrRetriesExhausted`.
`Engine.RunImport`/`RunDelivery` both wrap their `Connector` call in
`WithRetry`.

`Reconcile(tenantID, connectorConfigID, kind, expected, observed, ranBy, now)`
(reconcile.go) is a real set-comparison function, not a stub that
always reports clean: it compares an expected target set of external
IDs (what should have been imported or delivered) against what was
actually observed, and returns a `ReconciliationResult` with
`MissingExternalIDs` (expected but never seen -- the missed-records
case task 6 calls out) and `UnexpectedExternalIDs` (seen but not
expected). `HasDrift()` reports whether either list is non-empty.
`Engine.RunReconciliation` persists the result and audits the attempt
regardless of outcome; `TestEngineRunReconciliationDetectsDrift`
exercises this end-to-end against a `SandboxConnector`.

## Integration audit (task 7)

`AuditSink` (audit.go) composes with `packages/auditlog.Store` --
the platform's single durable, hash-chained audit sink -- exactly the
pattern `packages/compliance`'s and `packages/privacy`'s own `AuditSink`
established. Every connector registration
(`RecordConnectorRegister`), credentials change
(`RecordCredentialsSet`), import run (`RecordImportRun`), delivery run
(`RecordDeliveryRun`), and reconciliation attempt
(`RecordReconciliation`) is recorded, regardless of whether the
underlying operation succeeded, failed authorization, or failed at the
external system -- `Engine`'s every mutating method records an event
even on the `ErrForbidden`/`ErrUnauthenticated` path, using
`actorFromCtx`'s `uuid.Nil` fallback so an unauthenticated attempt is
still attributed to a resolvable (if anonymous) actor label. There is
no second audit table anywhere in this package.

## Sandbox/test connector (task 8)

`SandboxConnector` (sandbox.go) is a deterministic, real in-process
`Connector` implementation -- this platform's "no-op/mock provider for
tests" equivalent from Phase 011 (`packages/provider.NoOpProvider`),
applied to court case-management systems rather than model providers.
It supports `SeedCase` (register an `InboundCase` fixture),
`Delivered()` (inspect every accepted `OutboundReport` in call order),
and an `Unavailable` toggle that makes `Ping`/`ImportCases`/
`DeliverReport` all fail with `ErrConnectorUnavailable`, letting a test
exercise the retry and reconciliation paths against a connector that
is briefly down without any real external dependency.

## Access control

Two new `identity.Permission` constants gate every `Engine` operation,
added following `permission.go`'s exact
`PermViewCompliance`/`PermManageCompliance` precedent from Phase 082:

- `integration:view` (`identity.PermViewIntegration`): read-only access
  to connector configurations (minus credential material), import/
  delivery run history, and reconciliation results.
- `integration:manage` (`identity.PermManageIntegration`): register or
  update connector configurations, set connector credentials, trigger
  inbound case imports and outbound report deliveries, and run
  reconciliation.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Storage

One new migration pair, continuing directly after
`000027_enable_rls_compliance`:

- `packages/persistence/migrations/000028_create_integration.up.sql` /
  `.down.sql` create six tables, all tenant-scoped (unlike
  `packages/compliance`'s shared `compliance_controls` catalogue --
  there is no shared reference-data table in this phase):
  `integration_connector_configs`, `integration_connector_credentials`
  (a `secret_ref` handle column only, never raw secret bytes),
  `integration_field_mappings`, and the append-only
  `integration_import_runs` / `integration_delivery_runs` /
  `integration_reconciliation_results` run-history tables.
- `packages/persistence/migrations/000029_enable_rls_integration.up.sql`
  / `.down.sql` enable and force row-level security with the standard
  `tenant_isolation` policy on all six tables.

Each table follows the same `Repository` / `PostgresXRepository` /
`TenantScopedXRepository` three-layer pattern established by
`packages/compliance` and `packages/privacy`, with Row-Level Security
enforcing tenant isolation at the database layer in addition to each
repository's own application-level `requireMatchingTenant` guard.

## What is explicitly reused, not duplicated

- `packages/provider.LLMProvider`'s interface shape and
  `packages/provider.Registry`'s `Register`/`Get`/`List` pattern are
  the reference `Connector`/`Registry` follow, not a dependency --
  this package does not import `packages/provider`.
- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink`.
- `identity.Role`/`identity.Permission`/`identity.HasPermission`
  remain the coarse RBAC gate every `Engine` method calls through
  before doing anything integration-specific.
- `packages/keymanagement`/`packages/encryption` remain the only
  places raw secret material is ever stored; `ConnectorCredentials`
  names a secret by `SecretRef` handle only.
- `packages/caselifecycle.Case` and `packages/reportexport.Report`
  are referenced by field name only in `InboundCase`/`OutboundReport`/
  `FieldMapping`'s doc comments -- this package does not import either.
- `packages/compliance.Engine`'s authenticate/check-tenant/
  check-permission/mutate/audit-regardless-of-outcome shape is the
  reference `Engine` follows, not a dependency -- this package does
  not import `packages/compliance`.
