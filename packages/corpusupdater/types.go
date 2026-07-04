package corpusupdater

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// CorpusTarget names which corpus a CorpusUpdateJob or Amendment
// affects. A closed enum: this package only ever triggers updates
// against the two corpora Phase 035 (packages/statute) and Phase 036
// (packages/precedent) established, referenced by tag rather than
// import.
type CorpusTarget string

const (
	// CorpusStatute targets packages/statute's rule corpus.
	CorpusStatute CorpusTarget = "statute"

	// CorpusPrecedent targets packages/precedent's holding corpus.
	CorpusPrecedent CorpusTarget = "precedent"
)

// IsValid reports whether t is one of the named CorpusTarget
// constants.
func (t CorpusTarget) IsValid() bool {
	switch t {
	case CorpusStatute, CorpusPrecedent:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (t CorpusTarget) String() string { return string(t) }

// JobStatus is a CorpusUpdateJob's position in its state machine.
// Deliberately closed (unlike compliance.Framework) since every job
// this package creates follows the exact same lifecycle -- see
// IsValidTransition for the allowed edges.
type JobStatus string

const (
	// StatusPending is a job's initial state: created, not yet
	// validated.
	StatusPending JobStatus = "pending"

	// StatusValidating is a job whose staged Amendments are being
	// structurally checked (see Validate).
	StatusValidating JobStatus = "validating"

	// StatusApplying is a job whose validated Amendments are being
	// written to the target corpus (text change, re-embedding, change
	// notification).
	StatusApplying JobStatus = "applying"

	// StatusApplied is a job whose Amendments were all applied
	// successfully.
	StatusApplied JobStatus = "applied"

	// StatusFailed is a job that could not be validated or applied.
	StatusFailed JobStatus = "failed"

	// StatusRolledBack is a previously StatusApplied job whose
	// Amendments have all been reverted to their pre-amendment state
	// via Engine.Rollback.
	StatusRolledBack JobStatus = "rolled_back"
)

// IsValid reports whether s is one of the named JobStatus constants.
func (s JobStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusValidating, StatusApplying, StatusApplied, StatusFailed, StatusRolledBack:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s JobStatus) String() string { return string(s) }

// jobTransitions enumerates the allowed "from -> to" edges of the
// JobStatus state machine: Pending -> Validating -> Applying ->
// (Applied | Failed), plus Applied -> RolledBack (Engine.Rollback) and
// Validating -> Failed (a validation failure short-circuits before
// Applying).
var jobTransitions = map[JobStatus][]JobStatus{
	StatusPending:    {StatusValidating, StatusFailed},
	StatusValidating: {StatusApplying, StatusFailed},
	StatusApplying:   {StatusApplied, StatusFailed},
	StatusApplied:    {StatusRolledBack},
	StatusFailed:     {},
	StatusRolledBack: {},
}

// IsValidTransition reports whether moving a CorpusUpdateJob from
// status `from` to status `to` is a legal edge in the JobStatus state
// machine. Unknown statuses always return false.
func IsValidTransition(from, to JobStatus) bool {
	allowed, ok := jobTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// CorpusUpdateJob is a jurisdiction-scoped unit of work describing one
// batch of incoming changes to a TargetCorpus (task 1). A job owns
// zero or more staged Amendments (see AmendmentRepository.ListForJob)
// and moves through the JobStatus state machine as those amendments
// are validated, applied, and (optionally) rolled back.
type CorpusUpdateJob struct {
	// ID uniquely identifies this job.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this job to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// JurisdictionCode names the jurisdiction this job's amendments
	// apply to (e.g. "AE-DXB", mirroring packages/jurisdiction's own
	// opaque-string convention -- this package does not import
	// packages/jurisdiction).
	JurisdictionCode string `json:"jurisdiction_code"`

	// TargetCorpus names which corpus this job updates.
	TargetCorpus CorpusTarget `json:"target_corpus"`

	// SourceDescription is a short human-readable note on where this
	// batch of changes came from (e.g. "Official Gazette Issue 412",
	// "Court of Cassation bulletin Q3 2026").
	SourceDescription string `json:"source_description"`

	// Status is this job's current position in the JobStatus state
	// machine.
	Status JobStatus `json:"status"`

	// FailureReason carries a short explanation when Status ==
	// StatusFailed. Empty otherwise.
	FailureReason string `json:"failure_reason,omitempty"`

	// CreatedBy is the identity.User who created this job.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt, UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks j for structural well-formedness. It does not check
// the JobStatus state machine transition itself -- see
// IsValidTransition for that.
func (j *CorpusUpdateJob) Validate() error {
	if j == nil {
		return ErrInvalidJob
	}
	if strings.TrimSpace(j.JurisdictionCode) == "" {
		return wrapf("CorpusUpdateJob.Validate", ErrInvalidJob)
	}
	if !j.TargetCorpus.IsValid() {
		return wrapf("CorpusUpdateJob.Validate", ErrInvalidCorpusTarget)
	}
	if !j.Status.IsValid() {
		return wrapf("CorpusUpdateJob.Validate", ErrInvalidJob)
	}
	return nil
}

// ChangeType names the kind of change an Amendment makes to its
// TargetID.
type ChangeType string

const (
	// ChangeTypeAdd introduces a brand new rule/precedent entry.
	// TargetID is conventionally empty for an Add (the new entry does
	// not exist yet); callers that already know the ID the new entry
	// will be persisted under may still set TargetID.
	ChangeTypeAdd ChangeType = "add"

	// ChangeTypeAmend replaces an existing rule/precedent's text.
	// Requires a non-empty TargetID.
	ChangeTypeAmend ChangeType = "amend"

	// ChangeTypeRepeal marks an existing rule/precedent as no longer in
	// force. Requires a non-empty TargetID.
	ChangeTypeRepeal ChangeType = "repeal"
)

// IsValid reports whether c is one of the named ChangeType constants.
func (c ChangeType) IsValid() bool {
	switch c {
	case ChangeTypeAdd, ChangeTypeAmend, ChangeTypeRepeal:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (c ChangeType) String() string { return string(c) }

// Amendment is a single staged change to a statute section or
// precedent entry (task 2), belonging to exactly one CorpusUpdateJob.
// TargetID references an existing packages/statute.RuleNode.ID or
// packages/precedent node ID by string, or a new one being minted for
// ChangeTypeAdd -- this package does not import either corpus package,
// exactly mirroring packages/compliance.Control.MappedTo's
// reference-by-tag convention.
type Amendment struct {
	// ID uniquely identifies this amendment.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this amendment to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// JobID is the CorpusUpdateJob this amendment belongs to.
	JobID uuid.UUID `json:"job_id"`

	// TargetCorpus names which corpus TargetID resolves against.
	TargetCorpus CorpusTarget `json:"target_corpus"`

	// TargetID is the rule/precedent node ID this amendment changes.
	// Required for ChangeTypeAmend and ChangeTypeRepeal; optional for
	// ChangeTypeAdd.
	TargetID string `json:"target_id,omitempty"`

	// ChangeType names the kind of change this amendment makes.
	ChangeType ChangeType `json:"change_type"`

	// NewText is the rule/precedent's text after this amendment takes
	// effect. Empty for ChangeTypeRepeal (a repeal has no replacement
	// text).
	NewText string `json:"new_text,omitempty"`

	// Citation is the amending instrument's own citation (e.g.
	// "Federal Decree-Law No. 45 of 2023, Art. 12"). Required.
	Citation string `json:"citation"`

	// EffectiveDate is when this amendment takes legal effect. See
	// IsEffective.
	EffectiveDate time.Time `json:"effective_date"`

	// PreviousText is the TargetID's text as it read immediately before
	// this amendment was applied, captured by Engine.ApplyAmendment at
	// apply time so Engine.Rollback can restore it. Empty until the
	// amendment is actually applied.
	PreviousText string `json:"previous_text,omitempty"`

	// PreviousCitation is the TargetID's citation as it read
	// immediately before this amendment was applied, captured
	// alongside PreviousText for the same rollback purpose.
	PreviousCitation string `json:"previous_citation,omitempty"`

	// Applied reports whether this amendment has been applied (its
	// PreviousText/PreviousCitation snapshot taken and its new text
	// written). Distinct from the owning job's Status: a job can fail
	// partway through StatusApplying with some amendments Applied and
	// others not.
	Applied bool `json:"applied"`

	// RolledBack reports whether a previously Applied amendment has
	// since been reverted via Engine.Rollback.
	RolledBack bool `json:"rolled_back"`

	// CreatedBy is the identity.User who staged this amendment.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt, UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsEffective reports whether a is "live" at instant now: staged
// amendments are not visible to any query path until their
// EffectiveDate has passed (task 3). A zero EffectiveDate is never
// effective (Validate rejects a zero EffectiveDate outright, but
// IsEffective stays defensive for callers constructing an Amendment
// directly in a test).
func (a Amendment) IsEffective(now time.Time) bool {
	if a.EffectiveDate.IsZero() {
		return false
	}
	return !a.EffectiveDate.After(now)
}
