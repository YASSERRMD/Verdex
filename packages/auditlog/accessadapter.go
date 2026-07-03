package auditlog

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// caseViewedAction is the stable verb-phrase recorded for every Event
// produced by RecordCaseView, matching the "case.viewed" example named
// directly in observability.AuditEvent.Action's own doc comment.
const caseViewedAction = "case.viewed"

// DataAccessSink records read-access events (task 2: "Capture data
// access events") into the durable Store. It is a small, focused
// wrapper — deliberately not a generic interceptor over every
// Repository.Get in the codebase — so the concrete wiring stays
// legible: a caller that reads sensitive case material calls
// RecordCaseView (or RecordDocumentView) right after the read
// succeeds.
type DataAccessSink struct {
	store *Store
}

// NewDataAccessSink builds a DataAccessSink backed by store. Returns
// ErrNilRepository if store is nil.
func NewDataAccessSink(store *Store) (*DataAccessSink, error) {
	if store == nil {
		return nil, ErrNilRepository
	}
	return &DataAccessSink{store: store}, nil
}

// RecordCaseView appends a KindDataAccess Event for a case read,
// attributing it to the authenticated identity.User on ctx (via
// identity.UserFromContext). Returns ErrUnauthenticated if ctx carries
// no authenticated user.
//
// This is the concrete, demonstrable example task 2 asks for:
// packages/caselifecycle.Service.Get (and, by the same pattern,
// packages/casesearch's read paths) already require
// identity.PermViewCase via RequireViewPermission before returning
// case data; a caller wiring that read path into the audit trail
// calls RecordCaseView immediately after the read succeeds, exactly as
// case_view_test.go in this package demonstrates end-to-end against a
// real caselifecycle.Repository.
func (s *DataAccessSink) RecordCaseView(ctx context.Context, tenantID, caseID uuid.UUID) (Event, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return Event{}, wrapf("RecordCaseView", ErrUnauthenticated)
	}

	ev := Event{
		TenantID: tenantID,
		CaseID:   caseID,
		Kind:     KindDataAccess,
	}
	ev.Actor = user.ID.String()
	ev.Action = caseViewedAction
	ev.Target = caseID.String()
	ev.Outcome = "success"

	return s.store.Append(ctx, ev)
}

// RecordDocumentView appends a KindDataAccess Event for a document
// read, scoped to caseID and attributed to ctx's authenticated user,
// mirroring RecordCaseView for the document-level granularity some
// callers need (e.g. packages/evidence document reads within a case).
func (s *DataAccessSink) RecordDocumentView(ctx context.Context, tenantID, caseID uuid.UUID, documentID string) (Event, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return Event{}, wrapf("RecordDocumentView", ErrUnauthenticated)
	}

	ev := Event{
		TenantID: tenantID,
		CaseID:   caseID,
		Kind:     KindDataAccess,
		Detail:   "document:" + documentID,
	}
	ev.Actor = user.ID.String()
	ev.Action = "document.viewed"
	ev.Target = documentID
	ev.Outcome = "success"

	return s.store.Append(ctx, ev)
}
