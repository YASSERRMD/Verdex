package casesearch

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// SavedSearchService composes SavedSearchRepository with Engine, gating
// every operation on an authenticated identity.User (authorize,
// access.go) and scoping saved-search ownership to that user: Save
// stamps OwnerID from ctx, List/Delete only ever see/act on the caller's
// own saved searches, and Run executes a saved Query the same way
// Engine.Search would.
type SavedSearchService struct {
	repo   SavedSearchRepository
	engine *Engine
}

// NewSavedSearchService constructs a SavedSearchService over repo and
// engine. Returns ErrNilRepository if repo is nil, or ErrNilResolver if
// engine is nil.
func NewSavedSearchService(repo SavedSearchRepository, engine *Engine) (*SavedSearchService, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	if engine == nil {
		return nil, ErrNilResolver
	}
	return &SavedSearchService{repo: repo, engine: engine}, nil
}

// Save persists a new SavedSearch named name with query q, owned by the
// authenticated user on ctx, scoped to tenantID. Returns
// ErrUnauthenticated/ErrForbidden per authorize, ErrEmptyTenantID if
// tenantID is uuid.Nil, or ErrEmptyName if name is blank.
func (s *SavedSearchService) Save(ctx context.Context, tenantID uuid.UUID, name string, q Query) (*SavedSearch, error) {
	if err := authorize(ctx); err != nil {
		return nil, err
	}
	if tenantID == uuid.Nil {
		return nil, ErrEmptyTenantID
	}
	user, _ := identity.UserFromContext(ctx)

	saved := &SavedSearch{
		TenantID: tenantID,
		OwnerID:  user.ID,
		Name:     name,
		Query:    q,
	}
	if err := s.repo.Create(ctx, tenantID, saved); err != nil {
		return nil, err
	}
	return saved, nil
}

// List returns every saved search owned by the authenticated user on
// ctx, within tenantID.
func (s *SavedSearchService) List(ctx context.Context, tenantID uuid.UUID) ([]*SavedSearch, error) {
	if err := authorize(ctx); err != nil {
		return nil, err
	}
	if tenantID == uuid.Nil {
		return nil, ErrEmptyTenantID
	}
	user, _ := identity.UserFromContext(ctx)

	return s.repo.ListByOwner(ctx, tenantID, user.ID)
}

// Run loads the saved search identified by id (scoped to tenantID and
// the authenticated user's ownership) and executes its Query via
// Engine.Search. Returns ErrNotFound if id does not exist or is not
// owned by the authenticated user.
func (s *SavedSearchService) Run(ctx context.Context, tenantID, id uuid.UUID) (Results, error) {
	if err := authorize(ctx); err != nil {
		return Results{}, err
	}
	if tenantID == uuid.Nil {
		return Results{}, ErrEmptyTenantID
	}
	user, _ := identity.UserFromContext(ctx)

	saved, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return Results{}, err
	}
	if saved.OwnerID != user.ID {
		return Results{}, ErrNotFound
	}

	return s.engine.Search(ctx, tenantID, saved.Query)
}

// Delete removes the saved search identified by id, scoped to tenantID
// and the authenticated user's ownership. Returns ErrNotFound if id does
// not exist or is not owned by the authenticated user.
func (s *SavedSearchService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := authorize(ctx); err != nil {
		return err
	}
	if tenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	user, _ := identity.UserFromContext(ctx)

	saved, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if saved.OwnerID != user.ID {
		return ErrNotFound
	}

	return s.repo.Delete(ctx, tenantID, id)
}
