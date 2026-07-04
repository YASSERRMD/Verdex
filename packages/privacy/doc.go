// Package privacy is Phase 081: honoring data-subject rights and
// enforcing data minimization across every tenant's stored personal
// data. It draws on the PII detection/redaction primitives added in
// Phase 025 (packages/pii), the hash + chain-of-custody model added
// in Phase 020 (packages/provenance), the durable audit trail added
// in Phase 077 (packages/auditlog), and the role/permission model
// added in Phase 006 (packages/identity), composing them into a
// tenant-scoped, auditable data-subject-rights layer rather than
// duplicating any of them.
//
// # What is new here
//
//   - DataCategory / Sensitivity / LegalBasis / DataInventoryEntry
//     (types.go): a registry of what personal-data categories are
//     stored, where (by SourceTag reference, e.g. "case.parties",
//     "identity.user", "ingestion.transcript" -- this package does
//     not import those packages just to enumerate categories), on
//     what legal basis, and for how long (task 1). DataCategory is
//     deliberately its own closed enum, not a re-export of
//     packages/pii.PIICategory: a PIICategory classifies a single
//     detected text span within a document; a DataCategory classifies
//     a whole class of stored personal data at the system level.
//     Where the two do correspond (free-text content packages/pii can
//     detect within), AnonymizeForAnalytics maps one onto the other
//     rather than inventing a third taxonomy.
//   - RetentionPolicy / EnforceRetention (retention.go): per
//     DataCategory, a retention Window and a DeletionAction
//     (hard-delete vs anonymize). EnforceRetention is a real
//     evaluation function -- given a policy and a record's age, it
//     reports ActionRetain while within the window or the policy's
//     prescribed action once past it (task 3).
//   - ConsentRecord / HasValidConsent (consent.go): subject
//     identifier, tenant, purpose, legal basis, GrantedAt, and a
//     nilable WithdrawnAt. HasValidConsent evaluates a subject's full
//     consent history for a purpose, so a withdrawn-then-re-granted
//     subject correctly resolves to valid, and a withdrawn-with-no-
//     re-grant subject correctly resolves to invalid (task 6).
//   - SubjectAccessRequest / SARStatus / TransitionSAR (sar.go): a
//     tenant-scoped data-subject access request tracked through a
//     guarded status state machine (received -> in_progress ->
//     fulfilled/rejected), mirroring Phase 063's
//     packages/caselifecycle allowedTransitions-map +
//     CanTransition-guard shape by reference -- this package does not
//     import packages/caselifecycle (task 4).
//   - ErasureRequest / ErasureResult / Engine.ExecuteErasure
//     (erasure.go): task 5's centerpiece. An ErasureRequest may
//     reference the packages/provenance ProvenanceRecord describing
//     the content being erased (by ID and hash only, never by
//     importing packages/provenance's store); ExecuteErasure copies
//     that ProvenanceRecordID/ProvenanceHash into the returned
//     ErasureResult before invoking the caller-supplied ScrubFunc, so
//     the result's provenance fields are populated regardless of the
//     scrub's own outcome -- the chain-of-custody hash is
//     structurally guaranteed to survive, not merely promised. See
//     doc/privacy.md for the full explanation and non-negotiable
//     rationale.
//   - AnonymizeForAnalytics (anonymize.go): produces an
//     aggregated/pseudonymized projection of a record's free-text
//     fields suitable for analytics use, delegating every span-level
//     detection and redaction decision to
//     packages/pii.RuleBasedDetector, ClassifyMatches, and Redactor
//     rather than reimplementing NER/detection (task 8).
//   - AuditSink (audit.go): records every SAR transition, erasure
//     execution, consent change, and inventory registration into
//     packages/auditlog.Store -- the same durable, hash-chained sink
//     the rest of the platform already writes to and queries. No
//     second audit table (task 7).
//   - identity.PermViewPrivacy / identity.PermManagePrivacy
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewKeys/PermManageKeys precedent from Phase 076.
//
// # What is explicitly reused, not duplicated
//
//   - packages/pii's Detector/RuleBasedDetector/ClassifyMatches/
//     Redactor/RedactionMode remain the only PII detection and
//     redaction pipeline in this codebase; AnonymizeForAnalytics
//     calls through them rather than re-detecting or re-redacting.
//   - packages/provenance's ProvenanceRecord hash/chain-of-custody
//     model is preserved, never mutated, by erasure -- this package
//     has no dependency on packages/provenance's store at all, and
//     references a record purely by ID + hash string.
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/accessgovernance's own AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything privacy-specific.
//   - packages/caselifecycle's guarded state-transition shape
//     (allowedTransitions map + CanTransition guard function) is the
//     reference SARStatus/CanTransitionSAR/TransitionSAR follow, not
//     a dependency -- this package does not import
//     packages/caselifecycle.
//
// See doc/privacy.md for the full write-up.
package privacy
