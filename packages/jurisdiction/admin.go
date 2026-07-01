package jurisdiction

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AdminService provides authoritative CRUD operations over the jurisdiction
// registry, backed by a persistent Repository.
type AdminService struct {
	repo Repository
}

// NewAdminService creates a new AdminService that delegates persistence to repo.
// It panics if repo is nil.
func NewAdminService(repo Repository) *AdminService {
	if repo == nil {
		panic("jurisdiction: AdminService requires a non-nil Repository")
	}
	return &AdminService{repo: repo}
}

// CreateJurisdiction validates j and persists it to the repository.
// The ID field of j is ignored; a new UUID is assigned before saving.
// Returns the persisted Jurisdiction on success.
func (s *AdminService) CreateJurisdiction(ctx context.Context, j Jurisdiction) (Jurisdiction, error) {
	if err := Validate(j); err != nil {
		return Jurisdiction{}, fmt.Errorf("create jurisdiction: %w", err)
	}

	now := time.Now().UTC()
	j.ID = uuid.New()
	j.CreatedAt = now
	j.UpdatedAt = now

	created, err := s.repo.Create(ctx, j)
	if err != nil {
		return Jurisdiction{}, fmt.Errorf("create jurisdiction: %w", err)
	}
	return created, nil
}

// UpdateJurisdiction validates j and overwrites the stored record identified
// by j.ID.  Returns ErrJurisdictionNotFound if no such record exists.
func (s *AdminService) UpdateJurisdiction(ctx context.Context, j Jurisdiction) (Jurisdiction, error) {
	if j.ID == uuid.Nil {
		return Jurisdiction{}, fmt.Errorf("update jurisdiction: %w: ID must not be nil", ErrInvalidJurisdiction)
	}
	if err := Validate(j); err != nil {
		return Jurisdiction{}, fmt.Errorf("update jurisdiction: %w", err)
	}

	j.UpdatedAt = time.Now().UTC()

	updated, err := s.repo.Update(ctx, j)
	if err != nil {
		return Jurisdiction{}, fmt.Errorf("update jurisdiction: %w", err)
	}
	return updated, nil
}

// DeleteJurisdiction removes the jurisdiction identified by id from the
// repository.  Returns ErrJurisdictionNotFound if no such record exists.
func (s *AdminService) DeleteJurisdiction(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("delete jurisdiction: %w: ID must not be nil", ErrInvalidJurisdiction)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete jurisdiction: %w", err)
	}
	return nil
}

// ListJurisdictions returns every jurisdiction currently held in the
// repository.
func (s *AdminService) ListJurisdictions(ctx context.Context) ([]Jurisdiction, error) {
	list, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list jurisdictions: %w", err)
	}
	return list, nil
}
