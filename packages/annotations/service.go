package annotations

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// Service is the access-controlled, audited entrypoint for annotation
// operations, composing a Repository (storage) with
// packages/caselifecycle.Repository (case-accessibility checks) and a
// MentionSink (mention delivery), mirroring how packages/casesearch's
// Engine composes packages/caselifecycle.Repository rather than
// reimplementing case lookup, and how packages/signoff.Service
// composes a NotificationSink.
//
// Every method requires ctx to carry an authenticated identity.User
// (see access.go): reads require identity.PermViewCase, writes
// require identity.PermEditCase. Beyond the permission check, every
// operation also confirms the target case is visible to the actor's
// tenant via cases.Get, so annotations can never be read or written
// against a case the tenant cannot see (task 6's cross-tenant-leakage
// contract).
type Service struct {
	repo    Repository
	cases   caselifecycle.Repository
	mention MentionSink
}

// NewService builds a Service. cases and repo must be non-nil.
// mention may be nil, in which case NoOpMentionSink is used.
func NewService(repo Repository, cases caselifecycle.Repository, mention MentionSink) (*Service, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	if cases == nil {
		return nil, ErrNilCaseReader
	}
	if mention == nil {
		mention = NoOpMentionSink{}
	}
	return &Service{repo: repo, cases: cases, mention: mention}, nil
}

// checkCaseAccess confirms caseID is visible to tenantID via the
// composed caselifecycle.Repository, translating any lookup failure
// into ErrForbidden so callers cannot distinguish "case does not
// exist" from "case belongs to another tenant" — the same
// information-hiding posture packages/casesearch takes.
func (s *Service) checkCaseAccess(ctx context.Context, tenantID, caseID uuid.UUID) error {
	if _, err := s.cases.Get(ctx, tenantID, caseID); err != nil {
		return ErrForbidden
	}
	return nil
}

// Create validates and persists a new annotation, then records an
// AuditCreated entry and notifies MentionSink for every "@<userID>"
// token found in a.Body. Requires identity.PermEditCase.
func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, a *Annotation) (*Annotation, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrNilAnnotation
	}
	if err := s.checkCaseAccess(ctx, tenantID, a.CaseID); err != nil {
		return nil, err
	}
	a.AuthorID = user.ID
	if err := s.repo.Create(ctx, tenantID, a); err != nil {
		return nil, err
	}
	if err := s.repo.AppendAudit(ctx, tenantID, newAuditRecord(a, AuditCreated, user.ID)); err != nil {
		return nil, err
	}
	s.notifyMentions(ctx, a)
	return a, nil
}

// notifyMentions parses a.Body for mentions and calls MentionSink.Notify
// for each one. Delivery errors are swallowed (mirroring
// packages/signoff's "notification is best-effort" posture) so a sink
// outage never blocks the annotation write that already succeeded.
func (s *Service) notifyMentions(ctx context.Context, a *Annotation) {
	for _, userID := range ExtractMentions(a.Body) {
		_ = s.mention.Notify(ctx, Mention{
			AnnotationID:    a.ID,
			CaseID:          a.CaseID,
			TenantID:        a.TenantID,
			AuthorID:        a.AuthorID,
			MentionedUserID: userID,
			CreatedAt:       a.CreatedAt,
		})
	}
}

// Get returns the annotation with the given id, scoped to tenantID.
// Requires identity.PermViewCase.
func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	if _, err := authorizeView(ctx); err != nil {
		return nil, err
	}
	a, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.checkCaseAccess(ctx, tenantID, a.CaseID); err != nil {
		return nil, err
	}
	return a, nil
}

// ListByCase returns every annotation for caseID visible to tenantID,
// optionally narrowed by filter. Requires identity.PermViewCase.
func (s *Service) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, filter AnchorFilter) ([]*Annotation, error) {
	if _, err := authorizeView(ctx); err != nil {
		return nil, err
	}
	if err := s.checkCaseAccess(ctx, tenantID, caseID); err != nil {
		return nil, err
	}
	return s.repo.ListByCase(ctx, tenantID, caseID, filter)
}

// Thread returns rootID's annotation and its ordered replies. Requires
// identity.PermViewCase.
func (s *Service) Thread(ctx context.Context, tenantID, rootID uuid.UUID) ([]*Annotation, error) {
	if _, err := authorizeView(ctx); err != nil {
		return nil, err
	}
	items, err := s.repo.Thread(ctx, tenantID, rootID)
	if err != nil {
		return nil, err
	}
	if len(items) > 0 {
		if err := s.checkCaseAccess(ctx, tenantID, items[0].CaseID); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// UpdateBody edits an annotation's Body. Requires identity.PermEditCase
// and that the acting user is the annotation's author (ErrNotAuthor
// otherwise). Re-notifies MentionSink for any new mentions in the
// updated Body.
func (s *Service) UpdateBody(ctx context.Context, tenantID, id uuid.UUID, body string) (*Annotation, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.checkCaseAccess(ctx, tenantID, existing.CaseID); err != nil {
		return nil, err
	}
	if existing.AuthorID != user.ID {
		return nil, ErrNotAuthor
	}
	updated, err := s.repo.UpdateBody(ctx, tenantID, id, body)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AppendAudit(ctx, tenantID, newAuditRecord(updated, AuditEdited, user.ID)); err != nil {
		return nil, err
	}
	s.notifyMentions(ctx, updated)
	return updated, nil
}

// Delete removes an annotation. Requires identity.PermEditCase and
// that the acting user is the annotation's author (ErrNotAuthor
// otherwise).
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return err
	}
	existing, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.checkCaseAccess(ctx, tenantID, existing.CaseID); err != nil {
		return err
	}
	if existing.AuthorID != user.ID {
		return ErrNotAuthor
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	return s.repo.AppendAudit(ctx, tenantID, newAuditRecord(existing, AuditDeleted, user.ID))
}

// Resolve marks an annotation resolved by the acting user. Requires
// identity.PermEditCase. Unlike UpdateBody/Delete, any actor holding
// PermEditCase may resolve/reopen — resolution is a case-workflow
// action, not an authorship one.
func (s *Service) Resolve(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.checkCaseAccess(ctx, tenantID, existing.CaseID); err != nil {
		return nil, err
	}
	updated, err := s.repo.Resolve(ctx, tenantID, id, user.ID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AppendAudit(ctx, tenantID, newAuditRecord(updated, AuditResolved, user.ID)); err != nil {
		return nil, err
	}
	return updated, nil
}

// Reopen clears an annotation's resolved state. Requires
// identity.PermEditCase.
func (s *Service) Reopen(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.checkCaseAccess(ctx, tenantID, existing.CaseID); err != nil {
		return nil, err
	}
	updated, err := s.repo.Reopen(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AppendAudit(ctx, tenantID, newAuditRecord(updated, AuditReopened, user.ID)); err != nil {
		return nil, err
	}
	return updated, nil
}

// MentionsFor returns every Mention recorded for userID within
// tenantID. Requires identity.PermViewCase. Callers typically pass
// their own user ID; this method does not further restrict userID to
// "self", since a supervisor role may legitimately audit another
// user's mentions.
func (s *Service) MentionsFor(ctx context.Context, tenantID, userID uuid.UUID) ([]Mention, error) {
	if _, err := authorizeView(ctx); err != nil {
		return nil, err
	}
	return s.repo.MentionsFor(ctx, tenantID, userID)
}

// AuditTrail returns the audit log for a single annotation. Requires
// identity.PermAuditRead in addition to identity.PermViewCase, since
// audit history is a more sensitive view than the annotation content
// itself.
func (s *Service) AuditTrail(ctx context.Context, tenantID, annotationID uuid.UUID) ([]*AuditRecord, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if !user.HasPermission(identity.PermAuditRead) {
		return nil, ErrForbidden
	}
	return s.repo.ListAudit(ctx, tenantID, annotationID)
}
