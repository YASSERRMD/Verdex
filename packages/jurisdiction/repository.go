package jurisdiction

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the persistence contract for Jurisdiction records.
// Implementations may store data in a relational database, a document store,
// or any other durable backend.
type Repository interface {
	// Create persists a new Jurisdiction and returns the stored record.
	// Returns ErrDuplicateJurisdiction if a record with the same country code
	// and court name already exists.
	Create(ctx context.Context, j Jurisdiction) (Jurisdiction, error)

	// GetByID retrieves a single Jurisdiction by its UUID primary key.
	// Returns ErrJurisdictionNotFound if no matching record exists.
	GetByID(ctx context.Context, id uuid.UUID) (Jurisdiction, error)

	// GetByCountry returns all Jurisdiction records for the given ISO 3166-1
	// alpha-2 country code.  Returns an empty slice (not an error) when none
	// are found.
	GetByCountry(ctx context.Context, countryCode string) ([]Jurisdiction, error)

	// ListAll returns every Jurisdiction stored in the repository.
	ListAll(ctx context.Context) ([]Jurisdiction, error)

	// Update replaces the stored Jurisdiction identified by j.ID with the
	// provided value.  Returns ErrJurisdictionNotFound if the record does not
	// exist.
	Update(ctx context.Context, j Jurisdiction) (Jurisdiction, error)

	// Delete removes the Jurisdiction identified by id from the repository.
	// Returns ErrJurisdictionNotFound if the record does not exist.
	Delete(ctx context.Context, id uuid.UUID) error
}
