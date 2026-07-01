package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Deployment statuses allowed by the deployments_status_allowed
// database constraint (see migrations/000002_create_deployments.up.sql).
const (
	DeploymentStatusProvisioning   = "provisioning"
	DeploymentStatusActive         = "active"
	DeploymentStatusSuspended      = "suspended"
	DeploymentStatusDecommissioned = "decommissioned"
)

// Deployment is a single provisioned instance of Verdex for a tenant.
// Jurisdiction assignment is owned by Phase 007 and is not modeled
// here yet.
type Deployment struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Profile   string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DeploymentRepository persists Deployment records. Implementations
// accept an Executor per call so callers can compose repository
// operations inside a transaction started by WithTx.
type DeploymentRepository interface {
	Create(ctx context.Context, exec Executor, d *Deployment) error
	Get(ctx context.Context, exec Executor, id uuid.UUID) (*Deployment, error)
	List(ctx context.Context, exec Executor) ([]*Deployment, error)
	Update(ctx context.Context, exec Executor, d *Deployment) error
	Delete(ctx context.Context, exec Executor, id uuid.UUID) error
}

// PostgresDeploymentRepository is a PostgreSQL-backed
// DeploymentRepository.
type PostgresDeploymentRepository struct{}

// NewPostgresDeploymentRepository builds a
// PostgresDeploymentRepository.
func NewPostgresDeploymentRepository() *PostgresDeploymentRepository {
	return &PostgresDeploymentRepository{}
}

// Create inserts d, generating an id if unset and defaulting Status
// to DeploymentStatusProvisioning if unset, and writes the persisted
// values back onto d.
func (r *PostgresDeploymentRepository) Create(ctx context.Context, exec Executor, d *Deployment) error {
	if d == nil {
		return fmt.Errorf("persistence: DeploymentRepository.Create: d must not be nil")
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.Status == "" {
		d.Status = DeploymentStatusProvisioning
	}

	const q = `
		INSERT INTO deployments (id, tenant_id, profile, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, profile, status, created_at, updated_at`

	row := exec.QueryRow(ctx, q, d.ID, d.TenantID, d.Profile, d.Status)
	if err := scanDeployment(row, d); err != nil {
		return fmt.Errorf("persistence: DeploymentRepository.Create: %w", err)
	}
	return nil
}

// Get returns the deployment with the given id, or ErrNotFound if
// none exists.
func (r *PostgresDeploymentRepository) Get(ctx context.Context, exec Executor, id uuid.UUID) (*Deployment, error) {
	const q = `
		SELECT id, tenant_id, profile, status, created_at, updated_at
		FROM deployments
		WHERE id = $1`

	d := &Deployment{}
	row := exec.QueryRow(ctx, q, id)
	if err := scanDeployment(row, d); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("persistence: DeploymentRepository.Get: %w", err)
	}
	return d, nil
}

// List returns every deployment, ordered by creation time.
func (r *PostgresDeploymentRepository) List(ctx context.Context, exec Executor) ([]*Deployment, error) {
	const q = `
		SELECT id, tenant_id, profile, status, created_at, updated_at
		FROM deployments
		ORDER BY created_at ASC`

	rows, err := exec.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: DeploymentRepository.List: %w", err)
	}
	defer rows.Close()

	var out []*Deployment
	for rows.Next() {
		d := &Deployment{}
		if err := scanDeployment(rows, d); err != nil {
			return nil, fmt.Errorf("persistence: DeploymentRepository.List: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: DeploymentRepository.List: %w", err)
	}
	return out, nil
}

// Update overwrites the mutable fields (profile, status) of the
// deployment identified by d.ID, bumps updated_at, and writes the
// persisted values back onto d. It returns ErrNotFound if no row
// matches d.ID.
func (r *PostgresDeploymentRepository) Update(ctx context.Context, exec Executor, d *Deployment) error {
	if d == nil {
		return fmt.Errorf("persistence: DeploymentRepository.Update: d must not be nil")
	}

	const q = `
		UPDATE deployments
		SET profile = $2, status = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, tenant_id, profile, status, created_at, updated_at`

	row := exec.QueryRow(ctx, q, d.ID, d.Profile, d.Status)
	if err := scanDeployment(row, d); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("persistence: DeploymentRepository.Update: %w", err)
	}
	return nil
}

// Delete removes the deployment with the given id. It returns
// ErrNotFound if no row matches.
func (r *PostgresDeploymentRepository) Delete(ctx context.Context, exec Executor, id uuid.UUID) error {
	const q = `DELETE FROM deployments WHERE id = $1`

	tag, err := exec.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("persistence: DeploymentRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanDeployment(row rowScanner, d *Deployment) error {
	return row.Scan(&d.ID, &d.TenantID, &d.Profile, &d.Status, &d.CreatedAt, &d.UpdatedAt)
}
