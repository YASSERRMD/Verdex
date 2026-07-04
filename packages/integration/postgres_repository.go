package integration

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// rowScanner is the subset of pgx.Row / pgx.Rows this file's scan
// helpers depend on, mirroring packages/compliance.rowScanner exactly.
type rowScanner interface {
	Scan(dest ...any) error
}

// PostgresConfigRepository is a PostgreSQL-backed ConfigRepository,
// storing ConnectorConfig rows in the `integration_connector_configs`
// table (see
// packages/persistence/migrations/000034_create_integration.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/compliance.PostgresControlRepository exactly, so callers
// can run it directly against a pool or compose it inside a
// transaction via persistence.WithTx or
// packages/tenancy.WithTenantScope.
type PostgresConfigRepository struct {
	exec persistence.Executor
}

// NewPostgresConfigRepository builds a PostgresConfigRepository bound
// to exec.
func NewPostgresConfigRepository(exec persistence.Executor) *PostgresConfigRepository {
	return &PostgresConfigRepository{exec: exec}
}

const configColumns = `id, tenant_id, connector_type, display_name, endpoint, credentials_id, field_mapping_id, enabled, created_by, created_at, updated_at`

func scanConfig(row rowScanner, cfg *ConnectorConfig) error {
	var credentialsID, fieldMappingID uuid.NullUUID
	if err := row.Scan(
		&cfg.ID, &cfg.TenantID, &cfg.ConnectorType, &cfg.DisplayName, &cfg.Endpoint,
		&credentialsID, &fieldMappingID, &cfg.Enabled, &cfg.CreatedBy, &cfg.CreatedAt, &cfg.UpdatedAt,
	); err != nil {
		return err
	}
	if credentialsID.Valid {
		cfg.CredentialsID = credentialsID.UUID
	}
	if fieldMappingID.Valid {
		cfg.FieldMappingID = fieldMappingID.UUID
	}
	return nil
}

// Create implements ConfigRepository.
func (r *PostgresConfigRepository) Create(ctx context.Context, tenantID uuid.UUID, cfg *ConnectorConfig) error {
	if cfg == nil {
		return ErrInvalidConnectorConfig
	}
	if cfg.TenantID == uuid.Nil {
		cfg.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, cfg.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO integration_connector_configs (id, tenant_id, connector_type, display_name, endpoint, credentials_id, field_mapping_id, enabled, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($11, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + configColumns

	nullCredentialsID := uuid.NullUUID{UUID: cfg.CredentialsID, Valid: cfg.CredentialsID != uuid.Nil}
	nullFieldMappingID := uuid.NullUUID{UUID: cfg.FieldMappingID, Valid: cfg.FieldMappingID != uuid.Nil}
	row := r.exec.QueryRow(ctx, q, cfg.ID, cfg.TenantID, cfg.ConnectorType, cfg.DisplayName, cfg.Endpoint,
		nullCredentialsID, nullFieldMappingID, cfg.Enabled, cfg.CreatedBy, cfg.CreatedAt, cfg.UpdatedAt)
	if err := scanConfig(row, cfg); err != nil {
		return wrapf("PostgresConfigRepository.Create", err)
	}
	return nil
}

// Get implements ConfigRepository.
func (r *PostgresConfigRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ConnectorConfig, error) {
	q := `SELECT ` + configColumns + ` FROM integration_connector_configs WHERE id = $1 AND tenant_id = $2`
	cfg := &ConnectorConfig{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanConfig(row, cfg); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConnectorNotFound
		}
		return nil, wrapf("PostgresConfigRepository.Get", err)
	}
	return cfg, nil
}

// List implements ConfigRepository.
func (r *PostgresConfigRepository) List(ctx context.Context, tenantID uuid.UUID) ([]ConnectorConfig, error) {
	q := `SELECT ` + configColumns + ` FROM integration_connector_configs WHERE tenant_id = $1 ORDER BY created_at ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresConfigRepository.List", err)
	}
	defer rows.Close()

	out := make([]ConnectorConfig, 0)
	for rows.Next() {
		var cfg ConnectorConfig
		if err := scanConfig(rows, &cfg); err != nil {
			return nil, wrapf("PostgresConfigRepository.List", err)
		}
		out = append(out, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresConfigRepository.List", err)
	}
	return out, nil
}

// Update implements ConfigRepository.
func (r *PostgresConfigRepository) Update(ctx context.Context, tenantID uuid.UUID, cfg *ConnectorConfig) error {
	if cfg == nil {
		return ErrInvalidConnectorConfig
	}
	const q = `
		UPDATE integration_connector_configs
		SET display_name = $1, endpoint = $2, credentials_id = $3, field_mapping_id = $4, enabled = $5, updated_at = now()
		WHERE id = $6 AND tenant_id = $7`

	nullCredentialsID := uuid.NullUUID{UUID: cfg.CredentialsID, Valid: cfg.CredentialsID != uuid.Nil}
	nullFieldMappingID := uuid.NullUUID{UUID: cfg.FieldMappingID, Valid: cfg.FieldMappingID != uuid.Nil}
	tag, err := r.exec.Exec(ctx, q, cfg.DisplayName, cfg.Endpoint, nullCredentialsID, nullFieldMappingID, cfg.Enabled, cfg.ID, tenantID)
	if err != nil {
		return wrapf("PostgresConfigRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConnectorNotFound
	}
	return nil
}

var _ ConfigRepository = (*PostgresConfigRepository)(nil)

// PostgresCredentialsRepository is a PostgreSQL-backed
// CredentialsRepository, storing ConnectorCredentials rows in the
// `integration_connector_credentials` table. It never persists raw
// secret material -- only the SecretRef handle (see credentials.go).
type PostgresCredentialsRepository struct {
	exec persistence.Executor
}

// NewPostgresCredentialsRepository builds a
// PostgresCredentialsRepository bound to exec.
func NewPostgresCredentialsRepository(exec persistence.Executor) *PostgresCredentialsRepository {
	return &PostgresCredentialsRepository{exec: exec}
}

const credentialsColumns = `id, tenant_id, kind, secret_ref, client_id, token_url, scopes, last_verified_at, created_by, created_at, updated_at` //nolint:gosec // this is a SQL column list, not a credential value; secret_ref is a handle, never the secret itself

func scanCredentials(row rowScanner, c *ConnectorCredentials) error {
	var scopesJSON []byte
	var lastVerifiedAt *time.Time
	if err := row.Scan(
		&c.ID, &c.TenantID, &c.Kind, &c.SecretRef, &c.ClientID, &c.TokenURL,
		&scopesJSON, &lastVerifiedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return err
	}
	if lastVerifiedAt != nil {
		c.LastVerifiedAt = *lastVerifiedAt
	}
	if len(scopesJSON) > 0 {
		if err := json.Unmarshal(scopesJSON, &c.Scopes); err != nil {
			return err
		}
	}
	return nil
}

// Create implements CredentialsRepository.
func (r *PostgresCredentialsRepository) Create(ctx context.Context, tenantID uuid.UUID, c *ConnectorCredentials) error {
	if c == nil {
		return ErrInvalidCredentials
	}
	if c.TenantID == uuid.Nil {
		c.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, c.TenantID); err != nil {
		return err
	}
	scopesJSON, err := json.Marshal(c.Scopes)
	if err != nil {
		return wrapf("PostgresCredentialsRepository.Create", err)
	}

	q := `
		INSERT INTO integration_connector_credentials (id, tenant_id, kind, secret_ref, client_id, token_url, scopes, last_verified_at, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($11, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + credentialsColumns

	var nullLastVerified *time.Time
	if !c.LastVerifiedAt.IsZero() {
		nullLastVerified = &c.LastVerifiedAt
	}
	row := r.exec.QueryRow(ctx, q, c.ID, c.TenantID, c.Kind, c.SecretRef, c.ClientID, c.TokenURL,
		scopesJSON, nullLastVerified, c.CreatedBy, c.CreatedAt, c.UpdatedAt)
	if err := scanCredentials(row, c); err != nil {
		return wrapf("PostgresCredentialsRepository.Create", err)
	}
	return nil
}

// Get implements CredentialsRepository.
func (r *PostgresCredentialsRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ConnectorCredentials, error) {
	q := `SELECT ` + credentialsColumns + ` FROM integration_connector_credentials WHERE id = $1 AND tenant_id = $2`
	c := &ConnectorCredentials{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanCredentials(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCredentialsNotFound
		}
		return nil, wrapf("PostgresCredentialsRepository.Get", err)
	}
	return c, nil
}

// Update implements CredentialsRepository.
func (r *PostgresCredentialsRepository) Update(ctx context.Context, tenantID uuid.UUID, c *ConnectorCredentials) error {
	if c == nil {
		return ErrInvalidCredentials
	}
	scopesJSON, err := json.Marshal(c.Scopes)
	if err != nil {
		return wrapf("PostgresCredentialsRepository.Update", err)
	}
	const q = `
		UPDATE integration_connector_credentials
		SET kind = $1, secret_ref = $2, client_id = $3, token_url = $4, scopes = $5, last_verified_at = $6, updated_at = now()
		WHERE id = $7 AND tenant_id = $8`

	var nullLastVerified *time.Time
	if !c.LastVerifiedAt.IsZero() {
		nullLastVerified = &c.LastVerifiedAt
	}
	tag, err := r.exec.Exec(ctx, q, c.Kind, c.SecretRef, c.ClientID, c.TokenURL, scopesJSON, nullLastVerified, c.ID, tenantID)
	if err != nil {
		return wrapf("PostgresCredentialsRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCredentialsNotFound
	}
	return nil
}

var _ CredentialsRepository = (*PostgresCredentialsRepository)(nil)

// PostgresFieldMappingRepository is a PostgreSQL-backed
// FieldMappingRepository, storing FieldMapping rows in the
// `integration_field_mappings` table.
type PostgresFieldMappingRepository struct {
	exec persistence.Executor
}

// NewPostgresFieldMappingRepository builds a
// PostgresFieldMappingRepository bound to exec.
func NewPostgresFieldMappingRepository(exec persistence.Executor) *PostgresFieldMappingRepository {
	return &PostgresFieldMappingRepository{exec: exec}
}

const fieldMappingColumns = `id, tenant_id, connector_type, name, rules, created_by, created_at, updated_at`

func scanFieldMapping(row rowScanner, m *FieldMapping) error {
	var rulesJSON []byte
	if err := row.Scan(&m.ID, &m.TenantID, &m.ConnectorType, &m.Name, &rulesJSON, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return err
	}
	if len(rulesJSON) > 0 {
		if err := json.Unmarshal(rulesJSON, &m.Rules); err != nil {
			return err
		}
	}
	return nil
}

// Create implements FieldMappingRepository.
func (r *PostgresFieldMappingRepository) Create(ctx context.Context, tenantID uuid.UUID, m *FieldMapping) error {
	if m == nil {
		return ErrInvalidFieldMapping
	}
	if m.TenantID == uuid.Nil {
		m.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, m.TenantID); err != nil {
		return err
	}
	rulesJSON, err := json.Marshal(m.Rules)
	if err != nil {
		return wrapf("PostgresFieldMappingRepository.Create", err)
	}

	q := `
		INSERT INTO integration_field_mappings (id, tenant_id, connector_type, name, rules, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($8, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + fieldMappingColumns

	row := r.exec.QueryRow(ctx, q, m.ID, m.TenantID, m.ConnectorType, m.Name, rulesJSON, m.CreatedBy, m.CreatedAt, m.UpdatedAt)
	if err := scanFieldMapping(row, m); err != nil {
		return wrapf("PostgresFieldMappingRepository.Create", err)
	}
	return nil
}

// Get implements FieldMappingRepository.
func (r *PostgresFieldMappingRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*FieldMapping, error) {
	q := `SELECT ` + fieldMappingColumns + ` FROM integration_field_mappings WHERE id = $1 AND tenant_id = $2`
	m := &FieldMapping{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanFieldMapping(row, m); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMappingNotFound
		}
		return nil, wrapf("PostgresFieldMappingRepository.Get", err)
	}
	return m, nil
}

// List implements FieldMappingRepository.
func (r *PostgresFieldMappingRepository) List(ctx context.Context, tenantID uuid.UUID) ([]FieldMapping, error) {
	q := `SELECT ` + fieldMappingColumns + ` FROM integration_field_mappings WHERE tenant_id = $1 ORDER BY created_at ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresFieldMappingRepository.List", err)
	}
	defer rows.Close()

	out := make([]FieldMapping, 0)
	for rows.Next() {
		var m FieldMapping
		if err := scanFieldMapping(rows, &m); err != nil {
			return nil, wrapf("PostgresFieldMappingRepository.List", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresFieldMappingRepository.List", err)
	}
	return out, nil
}

// Update implements FieldMappingRepository.
func (r *PostgresFieldMappingRepository) Update(ctx context.Context, tenantID uuid.UUID, m *FieldMapping) error {
	if m == nil {
		return ErrInvalidFieldMapping
	}
	rulesJSON, err := json.Marshal(m.Rules)
	if err != nil {
		return wrapf("PostgresFieldMappingRepository.Update", err)
	}
	const q = `
		UPDATE integration_field_mappings
		SET name = $1, rules = $2, updated_at = now()
		WHERE id = $3 AND tenant_id = $4`

	tag, err := r.exec.Exec(ctx, q, m.Name, rulesJSON, m.ID, tenantID)
	if err != nil {
		return wrapf("PostgresFieldMappingRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMappingNotFound
	}
	return nil
}

var _ FieldMappingRepository = (*PostgresFieldMappingRepository)(nil)

// PostgresImportRunRepository is a PostgreSQL-backed
// ImportRunRepository, storing ImportRun rows in the
// `integration_import_runs` table. ImportRun/DeliveryRun/
// ReconciliationResult are append-only history: there is deliberately
// no Update method on this repository or its interface.
type PostgresImportRunRepository struct {
	exec persistence.Executor
}

// NewPostgresImportRunRepository builds a PostgresImportRunRepository
// bound to exec.
func NewPostgresImportRunRepository(exec persistence.Executor) *PostgresImportRunRepository {
	return &PostgresImportRunRepository{exec: exec}
}

const importRunColumns = `id, tenant_id, connector_config_id, since, status, imported_count, mapped_count, failed_external_ids, imported_external_ids, error_message, started_at, finished_at, triggered_by`

func scanImportRun(row rowScanner, run *ImportRun) error {
	var since *time.Time
	var failedJSON, importedJSON []byte
	if err := row.Scan(
		&run.ID, &run.TenantID, &run.ConnectorConfigID, &since, &run.Status,
		&run.ImportedCount, &run.MappedCount, &failedJSON, &importedJSON,
		&run.ErrorMessage, &run.StartedAt, &run.FinishedAt, &run.TriggeredBy,
	); err != nil {
		return err
	}
	if since != nil {
		run.Since = *since
	}
	if len(failedJSON) > 0 {
		if err := json.Unmarshal(failedJSON, &run.FailedExternalIDs); err != nil {
			return err
		}
	}
	if len(importedJSON) > 0 {
		if err := json.Unmarshal(importedJSON, &run.ImportedExternalIDs); err != nil {
			return err
		}
	}
	return nil
}

// Create implements ImportRunRepository.
func (r *PostgresImportRunRepository) Create(ctx context.Context, tenantID uuid.UUID, run *ImportRun) error {
	if run == nil {
		return ErrInvalidImportRun
	}
	if run.TenantID == uuid.Nil {
		run.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, run.TenantID); err != nil {
		return err
	}
	failedJSON, err := json.Marshal(run.FailedExternalIDs)
	if err != nil {
		return wrapf("PostgresImportRunRepository.Create", err)
	}
	importedJSON, err := json.Marshal(run.ImportedExternalIDs)
	if err != nil {
		return wrapf("PostgresImportRunRepository.Create", err)
	}

	q := `
		INSERT INTO integration_import_runs (id, tenant_id, connector_config_id, since, status, imported_count, mapped_count, failed_external_ids, imported_external_ids, error_message, started_at, finished_at, triggered_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING ` + importRunColumns

	var nullSince *time.Time
	if !run.Since.IsZero() {
		nullSince = &run.Since
	}
	row := r.exec.QueryRow(ctx, q, run.ID, run.TenantID, run.ConnectorConfigID, nullSince, run.Status,
		run.ImportedCount, run.MappedCount, failedJSON, importedJSON, run.ErrorMessage,
		run.StartedAt, run.FinishedAt, run.TriggeredBy)
	if err := scanImportRun(row, run); err != nil {
		return wrapf("PostgresImportRunRepository.Create", err)
	}
	return nil
}

// Get implements ImportRunRepository.
func (r *PostgresImportRunRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportRun, error) {
	q := `SELECT ` + importRunColumns + ` FROM integration_import_runs WHERE id = $1 AND tenant_id = $2`
	run := &ImportRun{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanImportRun(row, run); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrImportRunNotFound
		}
		return nil, wrapf("PostgresImportRunRepository.Get", err)
	}
	return run, nil
}

// ListForConnector implements ImportRunRepository.
func (r *PostgresImportRunRepository) ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]ImportRun, error) {
	q := `SELECT ` + importRunColumns + ` FROM integration_import_runs WHERE tenant_id = $1 AND connector_config_id = $2 ORDER BY started_at ASC`
	return r.queryImportRuns(ctx, q, tenantID, connectorConfigID)
}

// ListAll implements ImportRunRepository.
func (r *PostgresImportRunRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ImportRun, error) {
	q := `SELECT ` + importRunColumns + ` FROM integration_import_runs WHERE tenant_id = $1 ORDER BY started_at ASC`
	return r.queryImportRuns(ctx, q, tenantID)
}

func (r *PostgresImportRunRepository) queryImportRuns(ctx context.Context, q string, args ...any) ([]ImportRun, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresImportRunRepository.query", err)
	}
	defer rows.Close()

	out := make([]ImportRun, 0)
	for rows.Next() {
		var run ImportRun
		if err := scanImportRun(rows, &run); err != nil {
			return nil, wrapf("PostgresImportRunRepository.query", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresImportRunRepository.query", err)
	}
	return out, nil
}

var _ ImportRunRepository = (*PostgresImportRunRepository)(nil)

// PostgresDeliveryRunRepository is a PostgreSQL-backed
// DeliveryRunRepository, storing DeliveryRun rows in the
// `integration_delivery_runs` table.
type PostgresDeliveryRunRepository struct {
	exec persistence.Executor
}

// NewPostgresDeliveryRunRepository builds a
// PostgresDeliveryRunRepository bound to exec.
func NewPostgresDeliveryRunRepository(exec persistence.Executor) *PostgresDeliveryRunRepository {
	return &PostgresDeliveryRunRepository{exec: exec}
}

const deliveryRunColumns = `id, tenant_id, connector_config_id, case_external_id, report_kind, status, external_receipt_id, detail, attempt_count, started_at, finished_at, triggered_by`

func scanDeliveryRun(row rowScanner, run *DeliveryRun) error {
	return row.Scan(
		&run.ID, &run.TenantID, &run.ConnectorConfigID, &run.CaseExternalID, &run.ReportKind,
		&run.Status, &run.ExternalReceiptID, &run.Detail, &run.AttemptCount,
		&run.StartedAt, &run.FinishedAt, &run.TriggeredBy,
	)
}

// Create implements DeliveryRunRepository.
func (r *PostgresDeliveryRunRepository) Create(ctx context.Context, tenantID uuid.UUID, run *DeliveryRun) error {
	if run == nil {
		return ErrInvalidDeliveryRun
	}
	if run.TenantID == uuid.Nil {
		run.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, run.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO integration_delivery_runs (id, tenant_id, connector_config_id, case_external_id, report_kind, status, external_receipt_id, detail, attempt_count, started_at, finished_at, triggered_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING ` + deliveryRunColumns

	row := r.exec.QueryRow(ctx, q, run.ID, run.TenantID, run.ConnectorConfigID, run.CaseExternalID, run.ReportKind,
		run.Status, run.ExternalReceiptID, run.Detail, run.AttemptCount, run.StartedAt, run.FinishedAt, run.TriggeredBy)
	if err := scanDeliveryRun(row, run); err != nil {
		return wrapf("PostgresDeliveryRunRepository.Create", err)
	}
	return nil
}

// Get implements DeliveryRunRepository.
func (r *PostgresDeliveryRunRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*DeliveryRun, error) {
	q := `SELECT ` + deliveryRunColumns + ` FROM integration_delivery_runs WHERE id = $1 AND tenant_id = $2`
	run := &DeliveryRun{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanDeliveryRun(row, run); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeliveryRunNotFound
		}
		return nil, wrapf("PostgresDeliveryRunRepository.Get", err)
	}
	return run, nil
}

// ListForConnector implements DeliveryRunRepository.
func (r *PostgresDeliveryRunRepository) ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]DeliveryRun, error) {
	q := `SELECT ` + deliveryRunColumns + ` FROM integration_delivery_runs WHERE tenant_id = $1 AND connector_config_id = $2 ORDER BY started_at ASC`
	return r.queryDeliveryRuns(ctx, q, tenantID, connectorConfigID)
}

// ListAll implements DeliveryRunRepository.
func (r *PostgresDeliveryRunRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]DeliveryRun, error) {
	q := `SELECT ` + deliveryRunColumns + ` FROM integration_delivery_runs WHERE tenant_id = $1 ORDER BY started_at ASC`
	return r.queryDeliveryRuns(ctx, q, tenantID)
}

func (r *PostgresDeliveryRunRepository) queryDeliveryRuns(ctx context.Context, q string, args ...any) ([]DeliveryRun, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresDeliveryRunRepository.query", err)
	}
	defer rows.Close()

	out := make([]DeliveryRun, 0)
	for rows.Next() {
		var run DeliveryRun
		if err := scanDeliveryRun(rows, &run); err != nil {
			return nil, wrapf("PostgresDeliveryRunRepository.query", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresDeliveryRunRepository.query", err)
	}
	return out, nil
}

var _ DeliveryRunRepository = (*PostgresDeliveryRunRepository)(nil)

// PostgresReconciliationRepository is a PostgreSQL-backed
// ReconciliationRepository, storing ReconciliationResult rows in the
// `integration_reconciliation_results` table.
type PostgresReconciliationRepository struct {
	exec persistence.Executor
}

// NewPostgresReconciliationRepository builds a
// PostgresReconciliationRepository bound to exec.
func NewPostgresReconciliationRepository(exec persistence.Executor) *PostgresReconciliationRepository {
	return &PostgresReconciliationRepository{exec: exec}
}

const reconciliationColumns = `id, tenant_id, connector_config_id, kind, expected_count, observed_count, missing_external_ids, unexpected_external_ids, ran_at, ran_by`

func scanReconciliation(row rowScanner, res *ReconciliationResult) error {
	var missingJSON, unexpectedJSON []byte
	if err := row.Scan(
		&res.ID, &res.TenantID, &res.ConnectorConfigID, &res.Kind, &res.ExpectedCount, &res.ObservedCount,
		&missingJSON, &unexpectedJSON, &res.RanAt, &res.RanBy,
	); err != nil {
		return err
	}
	if len(missingJSON) > 0 {
		if err := json.Unmarshal(missingJSON, &res.MissingExternalIDs); err != nil {
			return err
		}
	}
	if len(unexpectedJSON) > 0 {
		if err := json.Unmarshal(unexpectedJSON, &res.UnexpectedExternalIDs); err != nil {
			return err
		}
	}
	return nil
}

// Create implements ReconciliationRepository.
func (r *PostgresReconciliationRepository) Create(ctx context.Context, tenantID uuid.UUID, res *ReconciliationResult) error {
	if res == nil {
		return ErrInvalidReconciliation
	}
	if res.TenantID == uuid.Nil {
		res.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, res.TenantID); err != nil {
		return err
	}
	missingJSON, err := json.Marshal(res.MissingExternalIDs)
	if err != nil {
		return wrapf("PostgresReconciliationRepository.Create", err)
	}
	unexpectedJSON, err := json.Marshal(res.UnexpectedExternalIDs)
	if err != nil {
		return wrapf("PostgresReconciliationRepository.Create", err)
	}

	q := `
		INSERT INTO integration_reconciliation_results (id, tenant_id, connector_config_id, kind, expected_count, observed_count, missing_external_ids, unexpected_external_ids, ran_at, ran_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + reconciliationColumns

	row := r.exec.QueryRow(ctx, q, res.ID, res.TenantID, res.ConnectorConfigID, res.Kind, res.ExpectedCount, res.ObservedCount,
		missingJSON, unexpectedJSON, res.RanAt, res.RanBy)
	if err := scanReconciliation(row, res); err != nil {
		return wrapf("PostgresReconciliationRepository.Create", err)
	}
	return nil
}

// Get implements ReconciliationRepository.
func (r *PostgresReconciliationRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ReconciliationResult, error) {
	q := `SELECT ` + reconciliationColumns + ` FROM integration_reconciliation_results WHERE id = $1 AND tenant_id = $2`
	res := &ReconciliationResult{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanReconciliation(row, res); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReconciliationNotFound
		}
		return nil, wrapf("PostgresReconciliationRepository.Get", err)
	}
	return res, nil
}

// ListForConnector implements ReconciliationRepository.
func (r *PostgresReconciliationRepository) ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]ReconciliationResult, error) {
	q := `SELECT ` + reconciliationColumns + ` FROM integration_reconciliation_results WHERE tenant_id = $1 AND connector_config_id = $2 ORDER BY ran_at ASC`
	return r.queryReconciliations(ctx, q, tenantID, connectorConfigID)
}

// ListAll implements ReconciliationRepository.
func (r *PostgresReconciliationRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ReconciliationResult, error) {
	q := `SELECT ` + reconciliationColumns + ` FROM integration_reconciliation_results WHERE tenant_id = $1 ORDER BY ran_at ASC`
	return r.queryReconciliations(ctx, q, tenantID)
}

func (r *PostgresReconciliationRepository) queryReconciliations(ctx context.Context, q string, args ...any) ([]ReconciliationResult, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresReconciliationRepository.query", err)
	}
	defer rows.Close()

	out := make([]ReconciliationResult, 0)
	for rows.Next() {
		var res ReconciliationResult
		if err := scanReconciliation(rows, &res); err != nil {
			return nil, wrapf("PostgresReconciliationRepository.query", err)
		}
		out = append(out, res)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresReconciliationRepository.query", err)
	}
	return out, nil
}

var _ ReconciliationRepository = (*PostgresReconciliationRepository)(nil)
