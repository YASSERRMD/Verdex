package integration

import (
	"context"

	"github.com/google/uuid"
)

// ConfigRepository persists ConnectorConfig records, scoped to a
// tenant on every call, mirroring
// packages/compliance.ControlRepository's conventions.
type ConfigRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, cfg *ConnectorConfig) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ConnectorConfig, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]ConnectorConfig, error)
	Update(ctx context.Context, tenantID uuid.UUID, cfg *ConnectorConfig) error
}

// CredentialsRepository persists ConnectorCredentials records, scoped
// to a tenant.
type CredentialsRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, creds *ConnectorCredentials) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ConnectorCredentials, error)
	Update(ctx context.Context, tenantID uuid.UUID, creds *ConnectorCredentials) error
}

// FieldMappingRepository persists FieldMapping records, scoped to a
// tenant.
type FieldMappingRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, m *FieldMapping) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*FieldMapping, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]FieldMapping, error)
	Update(ctx context.Context, tenantID uuid.UUID, m *FieldMapping) error
}

// ImportRunRepository persists ImportRun records, scoped to a tenant.
type ImportRunRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, run *ImportRun) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportRun, error)
	ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]ImportRun, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]ImportRun, error)
}

// DeliveryRunRepository persists DeliveryRun records, scoped to a
// tenant.
type DeliveryRunRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, run *DeliveryRun) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*DeliveryRun, error)
	ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]DeliveryRun, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]DeliveryRun, error)
}

// ReconciliationRepository persists ReconciliationResult records,
// scoped to a tenant.
type ReconciliationRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, r *ReconciliationResult) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ReconciliationResult, error)
	ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]ReconciliationResult, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]ReconciliationResult, error)
}
