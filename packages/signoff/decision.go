package signoff

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// AcknowledgementConfirmation is the exact confirmation string a
// caller must supply to Approve or Reject. Requiring a fixed literal
// (rather than merely a non-empty string or a boolean flag) makes the
// acknowledgement explicit and intentional: a caller cannot
// accidentally satisfy it by passing through some other truthy value,
// and a human-facing UI is expected to require the reviewer to type
// or otherwise deliberately confirm this exact phrase before the
// request is even built.
const AcknowledgementConfirmation = "I acknowledge and approve this review decision"

// DecisionInput bundles the fields common to Approve and Reject.
type DecisionInput struct {
	// TenantID scopes the operation.
	TenantID uuid.UUID

	// CaseID identifies the case being reviewed. Required.
	CaseID uuid.UUID

	// CaseVersion is the packages/caselifecycle Case.MetadataVersion
	// the reviewer actually reviewed. Approve/Reject compare this
	// against CaseVersionReader's live value and fail with
	// ErrCaseVersionMismatch if they differ, so a reviewer can never
	// approve or reject content other than what they saw. Required
	// (must be > 0).
	CaseVersion int

	// Notes is the reviewer's free-text explanation. Required
	// (non-blank, after trimming) for Reject; optional for Approve.
	Notes string

	// Acknowledgement must equal AcknowledgementConfirmation exactly,
	// or the call fails with ErrAcknowledgementRequired. This is the
	// explicit-human-acknowledgement requirement: sign-off can never
	// happen implicitly or by a default value.
	Acknowledgement string
}

// Service is the sign-off workflow's write/read API: Approve, Reject,
// and the queries built on top of Repository and CaseVersionReader.
// It is the type callers construct once and reuse; GateImpl (see
// gate.go) wraps a Service (or, more precisely, its Repository) to
// satisfy guardrail.SignoffGate for CanFinalize.
type Service struct {
	repo       Repository
	caseReader CaseVersionReader
	notifier   NotificationSink
	clock      func() time.Time
}

// NewService builds a Service backed by repo and caseReader. notifier
// may be nil, in which case NoOpNotificationSink is used.
func NewService(repo Repository, caseReader CaseVersionReader, notifier NotificationSink) (*Service, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	if caseReader == nil {
		return nil, ErrNilCaseReader
	}
	if notifier == nil {
		notifier = NoOpNotificationSink{}
	}
	return &Service{repo: repo, caseReader: caseReader, notifier: notifier, clock: time.Now}, nil
}

func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock().UTC()
	}
	return time.Now().UTC()
}

// validateCommon runs the checks shared by Approve and Reject: actor
// authentication/permission, non-empty case ID, explicit
// acknowledgement, and a live case-version match. It returns the
// authenticated actor on success.
func (s *Service) validateCommon(ctx context.Context, in DecisionInput) (*identity.User, error) {
	if in.CaseID == uuid.Nil {
		return nil, ErrEmptyCaseID
	}
	if err := RequireSignoffPermission(ctx); err != nil {
		return nil, err
	}
	user, _ := identity.UserFromContext(ctx)

	if in.Acknowledgement != AcknowledgementConfirmation {
		return nil, ErrAcknowledgementRequired
	}

	liveVersion, err := s.caseReader.CaseVersion(ctx, in.TenantID, in.CaseID)
	if err != nil {
		return nil, wrapf("validateCommon", err)
	}
	if in.CaseVersion != liveVersion {
		return nil, ErrCaseVersionMismatch
	}

	return user, nil
}

// currentOrInitial returns the case's existing SignoffRecord, or a
// freshly-initialized Pending one (DecisionSourceInitial) if none
// exists yet.
func (s *Service) currentOrInitial(ctx context.Context, tenantID, caseID uuid.UUID, caseVersion int, at time.Time) (*SignoffRecord, error) {
	existing, err := s.repo.Get(ctx, tenantID, caseID)
	if err == nil {
		return existing, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	return &SignoffRecord{
		ID:          uuid.New(),
		CaseID:      caseID,
		TenantID:    tenantID,
		Status:      guardrail.SignoffPending,
		CaseVersion: caseVersion,
		Source:      DecisionSourceInitial,
		DecidedAt:   at,
		CreatedAt:   at,
	}, nil
}

// Approve records an approved sign-off decision for in.CaseID. It
// requires: an authenticated actor holding identity.PermSignOff, an
// exact AcknowledgementConfirmation, and a CaseVersion matching the
// case's live version. Notes are optional. Approve appends an
// AuditEntry and persists the updated SignoffRecord.
func (s *Service) Approve(ctx context.Context, in DecisionInput) (*SignoffRecord, error) {
	user, err := s.validateCommon(ctx, in)
	if err != nil {
		return nil, wrapf("Approve", err)
	}

	at := s.now()
	current, err := s.currentOrInitial(ctx, in.TenantID, in.CaseID, in.CaseVersion, at)
	if err != nil {
		return nil, wrapf("Approve", err)
	}
	fromStatus := current.Status

	rec := &SignoffRecord{
		ID:          current.ID,
		CaseID:      in.CaseID,
		TenantID:    in.TenantID,
		Status:      guardrail.SignoffApproved,
		ReviewerID:  user.ID,
		Notes:       strings.TrimSpace(in.Notes),
		CaseVersion: in.CaseVersion,
		Source:      DecisionSourceReviewer,
		DecidedAt:   at,
		CreatedAt:   current.CreatedAt,
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = at
	}

	if err := s.repo.Upsert(ctx, in.TenantID, rec); err != nil {
		return nil, wrapf("Approve", err)
	}
	if err := s.repo.AppendAudit(ctx, in.TenantID, &AuditEntry{
		ID:          uuid.New(),
		CaseID:      in.CaseID,
		TenantID:    in.TenantID,
		FromStatus:  fromStatus,
		ToStatus:    guardrail.SignoffApproved,
		Actor:       user.ID,
		Source:      DecisionSourceReviewer,
		Notes:       rec.Notes,
		CaseVersion: in.CaseVersion,
		OccurredAt:  at,
	}); err != nil {
		return nil, wrapf("Approve", err)
	}

	return rec, nil
}

// Reject records a rejected sign-off decision for in.CaseID. It
// requires everything Approve requires, plus non-blank Notes
// (ErrNotesRequired if blank after trimming) — a rejection must
// always explain itself.
func (s *Service) Reject(ctx context.Context, in DecisionInput) (*SignoffRecord, error) {
	notes := strings.TrimSpace(in.Notes)
	if notes == "" {
		return nil, wrapf("Reject", ErrNotesRequired)
	}

	user, err := s.validateCommon(ctx, in)
	if err != nil {
		return nil, wrapf("Reject", err)
	}

	at := s.now()
	current, err := s.currentOrInitial(ctx, in.TenantID, in.CaseID, in.CaseVersion, at)
	if err != nil {
		return nil, wrapf("Reject", err)
	}
	fromStatus := current.Status

	rec := &SignoffRecord{
		ID:          current.ID,
		CaseID:      in.CaseID,
		TenantID:    in.TenantID,
		Status:      guardrail.SignoffRejected,
		ReviewerID:  user.ID,
		Notes:       notes,
		CaseVersion: in.CaseVersion,
		Source:      DecisionSourceReviewer,
		DecidedAt:   at,
		CreatedAt:   current.CreatedAt,
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = at
	}

	if err := s.repo.Upsert(ctx, in.TenantID, rec); err != nil {
		return nil, wrapf("Reject", err)
	}
	if err := s.repo.AppendAudit(ctx, in.TenantID, &AuditEntry{
		ID:          uuid.New(),
		CaseID:      in.CaseID,
		TenantID:    in.TenantID,
		FromStatus:  fromStatus,
		ToStatus:    guardrail.SignoffRejected,
		Actor:       user.ID,
		Source:      DecisionSourceReviewer,
		Notes:       notes,
		CaseVersion: in.CaseVersion,
		OccurredAt:  at,
	}); err != nil {
		return nil, wrapf("Reject", err)
	}

	return rec, nil
}

// Get returns the current SignoffRecord for caseID, scoped to
// tenantID, requiring identity.PermViewCase. Returns a synthetic
// Pending record (not persisted) if the case has never entered the
// sign-off workflow, so callers do not need to special-case
// ErrNotFound for "nobody has reviewed this yet".
func (s *Service) Get(ctx context.Context, tenantID, caseID uuid.UUID) (*SignoffRecord, error) {
	if err := RequireViewPermission(ctx); err != nil {
		return nil, err
	}
	if caseID == uuid.Nil {
		return nil, ErrEmptyCaseID
	}

	rec, err := s.repo.Get(ctx, tenantID, caseID)
	if err == nil {
		return rec, nil
	}
	if err != ErrNotFound {
		return nil, wrapf("Get", err)
	}

	liveVersion, verr := s.caseReader.CaseVersion(ctx, tenantID, caseID)
	if verr != nil {
		return nil, wrapf("Get", verr)
	}
	now := s.now()
	return &SignoffRecord{
		CaseID:      caseID,
		TenantID:    tenantID,
		Status:      guardrail.SignoffPending,
		CaseVersion: liveVersion,
		Source:      DecisionSourceInitial,
		DecidedAt:   now,
		CreatedAt:   now,
	}, nil
}

// History returns the full sign-off audit trail for caseID, scoped to
// tenantID, ordered oldest-first, requiring identity.PermViewCase.
func (s *Service) History(ctx context.Context, tenantID, caseID uuid.UUID) ([]*AuditEntry, error) {
	if err := RequireViewPermission(ctx); err != nil {
		return nil, err
	}
	if caseID == uuid.Nil {
		return nil, ErrEmptyCaseID
	}
	return s.repo.ListAudit(ctx, tenantID, caseID)
}
