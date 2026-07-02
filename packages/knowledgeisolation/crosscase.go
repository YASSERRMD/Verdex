package knowledgeisolation

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// CrossCaseAuthorization is a narrowly-scoped, explicit opt-in token
// granting read access across a fixed set of cases for a legitimate
// cross-case feature (e.g. a future cross-case analytics dashboard, per
// the Phase 048 knowledge-layer API). It is deliberately not reachable
// through CaseScopedStore/CaseScopedVectorStore's normal methods: the
// only way to read across cases is to construct one of these tokens
// explicitly and pass it to CrossCaseReader, so an accidental or
// implicit cross-case read is not possible — a caller has to opt in by
// name.
type CrossCaseAuthorization struct {
	// Cases lists every case this authorization grants read access to.
	// Must be non-empty.
	Cases []CaseID

	// Reason is a short human-readable justification recorded alongside
	// every read performed under this authorization, for audit trails
	// (e.g. "cross-case-analytics-dashboard-q3-review").
	Reason string

	// ExpiresAt bounds how long this authorization is valid. A zero
	// value means "no expiry" (use with caution — prefer always setting
	// a bounded expiry in production callers).
	ExpiresAt time.Time
}

// covers reports whether auth grants access to caseID.
func (auth CrossCaseAuthorization) covers(caseID CaseID) bool {
	for _, c := range auth.Cases {
		if c == caseID {
			return true
		}
	}
	return false
}

// validate checks auth's own shape (non-empty Cases, not expired)
// independent of any particular caseID being requested.
func (auth CrossCaseAuthorization) validate(now time.Time) error {
	if len(auth.Cases) == 0 {
		return ErrMissingAuthorization
	}
	if !auth.ExpiresAt.IsZero() && now.After(auth.ExpiresAt) {
		return ErrAuthorizationExpired
	}
	return nil
}

// CrossCaseReader is the sole entry point in this package for reading
// case-scoped nodes across case boundaries. It wraps a raw
// graph.GraphStore (deliberately not a CaseScopedStore — the whole point
// is to bypass a single case's guard under explicit authorization, not
// to fight it) and requires a valid, unexpired CrossCaseAuthorization
// covering every case named in a request before returning any data.
//
// Every call — authorized or not — is recorded as an AccessAttempt, so
// even legitimate cross-case reads leave an audit trail distinguishing
// them (via Kind and Detail) from a rejected leakage attempt.
type CrossCaseReader struct {
	inner   graph.GraphStore
	auditor *auditRecorder
	clock   func() time.Time
}

// NewCrossCaseReader wraps inner. sink receives every access attempt
// (both authorized reads and rejections); pass nil to use
// NoOpAlertSink. Returns ErrNilStore if inner is nil.
func NewCrossCaseReader(inner graph.GraphStore, sink AlertSink) (*CrossCaseReader, error) {
	if inner == nil {
		return nil, ErrNilStore
	}
	return &CrossCaseReader{
		inner:   inner,
		auditor: newAuditRecorder(sink, nil),
	}, nil
}

// AccessAttempts returns every access attempt (authorized or rejected)
// recorded by this reader so far, in recording order.
func (r *CrossCaseReader) AccessAttempts() []AccessAttempt {
	return r.auditor.snapshot()
}

// GetNodeAcrossCases returns the node with the given id, bypassing
// per-case isolation, provided auth is valid, unexpired, and covers the
// node's owning case (shared-law nodes are always readable and do not
// require the node's own case to be listed in auth.Cases — see
// IsSharedLawNode). Returns ErrMissingAuthorization if auth has no
// cases, ErrAuthorizationExpired if auth has expired, or
// ErrCaseNotAuthorized if the node's case is not covered.
func (r *CrossCaseReader) GetNodeAcrossCases(ctx context.Context, id string, auth CrossCaseAuthorization) (irac.Node, error) {
	if err := auth.validate(r.now()); err != nil {
		r.auditor.record(AccessAttempt{
			Kind:            ViolationCrossCaseAnalysis,
			AuthorizedCases: auth.Cases,
			NodeID:          id,
			Detail:          fmt.Sprintf("GetNodeAcrossCases: invalid authorization: %v (reason=%q)", err, auth.Reason),
		})
		return irac.Node{}, err
	}

	node, err := r.inner.GetNode(ctx, id)
	if err != nil {
		return irac.Node{}, err
	}

	if !IsSharedLawNode(node) && !auth.covers(node.CaseID) {
		r.auditor.record(AccessAttempt{
			Kind:            ViolationCrossCaseAnalysis,
			AuthorizedCases: auth.Cases,
			AttemptedCase:   node.CaseID,
			NodeID:          node.ID,
			Detail:          fmt.Sprintf("GetNodeAcrossCases: node %q belongs to case %q, not covered by authorization (reason=%q)", node.ID, node.CaseID, auth.Reason),
		})
		return irac.Node{}, ErrCaseNotAuthorized
	}

	r.auditor.record(AccessAttempt{
		Kind:            ViolationCrossCaseAnalysis,
		AuthorizedCases: auth.Cases,
		AttemptedCase:   node.CaseID,
		NodeID:          node.ID,
		Detail:          fmt.Sprintf("GetNodeAcrossCases: authorized read (reason=%q)", auth.Reason),
	})
	return node, nil
}

// TraverseAcrossCases returns every node matching query across every
// case listed in auth.Cases (plus any shared-law nodes the traversal
// touches), provided auth is valid and unexpired. Unlike
// CaseScopedStore.Traverse (which is pinned to one case), the caller
// supplies query.CaseID per-case and this method is expected to be
// called once per case in auth.Cases by a cross-case analytics feature;
// it still refuses to serve any case not listed in auth.Cases, so a
// caller cannot smuggle in an unauthorized case through query.CaseID.
func (r *CrossCaseReader) TraverseAcrossCases(ctx context.Context, query graph.TraversalQuery, auth CrossCaseAuthorization) ([]irac.Node, error) {
	if err := auth.validate(r.now()); err != nil {
		r.auditor.record(AccessAttempt{
			Kind:            ViolationCrossCaseAnalysis,
			AuthorizedCases: auth.Cases,
			Detail:          fmt.Sprintf("TraverseAcrossCases: invalid authorization: %v (reason=%q)", err, auth.Reason),
		})
		return nil, err
	}

	if query.CaseID != "" && !auth.covers(query.CaseID) {
		r.auditor.record(AccessAttempt{
			Kind:            ViolationCrossCaseAnalysis,
			AuthorizedCases: auth.Cases,
			AttemptedCase:   query.CaseID,
			Detail:          fmt.Sprintf("TraverseAcrossCases: case %q not covered by authorization (reason=%q)", query.CaseID, auth.Reason),
		})
		return nil, ErrCaseNotAuthorized
	}

	nodes, err := r.inner.Traverse(ctx, query)
	if err != nil {
		return nil, err
	}

	r.auditor.record(AccessAttempt{
		Kind:            ViolationCrossCaseAnalysis,
		AuthorizedCases: auth.Cases,
		AttemptedCase:   query.CaseID,
		Detail:          fmt.Sprintf("TraverseAcrossCases: authorized read of %d node(s) (reason=%q)", len(nodes), auth.Reason),
	})
	return nodes, nil
}

// now returns the reader's clock, defaulting to time.Now.
func (r *CrossCaseReader) now() time.Time {
	if r.clock != nil {
		return r.clock()
	}
	return time.Now()
}
