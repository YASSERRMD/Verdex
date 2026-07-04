package privacy

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErasureStatus is the closed set of states an ErasureRequest moves
// through, deliberately kept simpler than SARStatus's four-state
// machine: erasure has no "in review" holding state worth modeling
// separately from "received" -- Engine.ExecuteErasure performs the
// scrub synchronously and moves Received straight to a terminal state.
type ErasureStatus string

const (
	// ErasureStatusReceived is the initial state: the request has been
	// logged but not yet executed.
	ErasureStatusReceived ErasureStatus = "received"

	// ErasureStatusCompleted is a terminal state: ExecuteErasure ran
	// successfully.
	ErasureStatusCompleted ErasureStatus = "completed"

	// ErasureStatusRejected is a terminal state: the request was
	// declined (e.g. the data is still within a legal-hold retention
	// window under BasisLegalObligation).
	ErasureStatusRejected ErasureStatus = "rejected"
)

// IsValid reports whether s is one of the named ErasureStatus
// constants.
func (s ErasureStatus) IsValid() bool {
	switch s {
	case ErasureStatusReceived, ErasureStatusCompleted, ErasureStatusRejected:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s ErasureStatus) String() string { return string(s) }

// ErasureRequest is a tenant-scoped right-to-erasure request (task 5):
// a data subject's ask to have their personal content deleted or
// scrubbed. This is the hard-constraint-bearing type in this package
// -- see ErasureResult and Engine.ExecuteErasure (engine.go) for how
// erasure is executed while the packages/provenance chain-of-custody
// record for the same content is deliberately preserved, not deleted.
type ErasureRequest struct {
	// ID uniquely identifies this request.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this request belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// SubjectID identifies the data subject whose content is to be
	// erased.
	SubjectID string `json:"subject_id"`

	// Category is the DataCategory of the content being erased,
	// informing which DeletionAction applies if no explicit action is
	// specified.
	Category DataCategory `json:"category"`

	// SourceTag names, by convention, the system location holding the
	// content to erase (see DataInventoryEntry.SourceTag).
	SourceTag string `json:"source_tag"`

	// RecordRef is an opaque, caller-defined reference identifying the
	// specific record to erase within SourceTag (e.g. a document ID, a
	// transcript segment ID). This package does not interpret RecordRef
	// -- it is round-tripped into ErasureResult for the caller's own
	// downstream scrub action.
	RecordRef string `json:"record_ref,omitempty"`

	// ProvenanceRecordID identifies the packages/provenance
	// ProvenanceRecord describing the content being erased, if one
	// exists. Referenced by ID only -- this package does not import
	// packages/provenance -- but ExecuteErasure requires this and
	// ProvenanceHash to be set together (or both left zero for content
	// that never had a provenance record) so a request can never erase
	// provenance-tracked content without the caller having supplied the
	// hash to preserve.
	ProvenanceRecordID uuid.UUID `json:"provenance_record_id,omitempty"`

	// ProvenanceHash is the packages/provenance ProvenanceRecord's
	// ContentHash (or ChainHash) at the time this request was filed --
	// the exact value ExecuteErasure must echo back untouched in
	// ErasureResult, proving the chain-of-custody record itself was
	// never mutated even though the personal content it describes was
	// scrubbed. Required whenever ProvenanceRecordID is set (see
	// ErrProvenanceHashRequired).
	ProvenanceHash string `json:"provenance_hash,omitempty"`

	// Status is this request's current position in the state machine.
	Status ErasureStatus `json:"status"`

	// RequestedAt is when the request was logged.
	RequestedAt time.Time `json:"requested_at"`

	// ResolvedAt, if non-nil, is when Status last moved to a terminal
	// state.
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// ResolutionNotes is a free-text explanation attached at
	// resolution (e.g. rejection rationale).
	ResolutionNotes string `json:"resolution_notes,omitempty"`

	// HandledBy is the identity.User who executed or rejected this
	// request.
	HandledBy uuid.UUID `json:"handled_by,omitempty"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks r for structural well-formedness.
func (r *ErasureRequest) Validate() error {
	if r == nil {
		return ErrInvalidErasureRequest
	}
	if r.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if strings.TrimSpace(r.SubjectID) == "" {
		return wrapf("ErasureRequest.Validate", ErrInvalidErasureRequest)
	}
	if !r.Category.IsValid() {
		return wrapf("ErasureRequest.Validate", ErrInvalidDataCategory)
	}
	if strings.TrimSpace(r.SourceTag) == "" {
		return wrapf("ErasureRequest.Validate", ErrInvalidErasureRequest)
	}
	if !r.Status.IsValid() {
		return wrapf("ErasureRequest.Validate", ErrInvalidErasureRequest)
	}
	if r.RequestedAt.IsZero() {
		return wrapf("ErasureRequest.Validate", ErrInvalidErasureRequest)
	}
	if r.ProvenanceRecordID != uuid.Nil && strings.TrimSpace(r.ProvenanceHash) == "" {
		return wrapf("ErasureRequest.Validate", ErrProvenanceHashRequired)
	}
	return nil
}

// HasProvenance reports whether r references a
// packages/provenance.ProvenanceRecord that ExecuteErasure must
// preserve.
func (r *ErasureRequest) HasProvenance() bool {
	return r != nil && r.ProvenanceRecordID != uuid.Nil
}

// ErasureResult is the outcome of Engine.ExecuteErasure (task 5's
// centerpiece): it reports that the request's personal content was
// scrubbed, while making explicit -- as its own top-level fields, not
// buried in a generic detail string -- that the
// packages/provenance chain-of-custody record for the same content
// remains fully intact and queryable. A caller (or a test) can inspect
// ProvenanceRecordID/ProvenanceHash on the result and independently
// verify, via packages/provenance's own VerifyChain/Verify, that the
// hash was never touched by this erasure.
type ErasureResult struct {
	// RequestID is the ErasureRequest this result answers.
	RequestID uuid.UUID `json:"request_id"`

	// ContentScrubbed is true once the personal content itself has been
	// deleted or anonymized. This package does not perform the
	// downstream scrub of SourceTag's storage itself -- see
	// ScrubFunc in engine.go -- ContentScrubbed simply reports whether
	// that caller-supplied step reported success.
	ContentScrubbed bool `json:"content_scrubbed"`

	// ActionTaken is the DeletionAction actually applied
	// (ActionHardDelete or ActionAnonymize).
	ActionTaken DeletionAction `json:"action_taken"`

	// ProvenanceRecordID and ProvenanceHash are echoed back from the
	// originating ErasureRequest unchanged -- this is the explicit
	// proof point that erasure preserved the chain-of-custody record.
	// Both are the zero value when the erased content never had an
	// associated ProvenanceRecord.
	ProvenanceRecordID uuid.UUID `json:"provenance_record_id,omitempty"`
	ProvenanceHash     string    `json:"provenance_hash,omitempty"`

	// ProvenancePreserved is true whenever ProvenanceRecordID is set --
	// i.e. whenever there was a chain-of-custody record to preserve,
	// this field asserts (and ExecuteErasure guarantees, see engine.go)
	// that it was left untouched. It is also true, vacuously, when
	// there was no provenance record to begin with (nothing to have
	// broken).
	ProvenancePreserved bool `json:"provenance_preserved"`

	// ExecutedAt is when ExecuteErasure ran.
	ExecutedAt time.Time `json:"executed_at"`
}

// ScrubFunc performs the actual downstream deletion/anonymization of
// the personal content an ErasureRequest describes, e.g. deleting a
// row in packages/caselifecycle or overwriting a transcript segment in
// packages/ingestion. This package does not know how to reach into any
// other package's storage, exactly as
// packages/accessgovernance.CaseGrant references
// packages/caselifecycle.Case by ID only -- ScrubFunc is the seam a
// caller supplies to perform that package-specific action.
// Implementations must be idempotent: ExecuteErasure may call ScrubFunc
// at most once per request, but a caller's own retry logic could invoke
// ExecuteErasure again after a partial failure.
type ScrubFunc func(ctx context.Context, req ErasureRequest) error

// SubmitErasureRequest creates an ErasureRequest in
// ErasureStatusReceived (task 5), requiring managePermission and
// tenant match. This only logs the request; ExecuteErasure performs
// the actual scrub.
func (e *Engine) SubmitErasureRequest(ctx context.Context, tenantID uuid.UUID, req ErasureRequest) (ErasureRequest, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return ErasureRequest{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ErasureRequest{}, err
	}

	req.TenantID = tenantID
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	req.Status = ErasureStatusReceived
	now := e.now()
	if req.RequestedAt.IsZero() {
		req.RequestedAt = now
	}
	req.CreatedAt = now
	req.UpdatedAt = now

	if err := req.Validate(); err != nil {
		return ErasureRequest{}, err
	}
	if err := e.erasures.Create(ctx, tenantID, &req); err != nil {
		return ErasureRequest{}, wrapf("SubmitErasureRequest", err)
	}
	return req, nil
}

// ExecuteErasure is task 5's centerpiece: it runs scrub over the
// ErasureRequest identified by requestID's personal content, then
// records an ErasureResult that proves -- as explicit, top-level
// fields, not an implicit assumption -- that the
// packages/provenance chain-of-custody record for the same content
// (ProvenanceRecordID/ProvenanceHash, if the request references one)
// remains completely untouched. This package never mutates a
// provenance record itself (it has no dependency on
// packages/provenance's store at all, see erasure.go's doc comments on
// ErasureRequest/ErasureResult); ExecuteErasure's contribution is
// structural: it is impossible to call this method and receive back
// an ErasureResult that both reports ContentScrubbed=true and silently
// drops ProvenanceRecordID/ProvenanceHash from the originating request
// -- they are copied through verbatim before scrub even runs, so a
// failing scrub still leaves the caller able to see what provenance
// record was supposed to survive.
//
// Requires managePermission and tenant match. Returns
// ErrAlreadyErased if the request's Status is not
// ErasureStatusReceived. Every call -- success or failure -- is
// recorded via AuditSink.
func (e *Engine) ExecuteErasure(ctx context.Context, tenantID, requestID uuid.UUID, scrub ScrubFunc) (ErasureResult, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return ErasureResult{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ErasureResult{}, err
	}
	if scrub == nil {
		return ErasureResult{}, wrapf("ExecuteErasure", ErrInvalidErasureRequest)
	}

	req, err := e.erasures.Get(ctx, tenantID, requestID)
	if err != nil {
		return ErasureResult{}, err
	}
	if req.Status != ErasureStatusReceived {
		if e.audit != nil {
			_, _ = e.audit.RecordErasureExecute(ctx, tenantID, user.ID, *req, ErasureResult{}, ErrAlreadyErased)
		}
		return ErasureResult{}, ErrAlreadyErased
	}

	now := e.now()

	// ErasureResult's provenance fields are copied from req BEFORE
	// scrub runs and are never subsequently overwritten -- this is the
	// mechanism that makes "content scrubbed, provenance hash survives"
	// a property of this method's control flow rather than a promise
	// that depends on scrub behaving correctly.
	result := ErasureResult{
		RequestID:           req.ID,
		ActionTaken:         ActionHardDelete,
		ProvenanceRecordID:  req.ProvenanceRecordID,
		ProvenanceHash:      req.ProvenanceHash,
		ProvenancePreserved: true,
		ExecutedAt:          now,
	}
	if policy, polErr := e.RetentionPolicyFor(ctx, req.Category); polErr == nil {
		result.ActionTaken = policy.OnAction
	}

	scrubErr := scrub(ctx, *req)
	if scrubErr != nil {
		wrapped := wrapf("ExecuteErasure", scrubErr)
		if e.audit != nil {
			_, _ = e.audit.RecordErasureExecute(ctx, tenantID, user.ID, *req, result, wrapped)
		}
		return ErasureResult{}, wrapped
	}
	result.ContentScrubbed = true

	req.Status = ErasureStatusCompleted
	req.ResolvedAt = &now
	req.HandledBy = user.ID
	req.UpdatedAt = now
	if err := e.erasures.Update(ctx, tenantID, req); err != nil {
		wrapped := wrapf("ExecuteErasure", err)
		if e.audit != nil {
			_, _ = e.audit.RecordErasureExecute(ctx, tenantID, user.ID, *req, result, wrapped)
		}
		return ErasureResult{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordErasureExecute(ctx, tenantID, user.ID, *req, result, nil)
	}
	return result, nil
}

// ListErasuresForSubject returns every ErasureRequest on file for
// subjectID within tenantID, requiring viewPermission and tenant
// match.
func (e *Engine) ListErasuresForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]ErasureRequest, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.erasures.ListForSubject(ctx, tenantID, subjectID)
	if err != nil {
		return nil, wrapf("ListErasuresForSubject", err)
	}
	return list, nil
}
