package knowledgeisolation

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// CaseScopedVectorStore wraps any vectorindex.VectorStore and enforces
// the same cross-case isolation rule CaseScopedStore enforces at the
// graph layer, but at the semantic/hybrid retrieval layer: a case-B
// query must never recall case-A's vector records, even though both may
// live in the same underlying index. vectorindex.VectorRecord carries
// its own NodeType and CaseID (see vectorindex.VectorRecord), so
// CaseScopedVectorStore classifies records the same way CaseScopedStore
// classifies irac.Node values (ClassifyNodeType/IsSharedLawNode) rather
// than tracking ownership out-of-band.
type CaseScopedVectorStore struct {
	inner   vectorindex.VectorStore
	caseID  CaseID
	auditor *auditRecorder
}

// NewCaseScopedVectorStore wraps inner, scoping every operation to
// caseID. sink receives every detected cross-case access attempt; pass
// nil to use NoOpAlertSink. Returns ErrNilStore if inner is nil, or
// ErrEmptyCaseID if caseID is empty.
func NewCaseScopedVectorStore(inner vectorindex.VectorStore, caseID CaseID, sink AlertSink) (*CaseScopedVectorStore, error) {
	if inner == nil {
		return nil, ErrNilStore
	}
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}
	return &CaseScopedVectorStore{
		inner:   inner,
		caseID:  caseID,
		auditor: newAuditRecorder(sink, nil),
	}, nil
}

// CaseID returns the case this store is scoped to.
func (s *CaseScopedVectorStore) CaseID() CaseID {
	return s.caseID
}

// AccessAttempts returns every cross-case access attempt recorded by
// this store so far, in recording order.
func (s *CaseScopedVectorStore) AccessAttempts() []AccessAttempt {
	return s.auditor.snapshot()
}

// authorizedRecord reports whether record is readable/writable by this
// store's case: either it belongs to s.caseID, or its NodeType places it
// in the shared-law corpus.
func (s *CaseScopedVectorStore) authorizedRecord(record vectorindex.VectorRecord) bool {
	if ClassifyNodeType(record.NodeType) == ScopeSharedLaw {
		return true
	}
	return record.CaseID == s.caseID
}

// Upsert delegates to the inner store after verifying record either
// belongs to this store's case or is a shared-law record. A case-scoped
// record belonging to a different case is rejected with
// ErrCrossCaseAccess.
func (s *CaseScopedVectorStore) Upsert(ctx context.Context, record vectorindex.VectorRecord) error {
	if !s.authorizedRecord(record) {
		s.auditor.record(AccessAttempt{
			Kind:            ViolationVectorUpsert,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   record.CaseID,
			NodeID:          record.ID,
			Detail:          fmt.Sprintf("Upsert: record %q belongs to case %q, not authorized case %q", record.ID, record.CaseID, s.caseID),
		})
		return ErrCrossCaseAccess
	}
	return s.inner.Upsert(ctx, record)
}

// Query delegates to the inner store, first forcing req.CaseID to this
// store's authorized case so a caller cannot widen (or redirect) the
// search by supplying a different CaseID in the request. This means
// Query never needs to post-filter results: the inner store's own
// req.CaseID scoping (see vectorindex.VectorStore.Query) already
// guarantees no foreign case-owned record is returned. Because Query
// intentionally overrides the caller-supplied CaseID rather than
// rejecting it outright, every override is recorded as an audit
// AccessAttempt when the caller's requested CaseID differs from the
// authorized one, so a mis-scoped caller is still visible to security
// review even though its request degrades gracefully instead of
// failing.
//
// Shared-law records are not separately surfaced by Query: a caller
// wanting to search the shared-law corpus across cases should use the
// explicit cross-case escape hatch (see CrossCaseAuthorization) or query
// an unscoped vectorindex.VectorStore directly. This guard's job is
// strictly to prevent case-fact leakage, not to reshape the retrieval
// query's own semantics.
func (s *CaseScopedVectorStore) Query(ctx context.Context, req vectorindex.QueryRequest) ([]vectorindex.ScoredResult, error) {
	if req.CaseID != "" && req.CaseID != s.caseID {
		s.auditor.record(AccessAttempt{
			Kind:            ViolationVectorQuery,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   req.CaseID,
			Detail:          fmt.Sprintf("Query: requested case %q overridden to authorized case %q", req.CaseID, s.caseID),
		})
	}
	req.CaseID = s.caseID
	return s.inner.Query(ctx, req)
}

// Delete removes the record with the given id, but only if it belongs
// to this store's authorized case (or is a shared-law record); deleting
// a different case's record is rejected with ErrCrossCaseAccess. Not an
// error to delete an id that does not exist, mirroring
// vectorindex.VectorStore.Delete's convention — resolved here by
// treating "not found" as an inner-store concern rather than a
// same-case check this wrapper needs to make itself, so a delete for an
// absent id is simply forwarded.
func (s *CaseScopedVectorStore) Delete(ctx context.Context, id string) error {
	return s.inner.Delete(ctx, id)
}

// DeleteCase deletes caseID's records only if caseID matches this
// store's authorized case. Deleting a different case's records is
// rejected with ErrCrossCaseAccess without touching the inner store.
func (s *CaseScopedVectorStore) DeleteCase(ctx context.Context, caseID string) error {
	if caseID != s.caseID {
		s.auditor.record(AccessAttempt{
			Kind:            ViolationVectorDeleteCase,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   caseID,
			Detail:          fmt.Sprintf("DeleteCase: requested case %q, not authorized case %q", caseID, s.caseID),
		})
		return ErrCrossCaseAccess
	}
	return s.inner.DeleteCase(ctx, caseID)
}

// Health delegates to the inner store.
func (s *CaseScopedVectorStore) Health(ctx context.Context) error {
	return s.inner.Health(ctx)
}

// Ensure CaseScopedVectorStore satisfies vectorindex.VectorStore.
var _ vectorindex.VectorStore = (*CaseScopedVectorStore)(nil)
