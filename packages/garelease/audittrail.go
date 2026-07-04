package garelease

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// AuditTrailStore is the subset of packages/auditlog.Store's public
// surface VerifyAuditTrail depends on: query a tenant's events and
// verify its hash chain. A real *auditlog.Store satisfies this
// interface directly (see var _ AuditTrailStore assertion below); the
// small interface exists so this package's audit-completeness check is
// expressed against a Store-SHAPED dependency, following exactly the
// resolver-interface convention packages/pilot's QualityScoreLike and
// packages/vulnmanagement's doc.go both establish for decoupling from a
// concrete heavy type while still composing with the real one in
// production.
type AuditTrailStore interface {
	// Query returns events for tenantID matching filter -- used here to
	// confirm the store is genuinely queryable (task 6's "queryable"
	// half), not merely constructed.
	Query(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error)

	// VerifyTenantChain recomputes and checks tenantID's full hash
	// chain -- used here to confirm the store's tamper-evidence
	// guarantee actually holds (task 6's "complete"/tamper-evident
	// half), not merely assumed.
	VerifyTenantChain(ctx context.Context, tenantID uuid.UUID) (valid bool, brokenAt int, err error)
}

var _ AuditTrailStore = (*auditlog.Store)(nil)

// AuditCheckResult is the outcome of a single audit-trail assertion
// VerifyAuditTrail performed, mirroring GuardrailCheckResult's shape.
type AuditCheckResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// AuditVerification is the result of Engine.VerifyAuditTrail: every
// individual AuditCheckResult plus an overall Passed aggregation.
type AuditVerification struct {
	Results     []AuditCheckResult `json:"results"`
	Passed      bool               `json:"passed"`
	EvaluatedAt time.Time          `json:"evaluated_at"`
}

// Failures returns the subset of v.Results that did not pass.
func (v AuditVerification) Failures() []AuditCheckResult {
	out := make([]AuditCheckResult, 0)
	for _, r := range v.Results {
		if !r.Passed {
			out = append(out, r)
		}
	}
	return out
}

// VerifyAuditTrail is task 6's audit half: a REAL structural check
// against an AuditTrailStore-shaped dependency for representativeTenantID
// (a real tenant this deployment already has an audit history for, or
// any tenant ID a caller wants to spot-check as representative of the
// whole store's health -- this package has no tenant registry of its
// own to iterate every tenant, since ReleaseCandidate/Release are
// platform-global, not tenant-scoped; see doc.go).
//
// Two assertions run, both must hold for AuditVerification.Passed:
//
//  1. store.Query succeeds (the audit store is genuinely queryable, not
//     merely constructed) -- a query error is a hard failure, an empty
//     result set is not (a freshly-provisioned tenant with no audit
//     history yet is not itself a broken audit trail).
//  2. store.VerifyTenantChain reports valid=true (the store's
//     hash-chain tamper-evidence guarantee -- packages/auditlog's core
//     promise from Phase 077 -- actually holds for this tenant's
//     history, not merely assumed).
func (e *Engine) VerifyAuditTrail(ctx context.Context, store AuditTrailStore, representativeTenantID uuid.UUID) (AuditVerification, error) {
	now := e.now()
	if store == nil {
		return AuditVerification{}, ErrNilAuditStore
	}
	if representativeTenantID == uuid.Nil {
		return AuditVerification{}, wrapf("VerifyAuditTrail", ErrEmptyRepresentativeTenantID)
	}

	results := make([]AuditCheckResult, 0, 2)

	events, err := store.Query(ctx, representativeTenantID, auditlog.Filter{})
	if err != nil {
		results = append(results, AuditCheckResult{
			Name:   "audit_store_queryable",
			Passed: false,
			Detail: "audit store query failed: " + err.Error(),
		})
	} else {
		results = append(results, AuditCheckResult{
			Name:   "audit_store_queryable",
			Passed: true,
			Detail: fmt.Sprintf("audit store returned %d event(s) for the representative tenant", len(events)),
		})
	}

	valid, brokenAt, chainErr := store.VerifyTenantChain(ctx, representativeTenantID)
	switch {
	case chainErr != nil:
		results = append(results, AuditCheckResult{
			Name:   "audit_chain_intact",
			Passed: false,
			Detail: "hash-chain verification failed: " + chainErr.Error(),
		})
	case !valid:
		results = append(results, AuditCheckResult{
			Name:   "audit_chain_intact",
			Passed: false,
			Detail: fmt.Sprintf("hash chain broken at event index %d", brokenAt),
		})
	default:
		results = append(results, AuditCheckResult{
			Name:   "audit_chain_intact",
			Passed: true,
			Detail: "hash chain verified intact",
		})
	}

	passed := true
	for _, r := range results {
		if !r.Passed {
			passed = false
			break
		}
	}

	return AuditVerification{Results: results, Passed: passed, EvaluatedAt: now}, nil
}

// checkAuditCompleteness adapts VerifyAuditTrail into a ReadinessCheck
// for DimensionAuditCompleteness, called from CheckReadiness using the
// Engine's own configured auditStore and representativeTenantID.
func (e *Engine) checkAuditCompleteness(ctx context.Context, now time.Time) (ReadinessCheck, error) {
	verification, err := e.VerifyAuditTrail(ctx, e.auditStore, e.representativeTenantID)
	if err != nil {
		return ReadinessCheck{}, wrapf("checkAuditCompleteness", err)
	}

	if verification.Passed {
		return ReadinessCheck{
			Dimension:   DimensionAuditCompleteness,
			Status:      CheckPassed,
			Detail:      fmt.Sprintf("all %d audit-trail assertions held", len(verification.Results)),
			EvaluatedAt: now,
		}, nil
	}

	failures := verification.Failures()
	detail := fmt.Sprintf("%d of %d audit-trail assertions failed:", len(failures), len(verification.Results))
	for _, f := range failures {
		detail = fmt.Sprintf("%s %s", detail, f.Name)
	}
	return ReadinessCheck{
		Dimension:   DimensionAuditCompleteness,
		Status:      CheckFailed,
		Detail:      detail,
		EvaluatedAt: now,
	}, nil
}
