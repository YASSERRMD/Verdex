package setup

import (
	"time"

	"github.com/google/uuid"
)

// SetupWizard holds the complete state of a tenant's first-run configuration
// wizard.  Every field is serialised to JSON so the record can be persisted and
// returned by the REST API without additional transformation.
type SetupWizard struct {
	// TenantID is the identifier of the tenant that owns this wizard.
	TenantID uuid.UUID `json:"tenant_id"`

	// State is the current stage of the wizard (see [SetupState]).
	State SetupState `json:"state"`

	// JurisdictionID is set once the tenant selects a jurisdiction.
	JurisdictionID *uuid.UUID `json:"jurisdiction_id,omitempty"`

	// CourtLevel is set once the tenant selects a court tier.
	CourtLevel *string `json:"court_level,omitempty"`

	// Languages contains the reasoning-language codes selected by the tenant.
	Languages []string `json:"languages,omitempty"`

	// ProviderConfig holds the (stub) AI provider configuration.
	ProviderConfig *ProviderConfigStub `json:"provider_config,omitempty"`

	// CreatedAt is the timestamp at which the wizard record was first created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the timestamp of the most recent change.
	UpdatedAt time.Time `json:"updated_at"`

	// CompletedAt is set when the wizard transitions to [StateCompleted].
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// LockedAt is set when the wizard transitions to [StateLocked].
	LockedAt *time.Time `json:"locked_at,omitempty"`
}
