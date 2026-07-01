package setup

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SetupService orchestrates the setup wizard lifecycle for tenants.
//
// All methods are safe for concurrent use provided the underlying [Repository]
// implementation is also safe for concurrent use.
type SetupService struct {
	repo Repository
}

// NewSetupService returns a new [SetupService] backed by the given repository.
func NewSetupService(repo Repository) *SetupService {
	if repo == nil {
		panic("setup: NewSetupService: repo must not be nil")
	}
	return &SetupService{repo: repo}
}

// StartSetup creates (or retrieves) the wizard for tenantID and advances it
// from [StatePending] to [StateInProgress].
//
// Calling StartSetup on a wizard that is already past [StatePending] is
// idempotent — the existing wizard is returned unchanged.
// Returns [ErrSetupLocked] if the wizard is already locked.
func (s *SetupService) StartSetup(ctx context.Context, tenantID uuid.UUID) (*SetupWizard, error) {
	w, _, err := GetOrCreate(ctx, s.repo, tenantID)
	if err != nil {
		return nil, fmt.Errorf("setup: StartSetup: %w", err)
	}

	if err := EnsureNotLocked(w); err != nil {
		return nil, err
	}

	// Already started — return as-is.
	if w.State != StatePending {
		return w, nil
	}

	now := time.Now().UTC()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	w.UpdatedAt = now

	if err := transition(w, StateInProgress); err != nil {
		return nil, fmt.Errorf("setup: StartSetup: %w", err)
	}

	if err := s.repo.Update(ctx, w); err != nil {
		return nil, fmt.Errorf("setup: StartSetup: %w", err)
	}
	return w, nil
}

// StepFn is a function that mutates a [SetupWizard] in place, advancing it
// through one step.  Step functions in steps.go satisfy this signature.
type StepFn func(w *SetupWizard) error

// ApplyStep fetches the wizard for tenantID, calls stepFn against it, and
// persists the result.
//
// Returns [ErrSetupNotFound] if no wizard exists for the tenant.
// Returns [ErrSetupLocked] if the wizard is already in [StateLocked].
func (s *SetupService) ApplyStep(ctx context.Context, tenantID uuid.UUID, stepFn StepFn) (*SetupWizard, error) {
	w, err := s.repo.GetByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("setup: ApplyStep: %w", err)
	}

	if err := EnsureNotLocked(w); err != nil {
		return nil, err
	}

	if err := stepFn(w); err != nil {
		return nil, fmt.Errorf("setup: ApplyStep: %w", err)
	}

	if err := s.repo.Update(ctx, w); err != nil {
		return nil, fmt.Errorf("setup: ApplyStep: %w", err)
	}
	return w, nil
}

// GetStatus returns the current wizard for tenantID.
// Returns [ErrSetupNotFound] if no wizard has been started.
func (s *SetupService) GetStatus(ctx context.Context, tenantID uuid.UUID) (*SetupWizard, error) {
	w, err := s.repo.GetByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("setup: GetStatus: %w", err)
	}
	return w, nil
}

// IsComplete returns true when the wizard for tenantID has reached
// [StateCompleted] or [StateLocked].
// Returns [ErrSetupNotFound] if no wizard exists.
func (s *SetupService) IsComplete(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	w, err := s.repo.GetByTenant(ctx, tenantID)
	if err != nil {
		return false, fmt.Errorf("setup: IsComplete: %w", err)
	}
	return w.State == StateCompleted || w.State == StateLocked, nil
}
