// Package integration is Phase 087: connecting to external court
// case-management systems where permitted -- importing cases inbound
// and delivering reports outbound -- through one adapter interface
// rather than a bespoke client per court system. It draws on the
// LLMProvider/Registry adapter pattern added in Phase 011
// (packages/provider), the durable, hash-chained audit trail added in
// Phase 077 (packages/auditlog), and by-reference-only conventions
// established by packages/accessgovernance (Phase 080) for
// packages/caselifecycle (Phase 063) and by packages/compliance
// (Phase 082) for tag-referenced platform features, composing them
// into a tenant-scoped, auditable integration framework rather than
// duplicating any of them.
//
// # What is new here
//
//   - Connector / ConnectorCapability (connector.go, task 1): the
//     adapter contract every external system integration must
//     satisfy -- ID/Capabilities/ImportCases/DeliverReport/Ping --
//     mirroring packages/provider.LLMProvider's interface shape
//     exactly, applied to court case-management systems instead of
//     model providers. InboundCase and OutboundReport are this
//     package's own transfer types: InboundCase carries an external
//     system's raw field representation (never
//     packages/caselifecycle.Case itself); OutboundReport carries a
//     rendered report payload addressed by external case ID (never
//     packages/reportexport.Report itself). Both packages are
//     referenced by field name only in comments, never imported (task
//     1's "thin dependency footprint" requirement).
//   - Registry (registry.go, task 1): a thread-safe connector-ID ->
//     Connector map, mirroring packages/provider.Registry's
//     Register/Get/List/Unregister/DefaultRegistry shape precisely.
//   - ConnectorConfig (types.go): a tenant's registered binding from a
//     connector type to a Registry entry, its endpoint, and references
//     (by ID) to its ConnectorCredentials and FieldMapping records.
//   - ImportRun / summarizeImportRun (importrun.go, task 2): the
//     durable, tenant-scoped outcome record of one inbound case import
//     attempt -- window, per-run counts, and which external IDs (if
//     any) failed FieldMapping.Apply or downstream acceptance, with a
//     real ImportRunStatusSucceeded/Partial/Failed derivation, not a
//     status that is always "succeeded".
//   - DeliveryRun / statusFromReceipt (deliveryrun.go, task 3): the
//     outbound counterpart, recording one DeliverReport attempt's
//     outcome (accepted/rejected/failed) and its DeliveryReceipt.
//   - FieldMapping / FieldRule / Apply / Reverse (fieldmapping.go,
//     task 4): a real, tenant-configurable translation between this
//     platform's case/party/document field names and an external
//     system's own schema -- Apply enforces Required fields
//     (ErrUnmappedField) and applies DefaultValue for optional ones;
//     Reverse is the exact mirror image for outbound metadata. Not a
//     type with no logic.
//   - ConnectorCredentials / CredentialKind / AuthorizeCredentials
//     (credentials.go, task 5): an API-key / OAuth-client-credentials /
//     mutual-TLS / no-auth shape that never stores raw secret material
//     -- only a SecretRef handle into packages/keymanagement or
//     packages/encryption, resolved by the caller at call time.
//     AuthorizeCredentials performs structural Validate plus an
//     optional live VerifyFunc round-trip before any Connector call is
//     attempted.
//   - RetryPolicy / WithRetry (retry.go, task 6): a real
//     retry-with-backoff wrapper around any Connector call -- bounded
//     MaxAttempts, jittered exponential backoff, context-cancellation-
//     aware, with an optional per-error IsRetryable veto.
//   - ReconciliationResult / Reconcile (reconcile.go, task 6): a real
//     set-comparison function -- expected external IDs versus observed
//     external IDs -- surfacing MissingExternalIDs and
//     UnexpectedExternalIDs as durable, tenant-scoped drift detection,
//     not a stub that always reports clean.
//   - AuditSink (audit.go, task 7): records every connector
//     registration, credentials change, import run, delivery run, and
//     reconciliation attempt into packages/auditlog.Store -- the same
//     durable, hash-chained sink the rest of the platform already
//     writes to and queries. No second audit table. Credential events
//     never log SecretRef or any secret material.
//   - SandboxConnector (sandbox.go, task 8): a deterministic, real
//     in-process fake Connector implementation -- Verdex's
//     "no-op/mock provider for tests" equivalent from Phase 011
//     (packages/provider.NoOpProvider), applied to court
//     case-management systems: seedable InboundCase fixtures, an
//     inspectable delivery history, and an Unavailable toggle for
//     exercising retry/reconciliation against a connector that is
//     briefly down.
//   - Engine (engine.go): the tenant- and permission-scoped
//     orchestrator composing all of the above --
//     RegisterConnectorConfig, SetCredentials, SetFieldMapping,
//     RunImport, RunDelivery, RunReconciliation, and the List*
//     read-side methods -- mirroring packages/compliance.Engine's
//     authenticate/check-tenant/check-permission/mutate/audit-
//     regardless-of-outcome shape exactly.
//   - identity.PermViewIntegration / identity.PermManageIntegration
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewCompliance/PermManageCompliance precedent from
//     Phase 082.
//
// # Storage
//
// Six tenant-scoped tables back this package (migrations
// 000034_create_integration and 000035_enable_rls_integration):
// integration_connector_configs, integration_connector_credentials
// (secret_ref handle column only, never raw secret bytes),
// integration_field_mappings, and the append-only
// integration_import_runs / integration_delivery_runs /
// integration_reconciliation_results run-history tables. Unlike
// packages/compliance's shared compliance_controls catalogue, every
// table here is per-tenant data with its own Row-Level Security
// tenant_isolation policy. The usual three-layer repository pattern
// applies: ConfigRepository/CredentialsRepository/
// FieldMappingRepository/ImportRunRepository/DeliveryRunRepository/
// ReconciliationRepository interfaces (repository.go), InMemory*
// implementations for tests (inmemory_repository.go), Postgres*
// implementations accepting a persistence.Executor per call
// (postgres_repository.go), and TenantScoped* implementations
// composing packages/tenancy.WithTenantScope with their Postgres
// counterpart so RLS enforces isolation at the database layer in
// addition to the application-level requireMatchingTenant guard every
// repository already applies (tenant_scoped_repository.go).
//
// # What is explicitly reused, not duplicated
//
//   - packages/provider.LLMProvider's interface shape and
//     packages/provider.Registry's Register/Get/List pattern (Phase
//     011) are the reference Connector/Registry follow, not a
//     dependency -- this package does not import packages/provider.
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/compliance's and packages/privacy's own
//     AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything integration-specific.
//   - packages/keymanagement and packages/encryption remain the only
//     places raw secret material is ever stored; ConnectorCredentials
//     names a secret by SecretRef handle only, exactly as
//     packages/compliance.Control.MappedTo references platform
//     features by string tag without importing the tagged packages.
//   - packages/caselifecycle.Case's ID/TenantID/JurisdictionID/
//     CategoryID/Title/Reference/State/Metadata fields (Phase 063) and
//     packages/reportexport.Report's CaseID/TenantID/CaseTitle/
//     CaseReference/JurisdictionKey fields (Phase 073) are referenced
//     by name only in InboundCase/OutboundReport/FieldMapping's doc
//     comments -- this package does not import either package, mirroring
//     how packages/accessgovernance.CaseGrant references
//     packages/caselifecycle.Case by CaseID only.
//   - packages/compliance.Engine's authenticate/check-tenant/
//     check-permission/mutate/audit-regardless-of-outcome shape (Phase
//     082) is the reference Engine follows, not a dependency -- this
//     package does not import packages/compliance.
//
// See doc/integration-framework.md for the full write-up.
package integration
