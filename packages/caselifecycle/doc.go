// Package caselifecycle defines the canonical Case entity and its
// lifecycle state machine — the first time a Case is modeled as a
// first-class, persisted entity anywhere in Verdex.
//
// # Why this package exists now
//
// Every package built through Phase 062 (packages/ingestion,
// packages/category, packages/timeline, packages/grounding,
// packages/reasoningeval, and others) already threads a CaseID string
// through its own API, but none of them define what a "case" actually
// is: there has never been a row, a struct, or a state machine behind
// that string. This package is that missing entity. It does not
// replace or re-derive any of those packages' own concerns —
// packages/category still owns categorization logic,
// packages/timeline still owns party/event modeling, and
// packages/ingestion still owns its own per-job ingestion-pipeline
// status (see the "Relationship to ingestion status" section below).
// This package owns exactly one thing: the Case record itself
// (Case, in types.go) and the coarse lifecycle state it moves through
// from intake to closure (State, in types.go).
//
// # The state machine
//
//	        Transition (draft->active)
//	draft ────────────────────────────► active
//	                                      │  ▲
//	            Transition (active->under_review)
//	                                      │  │
//	                                      ▼  │ Transition (under_review->active)
//	                               under_review
//	                                      │
//	            Transition (under_review->closed)
//	                                      │
//	                                      ▼
//	                                   closed
//	                                      │
//	                  Reopen (closed->active, requires justification)
//	                                      │
//	                                      ▼
//	                                   active
//	                                      │
//	                   Archive (closed->archived, terminal)
//	                                      ▼
//	                                  archived
//
// draft is the only entry state (a case starts in draft the moment it
// is created — see NewCase in case.go) and archived is the only true
// terminal state: nothing transitions out of archived. See
// transition.go for the exact allowed-transition table
// (allowedTransitions) and the guard function (Transition) that
// enforces it; see doc/case-lifecycle.md for the full state diagram
// and rationale written out in prose.
//
// # Reopen and archive
//
// Reopen (reopen.go) is a distinct operation from a plain
// Transition(closed, active) call: it requires a caller-supplied
// justification string and always records a TransitionRecord whose
// Reason captures that justification, so "why was this closed case
// reopened" is always answerable from the audit log. Archive
// (archive.go) moves a case to StateArchived, a terminal state
// distinct from StateClosed: a closed case can still be reopened, an
// archived case cannot — archiving is the explicit, irreversible
// end of a case's lifecycle in this package.
//
// # Metadata
//
// A Case carries a free-form Metadata map (case.go) for fields this
// package does not know about in advance (e.g. docket numbers,
// external system references, jurisdiction-specific flags). SetMetadata
// and MergeMetadata (metadata.go) are the only sanctioned way to
// mutate it: both validate keys/values and bump the Case's metadata
// version, so concurrent metadata writers can detect a lost update.
//
// # Permitted actions per state
//
// PermittedActions (actions.go) maps each State to the set of
// operations downstream packages may perform against a case in that
// state (e.g. ActionIngestEvidence is permitted in StateDraft and
// StateActive but not StateClosed or StateArchived). Downstream
// packages call CanPerform as a guard before mutating case-scoped
// data; this package does not and cannot enforce that call happens —
// it only defines the table those packages should consult.
//
// # Audit log
//
// Every successful Transition, Reopen, and Archive call appends a
// TransitionRecord (audit.go) capturing the actor (from
// packages/identity.UserFromContext), timestamp, from/to state, and
// reason. Repository.AppendTransition/ListTransitions persist and
// query these records; this reuses packages/observability's
// AuditEvent shape by projecting every TransitionRecord into one (see
// TransitionRecord.ToAuditEvent) rather than inventing a second,
// parallel audit-logging channel.
//
// # Bulk operations
//
// BulkTransition and BulkSetMetadata (bulk.go) apply one operation
// across many case IDs and report a per-case BulkResult rather than
// failing the whole batch when one case is missing, cross-tenant, or
// fails a transition guard — see bulk.go's doc comment for the exact
// partial-failure contract.
//
// # Persistence and tenant isolation
//
// Repository (repository.go) is the storage-facing interface;
// PostgresRepository is its PostgreSQL-backed implementation (see
// packages/persistence/migrations/000006_create_cases.up.sql for the
// schema) and InMemoryRepository is an in-process implementation for
// tests and for other packages' test fixtures, mirroring
// packages/tenancy's TenantScopedDeploymentRepository /
// PostgresProvisioningRecordRepository split. Every Repository method
// takes a tenantID and refuses (via ErrCrossTenantAccess, checked
// before any database access — see requireMatchingTenant in
// errors.go) to operate on a Case whose TenantID does not match. The
// Postgres implementation additionally composes
// packages/tenancy.WithTenantScope so Row-Level Security enforces the
// same isolation at the database layer as defense-in-depth, exactly
// as packages/tenancy.TenantScopedDeploymentRepository does for
// Deployment.
//
// # Relationship to ingestion status
//
// packages/ingestion's own status.go/progress.go track the internal
// state of a single ingestion job (queued, transcribing, extracting,
// etc.) — a fine-grained, per-artifact pipeline concern. This
// package's State tracks the case's own coarse lifecycle (draft,
// active, under_review, closed, archived) — a case-wide concern that
// outlives any single ingestion job and is not derived from ingestion
// status. A case can sit in StateActive while ingestion jobs for it
// come and go; PermittedActions is what governs whether a new
// ingestion job may even be started against a case in a given State,
// not the other way around.
//
// See doc/case-lifecycle.md for the full write-up.
package caselifecycle
