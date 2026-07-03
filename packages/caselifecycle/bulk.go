package caselifecycle

import (
	"context"

	"github.com/google/uuid"
)

// BulkResult is the per-case outcome of a bulk operation (BulkTransition
// or BulkSetMetadata). Bulk operations in this package are
// partial-failure-safe by default: one case failing (not found,
// cross-tenant, illegal transition, invalid metadata) does not abort
// or roll back the operation for any other case in the batch. Callers
// that need all-or-nothing semantics should wrap their own call in a
// transaction at the Repository implementation level; this package
// does not provide that as the default because a single illegal
// transition among 200 requested bulk-closures should not block the
// other 199 legal ones.
type BulkResult struct {
	// CaseID identifies which case this result is for.
	CaseID uuid.UUID

	// Case is the updated case on success, nil on failure.
	Case *Case

	// Err is nil on success, or the error that caused this specific
	// case to fail.
	Err error
}

// Succeeded reports whether this case's operation completed without
// error.
func (r BulkResult) Succeeded() bool { return r.Err == nil }

// BulkTransitionInput bundles the arguments for BulkTransition.
type BulkTransitionInput struct {
	// TenantID scopes every case in the batch.
	TenantID uuid.UUID

	// CaseIDs is the set of cases to transition.
	CaseIDs []uuid.UUID

	// ToState is the requested destination state, applied identically
	// to every case in CaseIDs.
	ToState State

	// Reason optionally explains the transitions; recorded on every
	// resulting TransitionRecord.
	Reason string
}

// BulkTransition calls Transition once per input.CaseIDs entry and
// returns one BulkResult per case, in the same order as
// input.CaseIDs. It never returns a top-level error for an individual
// case's failure — see BulkResult's doc comment for the
// partial-failure contract. A top-level error is only returned for a
// nil repo or an unauthenticated ctx, since neither of those is a
// per-case condition.
func BulkTransition(ctx context.Context, repo Repository, input BulkTransitionInput) ([]BulkResult, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}

	results := make([]BulkResult, 0, len(input.CaseIDs))
	for _, caseID := range input.CaseIDs {
		c, err := Transition(ctx, repo, TransitionInput{
			TenantID: input.TenantID,
			CaseID:   caseID,
			ToState:  input.ToState,
			Reason:   input.Reason,
		})
		results = append(results, BulkResult{CaseID: caseID, Case: c, Err: err})
	}
	return results, nil
}

// BulkMetadataUpdateInput bundles the arguments for BulkSetMetadata
// and BulkMergeMetadata.
type BulkMetadataUpdateInput struct {
	// TenantID scopes every case in the batch.
	TenantID uuid.UUID

	// CaseIDs is the set of cases to update.
	CaseIDs []uuid.UUID

	// Values is the metadata to write or merge into every case in
	// CaseIDs.
	Values map[string]string
}

// BulkSetMetadata calls SetMetadata once per input.CaseIDs entry and
// returns one BulkResult per case, in the same order as
// input.CaseIDs, following the same partial-failure contract as
// BulkTransition.
func BulkSetMetadata(ctx context.Context, repo Repository, input BulkMetadataUpdateInput) ([]BulkResult, error) {
	return bulkMetadataUpdate(ctx, repo, input, false)
}

// BulkMergeMetadata calls MergeMetadata once per input.CaseIDs entry
// and returns one BulkResult per case, in the same order as
// input.CaseIDs, following the same partial-failure contract as
// BulkTransition.
func BulkMergeMetadata(ctx context.Context, repo Repository, input BulkMetadataUpdateInput) ([]BulkResult, error) {
	return bulkMetadataUpdate(ctx, repo, input, true)
}

func bulkMetadataUpdate(ctx context.Context, repo Repository, input BulkMetadataUpdateInput, merge bool) ([]BulkResult, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}

	results := make([]BulkResult, 0, len(input.CaseIDs))
	for _, caseID := range input.CaseIDs {
		updateInput := MetadataUpdateInput{
			TenantID: input.TenantID,
			CaseID:   caseID,
			Values:   input.Values,
		}
		var c *Case
		var err error
		if merge {
			c, err = MergeMetadata(ctx, repo, updateInput)
		} else {
			c, err = SetMetadata(ctx, repo, updateInput)
		}
		results = append(results, BulkResult{CaseID: caseID, Case: c, Err: err})
	}
	return results, nil
}

// SucceededCaseIDs returns the CaseIDs of every result that succeeded.
func SucceededCaseIDs(results []BulkResult) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(results))
	for _, r := range results {
		if r.Succeeded() {
			out = append(out, r.CaseID)
		}
	}
	return out
}

// FailedResults returns every result that failed.
func FailedResults(results []BulkResult) []BulkResult {
	out := make([]BulkResult, 0, len(results))
	for _, r := range results {
		if !r.Succeeded() {
			out = append(out, r)
		}
	}
	return out
}
