package integration

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ConnectorConfig is a tenant's registered configuration for one
// external court case-management system connection: which Connector
// implementation to use, tenant-specific settings, and a reference to
// its ConnectorCredentials. ConnectorConfig itself carries no secret
// material -- see credentials.go.
type ConnectorConfig struct {
	// ID uniquely identifies this configuration.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this configuration to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// ConnectorType names the registered Connector implementation this
	// configuration binds to (the ID a Registry.Get call resolves,
	// e.g. "efiling-dubai-courts", "sandbox") -- distinct from ID,
	// which is this configuration record's own primary key.
	ConnectorType string `json:"connector_type"`

	// DisplayName is a short human-readable label for this connection
	// (e.g. "Dubai Courts - Civil Division").
	DisplayName string `json:"display_name"`

	// Endpoint is the external system's base URL or address, if
	// applicable to the connector type. Never a credential itself.
	Endpoint string `json:"endpoint,omitempty"`

	// CredentialsID references the ConnectorCredentials record used to
	// authenticate calls made through this configuration. May be
	// uuid.Nil for a connector that requires no credentials (e.g. the
	// sandbox connector).
	CredentialsID uuid.UUID `json:"credentials_id,omitempty"`

	// FieldMappingID references the FieldMapping this configuration
	// uses to translate between this platform's fields and the
	// external system's schema. May be uuid.Nil if the connector needs
	// no mapping (e.g. it already speaks this platform's field names).
	FieldMappingID uuid.UUID `json:"field_mapping_id,omitempty"`

	// Enabled reports whether this configuration is currently active.
	// A disabled configuration is retained (for audit/history) but
	// Engine methods refuse to use it for new import/delivery
	// attempts.
	Enabled bool `json:"enabled"`

	// CreatedBy is the identity.User who registered this configuration.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks cfg for structural well-formedness.
func (cfg *ConnectorConfig) Validate() error {
	if cfg == nil {
		return ErrInvalidConnectorConfig
	}
	if cfg.TenantID == uuid.Nil {
		return wrapf("ConnectorConfig.Validate", ErrEmptyTenantID)
	}
	if strings.TrimSpace(cfg.ConnectorType) == "" {
		return wrapf("ConnectorConfig.Validate", ErrInvalidConnectorConfig)
	}
	if strings.TrimSpace(cfg.DisplayName) == "" {
		return wrapf("ConnectorConfig.Validate", ErrInvalidConnectorConfig)
	}
	return nil
}
