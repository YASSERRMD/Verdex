package bulkimport

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Status is the coarse lifecycle state of an ImportJob. See
// Status.CanTransitionTo for the full guarded state machine, and
// doc/bulk-import.md for the state diagram.
type Status string

const (
	// StatusPending is the initial state of every ImportJob (see
	// NewImportJob): the job has been registered but RunBatch has not
	// yet processed any records.
	StatusPending Status = "pending"

	// StatusRunning means at least one batch has started processing
	// and the job has not yet reached a terminal or paused state.
	StatusRunning Status = "running"

	// StatusPaused means batch processing was explicitly paused
	// (Engine.Pause) after some records were processed; the job's
	// Cursor is preserved so RunBatch can resume later.
	StatusPaused Status = "paused"

	// StatusCompleted means every record from the source corpus has
	// been processed (imported, skipped as a duplicate, or rejected)
	// and no further batches remain.
	StatusCompleted Status = "completed"

	// StatusFailed means batch processing stopped because of an
	// unrecoverable error (as opposed to individual record-level
	// validation failures, which are recorded as Rejected
	// ImportRecords without failing the job).
	StatusFailed Status = "failed"

	// StatusRolledBack means Engine.Rollback reversed a job's
	// previously imported records. Only reachable from StatusCompleted
	// or StatusFailed.
	StatusRolledBack Status = "rolled_back"
)

// IsValid reports whether s is one of the named Status constants.
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusRunning, StatusPaused, StatusCompleted, StatusFailed, StatusRolledBack:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s Status) String() string { return string(s) }

// IsTerminal reports whether s is a status no further RunBatch call
// can leave via normal batch processing. StatusRolledBack is also
// terminal (a rolled-back job is never re-run).
func (s Status) IsTerminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusRolledBack:
		return true
	default:
		return false
	}
}

// statusTransitions is the single authoritative adjacency list for
// ImportJob's status state machine: Pending -> Running -> Paused ->
// Completed/Failed/RolledBack, mirroring
// packages/caselifecycle.State's transition-table approach (see
// transition.go there) rather than scattering ad hoc if-statements
// across every call site.
var statusTransitions = map[Status][]Status{
	StatusPending:    {StatusRunning, StatusFailed},
	StatusRunning:    {StatusRunning, StatusPaused, StatusCompleted, StatusFailed},
	StatusPaused:     {StatusRunning, StatusFailed},
	StatusCompleted:  {StatusRolledBack},
	StatusFailed:     {StatusRolledBack, StatusRunning},
	StatusRolledBack: {},
}

// CanTransitionTo reports whether moving from s to next is a legal
// ImportJob status transition.
func (s Status) CanTransitionTo(next Status) bool {
	for _, allowed := range statusTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// ImportJob is a single tenant-scoped bulk-import run: onboarding one
// historical case corpus from one source description. ImportJob tracks
// aggregate progress (total/processed/failed counts) and the resumable
// Cursor; the per-record detail (raw payload reference, validation
// status, dedup outcome) lives on ImportRecord.
type ImportJob struct {
	// ID uniquely identifies this import job.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this job to a tenant. Every Repository method is
	// scoped to a tenantID and refuses cross-tenant access.
	TenantID uuid.UUID `json:"tenant_id"`

	// SourceDescription is a short human-readable description of where
	// this corpus came from (e.g. "2019-2022 District Court archive
	// CSV export", "Acme DMS migration batch 3"). Required.
	SourceDescription string `json:"source_description"`

	// Status is the job's current lifecycle state.
	Status Status `json:"status"`

	// TotalRecords is the total number of records the source corpus is
	// expected to contain. May be zero if the total is not known ahead
	// of time (e.g. a streaming source), in which case Progress reports
	// an indeterminate percent.
	TotalRecords int `json:"total_records"`

	// ProcessedRecords is the count of records processed so far
	// (imported + skipped + rejected).
	ProcessedRecords int `json:"processed_records"`

	// FailedRecords is the count of records that were rejected due to
	// validation failure.
	FailedRecords int `json:"failed_records"`

	// SkippedRecords is the count of records skipped as true
	// duplicates of an already-imported record.
	SkippedRecords int `json:"skipped_records"`

	// ImportedRecords is the count of records successfully imported.
	ImportedRecords int `json:"imported_records"`

	// Cursor is the resumability checkpoint: the zero-based index, into
	// the job's RecordSource, of the next record RunBatch should read.
	// Persisted after every successfully processed batch so a crash
	// mid-job can resume from the last completed record rather than
	// restarting from zero (task 4).
	Cursor int `json:"cursor"`

	// CreatedBy is the identity.User who registered this job.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// StartedAt is when the job first transitioned into StatusRunning.
	// Nil until then.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// FinishedAt is when the job reached a terminal status
	// (Completed/Failed). Nil until then.
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// FailureReason explains why the job moved to StatusFailed. Blank
	// otherwise.
	FailureReason string `json:"failure_reason,omitempty"`
}

// NewImportJob constructs an ImportJob in StatusPending for tenantID,
// generating a new ID and stamping CreatedAt/UpdatedAt.
func NewImportJob(tenantID uuid.UUID, sourceDescription string, createdBy uuid.UUID, totalRecords int, now time.Time) ImportJob {
	return ImportJob{
		ID:                uuid.New(),
		TenantID:          tenantID,
		SourceDescription: sourceDescription,
		Status:            StatusPending,
		TotalRecords:      totalRecords,
		CreatedBy:         createdBy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// Validate checks j for structural well-formedness.
func (j *ImportJob) Validate() error {
	if j == nil {
		return ErrInvalidJob
	}
	if j.TenantID == uuid.Nil {
		return wrapf("ImportJob.Validate", ErrEmptyTenantID)
	}
	if strings.TrimSpace(j.SourceDescription) == "" {
		return wrapf("ImportJob.Validate", ErrInvalidJob)
	}
	if !j.Status.IsValid() {
		return wrapf("ImportJob.Validate", ErrInvalidJob)
	}
	if j.TotalRecords < 0 || j.ProcessedRecords < 0 || j.FailedRecords < 0 ||
		j.SkippedRecords < 0 || j.ImportedRecords < 0 || j.Cursor < 0 {
		return wrapf("ImportJob.Validate", ErrInvalidJob)
	}
	return nil
}

// Clone returns a deep-enough copy of j (the pointer fields
// StartedAt/FinishedAt are copied to fresh pointers) so callers can
// hand out ImportJob values without letting the caller mutate internal
// repository state through a shared pointer.
func (j ImportJob) Clone() ImportJob {
	cp := j
	if j.StartedAt != nil {
		t := *j.StartedAt
		cp.StartedAt = &t
	}
	if j.FinishedAt != nil {
		t := *j.FinishedAt
		cp.FinishedAt = &t
	}
	return cp
}

// ValidationStatus classifies whether an ImportRecord passed structural
// validation (task 3).
type ValidationStatus string

const (
	// ValidationPending means the record has not yet been validated.
	ValidationPending ValidationStatus = "pending"

	// ValidationPassed means the record passed every check the
	// configured Validator ran.
	ValidationPassed ValidationStatus = "passed"

	// ValidationFailed means the record failed at least one check; see
	// ImportRecord.ValidationErrors for the structured detail.
	ValidationFailed ValidationStatus = "failed"
)

// IsValid reports whether v is one of the named ValidationStatus
// constants.
func (v ValidationStatus) IsValid() bool {
	switch v {
	case ValidationPending, ValidationPassed, ValidationFailed:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (v ValidationStatus) String() string { return string(v) }

// Outcome classifies the final disposition of an ImportRecord once
// RunBatch has finished processing it.
type Outcome string

const (
	// OutcomePending means the record has not yet been processed.
	OutcomePending Outcome = "pending"

	// OutcomeImported means the record was validated, found not to be
	// a duplicate, and successfully imported.
	OutcomeImported Outcome = "imported"

	// OutcomeSkippedDuplicate means the record's DedupKey matched an
	// already-imported record within the same job/tenant, so it was
	// skipped rather than imported again.
	OutcomeSkippedDuplicate Outcome = "skipped_duplicate"

	// OutcomeRejected means the record failed validation and was not
	// imported. See ImportRecord.ValidationErrors for why.
	OutcomeRejected Outcome = "rejected"

	// OutcomeRolledBack means the record was previously Imported but
	// Engine.Rollback has since reversed it.
	OutcomeRolledBack Outcome = "rolled_back"
)

// IsValid reports whether o is one of the named Outcome constants.
func (o Outcome) IsValid() bool {
	switch o {
	case OutcomePending, OutcomeImported, OutcomeSkippedDuplicate, OutcomeRejected, OutcomeRolledBack:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (o Outcome) String() string { return string(o) }

// ValidationError is a single structured field-level validation
// failure attached to an ImportRecord, rather than a bare bool -- a
// caller building an error report for a corpus owner needs to know
// which field failed and why, not just that validation failed (task
// 3).
type ValidationError struct {
	// Field names the record field that failed validation (e.g.
	// "case_number", "jurisdiction").
	Field string `json:"field"`

	// Reason is a short, human-readable explanation of the failure.
	Reason string `json:"reason"`
}

// ImportRecord is one row of the source corpus being imported: a raw
// payload reference, its validation status/errors, its computed
// DedupKey, and its final Outcome.
type ImportRecord struct {
	// ID uniquely identifies this import record.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this record to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// JobID is the ImportJob this record belongs to.
	JobID uuid.UUID `json:"job_id"`

	// SourceIndex is this record's zero-based position within the
	// job's RecordSource, matching the value ImportJob.Cursor advances
	// past once this record is processed.
	SourceIndex int `json:"source_index"`

	// PayloadRef is a reference to the raw source payload for this
	// record (e.g. a row number, an object-storage key, a file byte
	// offset) -- reference only, mirroring
	// packages/privacy.DataInventoryEntry.SourceTag's convention. This
	// package never dereferences it; PayloadRef is purely for a
	// corpus owner tracing a record back to its origin.
	PayloadRef string `json:"payload_ref"`

	// CaseNumber, Jurisdiction, and PartyNames are the minimal
	// structural fields this package validates and dedups on. A real
	// deployment's corpus carries many more fields, but bulk import's
	// job is onboarding, not full case modeling -- richer per-case data
	// belongs to packages/caselifecycle once a record is imported.
	CaseNumber   string   `json:"case_number"`
	Jurisdiction string   `json:"jurisdiction"`
	PartyNames   []string `json:"party_names,omitempty"`

	// DedupKey is the computed deduplication key for this record (see
	// ComputeDedupKey), used to detect true duplicates within the same
	// job/tenant (task 5).
	DedupKey string `json:"dedup_key"`

	// ValidationStatus and ValidationErrors record the outcome of
	// running this record through a Validator (task 3).
	ValidationStatus ValidationStatus  `json:"validation_status"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`

	// Outcome is this record's final disposition.
	Outcome Outcome `json:"outcome"`

	// OutcomeReason elaborates on Outcome (e.g. which existing record
	// this one duplicated, or a rollback timestamp note). Blank when
	// Outcome doesn't need elaboration beyond ValidationErrors.
	OutcomeReason string `json:"outcome_reason,omitempty"`

	// CreatedCaseID references the packages/caselifecycle.Case created
	// from this record, if this deployment models downstream case
	// creation. Reference only, by convention -- mirroring
	// packages/compliance.Control.MappedTo and
	// packages/privacy.DataInventoryEntry.SourceTag -- this package
	// does not import packages/caselifecycle to dereference it. Nil
	// (uuid.Nil) until Outcome is OutcomeImported.
	CreatedCaseID uuid.UUID `json:"created_case_id,omitempty"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks r for structural well-formedness.
func (r *ImportRecord) Validate() error {
	if r == nil {
		return ErrInvalidRecord
	}
	if r.TenantID == uuid.Nil {
		return wrapf("ImportRecord.Validate", ErrEmptyTenantID)
	}
	if r.JobID == uuid.Nil {
		return wrapf("ImportRecord.Validate", ErrInvalidRecord)
	}
	if !r.ValidationStatus.IsValid() {
		return wrapf("ImportRecord.Validate", ErrInvalidRecord)
	}
	if !r.Outcome.IsValid() {
		return wrapf("ImportRecord.Validate", ErrInvalidRecord)
	}
	if r.SourceIndex < 0 {
		return wrapf("ImportRecord.Validate", ErrInvalidRecord)
	}
	return nil
}

// Clone returns a copy of r, including a fresh copy of the
// PartyNames/ValidationErrors slices so callers cannot mutate
// repository-internal state through a shared backing array.
func (r ImportRecord) Clone() ImportRecord {
	cp := r
	if r.PartyNames != nil {
		cp.PartyNames = append([]string(nil), r.PartyNames...)
	}
	if r.ValidationErrors != nil {
		cp.ValidationErrors = append([]ValidationError(nil), r.ValidationErrors...)
	}
	return cp
}
