package setup

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Repository defines the persistence operations required by [SetupService].
// Implementations may use any storage back-end (PostgreSQL, in-memory, etc.).
type Repository interface {
	// Create persists a new wizard record.  Returns an error if a record for
	// the same tenant already exists.
	Create(ctx context.Context, w *SetupWizard) error

	// GetByTenant retrieves the wizard for the given tenant.
	// Returns [ErrSetupNotFound] when no record exists.
	GetByTenant(ctx context.Context, tenantID uuid.UUID) (*SetupWizard, error)

	// Update overwrites an existing wizard record with the supplied value.
	// Returns [ErrSetupNotFound] when the record does not exist.
	Update(ctx context.Context, w *SetupWizard) error
}

// GetOrCreate is an idempotency helper that fetches an existing wizard for the
// tenant, or creates a new one in [StatePending] if none exists.
//
// The returned bool is true when a new record was created.
func GetOrCreate(ctx context.Context, repo Repository, tenantID uuid.UUID) (*SetupWizard, bool, error) {
	w, err := repo.GetByTenant(ctx, tenantID)
	if err == nil {
		return w, false, nil
	}
	if !errors.Is(err, ErrSetupNotFound) {
		return nil, false, err
	}

	// No existing record — create one.
	w = &SetupWizard{
		TenantID: tenantID,
		State:    StatePending,
	}
	if createErr := repo.Create(ctx, w); createErr != nil {
		return nil, false, createErr
	}
	return w, true, nil
}

// EnsureNotLocked is a guard helper that returns [ErrSetupLocked] when the
// wizard is in [StateLocked].  Use it at the top of service methods to provide
// a clear error before attempting any state mutation.
func EnsureNotLocked(w *SetupWizard) error {
	if w.State == StateLocked {
		return ErrSetupLocked
	}
	return nil
}

// EnsureNotComplete is a guard that returns [ErrSetupAlreadyComplete] when the
// wizard has already reached [StateCompleted] or beyond.
func EnsureNotComplete(w *SetupWizard) error {
	switch w.State {
	case StateCompleted, StateLocked:
		return ErrSetupAlreadyComplete
	}
	return nil
}
