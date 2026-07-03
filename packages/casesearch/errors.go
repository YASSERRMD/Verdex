package casesearch

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyTenantID is returned when a Search or saved-search operation
	// is called with a zero tenant ID.
	ErrEmptyTenantID = errors.New("casesearch: tenant id is required")

	// ErrEmptyQuery is returned by Search when a Query carries none of
	// Text, IssueOrRuleID, or any structured filter — an entirely empty
	// query would degrade to "list every case", which this package
	// requires callers to request explicitly via an all-filters-empty
	// Query only in ModeKeyword/ModeAuto with an explicit acknowledgement
	// that Text is intentionally blank (see Query.AllowEmptyText).
	ErrEmptyQuery = errors.New("casesearch: query must specify text, an issue/rule id, or at least one filter")

	// ErrInvalidMode is returned when Query.Mode is set to a value other
	// than the recognized Mode constants.
	ErrInvalidMode = errors.New("casesearch: invalid search mode")

	// ErrNilRepository is returned by constructors that require a non-nil
	// caselifecycle.Repository or SavedSearchRepository.
	ErrNilRepository = errors.New("casesearch: repository must not be nil")

	// ErrNilResolver is returned by NewEngine when constructed with a nil
	// CaseSearcherResolver.
	ErrNilResolver = errors.New("casesearch: case searcher resolver must not be nil")

	// ErrUnauthenticated is returned when an operation requiring an actor
	// is called with a context carrying no authenticated identity.User.
	ErrUnauthenticated = errors.New("casesearch: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks
	// identity.PermViewCase.
	ErrForbidden = errors.New("casesearch: actor lacks required permission")

	// ErrNotFound is returned by SavedSearchRepository.Get/Delete when no
	// saved search matches the requested ID (or the tenant/owner scope
	// hides it).
	ErrNotFound = errors.New("casesearch: saved search not found")

	// ErrEmptyName is returned when SaveSearch is called with a blank
	// name.
	ErrEmptyName = errors.New("casesearch: saved search name is required")

	// ErrCrossTenantAccess is returned by SavedSearchRepository methods
	// when asked to operate on a SavedSearch whose TenantID does not
	// match the scope's tenantID, mirroring
	// packages/caselifecycle.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("casesearch: cross-tenant access denied")

	// ErrInvalidPage is returned when Query.Page carries a negative
	// Number or Size.
	ErrInvalidPage = errors.New("casesearch: invalid page request")

	// ErrNilEmbedFunc is returned by KnowledgeAPISearcher.SearchSemantic
	// when constructed without an EmbedFunc.
	ErrNilEmbedFunc = errors.New("casesearch: semantic search requires a configured embed function")

	// ErrNoSearcher is returned internally when a CaseSearcherResolver
	// yields neither a searcher nor an error for a candidate case; Search
	// treats this the same as a resolver error (skip the case).
	ErrNoSearcher = errors.New("casesearch: resolver returned no searcher")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("casesearch: %s: %w", fn, err)
}

// requireMatchingTenant returns ErrCrossTenantAccess if entityTenantID is
// set and does not equal scopeTenantID, mirroring
// packages/signoff's and packages/caselifecycle's unexported helper of
// the same name and behavior. A nil entityTenantID (the zero uuid.UUID)
// is treated as "not yet assigned" and is not an error here.
func requireMatchingTenant(scopeTenantID, entityTenantID uuid.UUID) error {
	if entityTenantID != uuid.Nil && entityTenantID != scopeTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}
