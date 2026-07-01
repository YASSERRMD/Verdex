package tenancy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// Provisioning record outcomes allowed by the
// deployment_provisioning_records_outcome_allowed database constraint
// (see packages/persistence/migrations/000004_create_deployment_provisioning_records.up.sql).
const (
	ProvisioningOutcomeStarted   = "started"
	ProvisioningOutcomeSucceeded = "succeeded"
	ProvisioningOutcomeFailed    = "failed"
)

// ProvisioningRecord captures a single attempt at provisioning a
// persistence.Deployment: when it started, when (if ever) it
// completed, its outcome, and any error detail. A Deployment
// accumulates one ProvisioningRecord per provisioning attempt, so the
// history of retries/failures survives even after the Deployment
// itself reaches DeploymentStatusActive.
//
// This type lives in packages/tenancy rather than packages/persistence
// as a sibling of Deployment because provisioning attempts are a
// tenancy/deployment-lifecycle concern layered on top of the storage
// entity, not a new storage primitive peer to Tenant/Deployment
// themselves; packages/persistence's existing entities are intentionally
// left untouched by this phase.
type ProvisioningRecord struct {
	ID           uuid.UUID
	DeploymentID uuid.UUID
	Outcome      string
	ErrorDetail  *string
	StartedAt    time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
}

// ProvisioningRecordRepository persists ProvisioningRecord entries.
// Implementations accept a persistence.Executor per call so callers
// can compose calls inside a transaction started by persistence.WithTx
// or tenancy.WithTenantScope.
type ProvisioningRecordRepository interface {
	Create(ctx context.Context, exec persistence.Executor, r *ProvisioningRecord) error
	Get(ctx context.Context, exec persistence.Executor, id uuid.UUID) (*ProvisioningRecord, error)
	ListByDeployment(ctx context.Context, exec persistence.Executor, deploymentID uuid.UUID) ([]*ProvisioningRecord, error)
	Complete(ctx context.Context, exec persistence.Executor, id uuid.UUID, outcome string, errorDetail *string) (*ProvisioningRecord, error)
}

// PostgresProvisioningRecordRepository is a PostgreSQL-backed
// ProvisioningRecordRepository.
type PostgresProvisioningRecordRepository struct{}

// NewPostgresProvisioningRecordRepository builds a
// PostgresProvisioningRecordRepository.
func NewPostgresProvisioningRecordRepository() *PostgresProvisioningRecordRepository {
	return &PostgresProvisioningRecordRepository{}
}

// Create inserts r, generating an id and defaulting Outcome to
// ProvisioningOutcomeStarted if unset, and writes the persisted values
// back onto r.
func (repo *PostgresProvisioningRecordRepository) Create(ctx context.Context, exec persistence.Executor, r *ProvisioningRecord) error {
	if r == nil {
		return fmt.Errorf("tenancy: ProvisioningRecordRepository.Create: r must not be nil")
	}
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.Outcome == "" {
		r.Outcome = ProvisioningOutcomeStarted
	}

	const q = `
		INSERT INTO deployment_provisioning_records (id, deployment_id, outcome, error_detail)
		VALUES ($1, $2, $3, $4)
		RETURNING id, deployment_id, outcome, error_detail, started_at, completed_at, created_at`

	row := exec.QueryRow(ctx, q, r.ID, r.DeploymentID, r.Outcome, r.ErrorDetail)
	if err := scanProvisioningRecord(row, r); err != nil {
		return fmt.Errorf("tenancy: ProvisioningRecordRepository.Create: %w", err)
	}
	return nil
}

// Get returns the provisioning record with the given id, or
// persistence.ErrNotFound if none exists.
func (repo *PostgresProvisioningRecordRepository) Get(ctx context.Context, exec persistence.Executor, id uuid.UUID) (*ProvisioningRecord, error) {
	const q = `
		SELECT id, deployment_id, outcome, error_detail, started_at, completed_at, created_at
		FROM deployment_provisioning_records
		WHERE id = $1`

	r := &ProvisioningRecord{}
	row := exec.QueryRow(ctx, q, id)
	if err := scanProvisioningRecord(row, r); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, persistence.ErrNotFound
		}
		return nil, fmt.Errorf("tenancy: ProvisioningRecordRepository.Get: %w", err)
	}
	return r, nil
}

// ListByDeployment returns every provisioning record for deploymentID,
// ordered by started_at ascending (oldest attempt first).
func (repo *PostgresProvisioningRecordRepository) ListByDeployment(ctx context.Context, exec persistence.Executor, deploymentID uuid.UUID) ([]*ProvisioningRecord, error) {
	const q = `
		SELECT id, deployment_id, outcome, error_detail, started_at, completed_at, created_at
		FROM deployment_provisioning_records
		WHERE deployment_id = $1
		ORDER BY started_at ASC`

	rows, err := exec.Query(ctx, q, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("tenancy: ProvisioningRecordRepository.ListByDeployment: %w", err)
	}
	defer rows.Close()

	var out []*ProvisioningRecord
	for rows.Next() {
		r := &ProvisioningRecord{}
		if err := scanProvisioningRecord(rows, r); err != nil {
			return nil, fmt.Errorf("tenancy: ProvisioningRecordRepository.ListByDeployment: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tenancy: ProvisioningRecordRepository.ListByDeployment: %w", err)
	}
	return out, nil
}

// Complete marks the provisioning record identified by id as finished:
// it sets completed_at to now(), and records outcome/errorDetail. It
// returns persistence.ErrNotFound if no row matches id.
func (repo *PostgresProvisioningRecordRepository) Complete(ctx context.Context, exec persistence.Executor, id uuid.UUID, outcome string, errorDetail *string) (*ProvisioningRecord, error) {
	const q = `
		UPDATE deployment_provisioning_records
		SET outcome = $2, error_detail = $3, completed_at = now()
		WHERE id = $1
		RETURNING id, deployment_id, outcome, error_detail, started_at, completed_at, created_at`

	r := &ProvisioningRecord{}
	row := exec.QueryRow(ctx, q, id, outcome, errorDetail)
	if err := scanProvisioningRecord(row, r); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, persistence.ErrNotFound
		}
		return nil, fmt.Errorf("tenancy: ProvisioningRecordRepository.Complete: %w", err)
	}
	return r, nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, iterated with Next), mirroring
// packages/persistence/tenant.go's rowScanner so scanProvisioningRecord
// can serve both single-row and multi-row query paths.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanProvisioningRecord(row rowScanner, r *ProvisioningRecord) error {
	return row.Scan(&r.ID, &r.DeploymentID, &r.Outcome, &r.ErrorDetail, &r.StartedAt, &r.CompletedAt, &r.CreatedAt)
}
