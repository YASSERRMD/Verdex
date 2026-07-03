package casesearch

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SavedSearch is a named Query persisted for a single user, scoped to a
// tenant. See SavedSearchRepository and SavedSearchService.
type SavedSearch struct {
	// ID uniquely identifies this saved search.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this saved search belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// OwnerID is the identity.User who created this saved search. Only
	// the owner (or, per a future admin capability, an actor with
	// broader access) can list/run/delete it — see access.go.
	OwnerID uuid.UUID `json:"owner_id"`

	// Name is a short human-readable label for the saved search.
	Name string `json:"name"`

	// Query is the persisted search request.
	Query Query `json:"query"`

	// CreatedAt is when this saved search was first created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this saved search was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that s has every field required to be persisted: a
// non-nil TenantID and OwnerID, a non-blank Name, and a Query.Mode that
// is either ModeAuto or one of the recognized Mode constants. It does
// not require the full Query.validate() contract (a saved search may
// intentionally be filter-only), so validation here only rejects
// structurally impossible values.
func (s *SavedSearch) Validate() error {
	if s == nil {
		return ErrNilRepository
	}
	if s.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if s.OwnerID == uuid.Nil {
		return ErrUnauthenticated
	}
	if strings.TrimSpace(s.Name) == "" {
		return ErrEmptyName
	}
	if s.Query.Mode != ModeAuto && !s.Query.Mode.IsValid() {
		return ErrInvalidMode
	}
	return nil
}
