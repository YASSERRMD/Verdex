package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned by repository Get/Update/Delete methods
// when no row matches the requested id.
var ErrNotFound = errors.New("persistence: not found")

// Tenant is a Verdex customer organization.
type Tenant struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TenantRepository persists Tenant records. Implementations accept an
// Executor per call so callers can compose repository operations
// inside a transaction started by WithTx.
type TenantRepository interface {
	Create(ctx context.Context, exec Executor, t *Tenant) error
	Get(ctx context.Context, exec Executor, id uuid.UUID) (*Tenant, error)
	List(ctx context.Context, exec Executor) ([]*Tenant, error)
	Update(ctx context.Context, exec Executor, t *Tenant) error
	Delete(ctx context.Context, exec Executor, id uuid.UUID) error
}

// PostgresTenantRepository is a PostgreSQL-backed TenantRepository.
type PostgresTenantRepository struct{}

// NewPostgresTenantRepository builds a PostgresTenantRepository.
func NewPostgresTenantRepository() *PostgresTenantRepository {
	return &PostgresTenantRepository{}
}

// Create inserts t, generating an id and timestamps if unset on t,
// and writes the persisted values (including defaults applied by the
// database) back onto t.
func (r *PostgresTenantRepository) Create(ctx context.Context, exec Executor, t *Tenant) error {
	if t == nil {
		return fmt.Errorf("persistence: TenantRepository.Create: t must not be nil")
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	const q = `
		INSERT INTO tenants (id, name, slug)
		VALUES ($1, $2, $3)
		RETURNING id, name, slug, created_at, updated_at`

	row := exec.QueryRow(ctx, q, t.ID, t.Name, t.Slug)
	if err := scanTenant(row, t); err != nil {
		return fmt.Errorf("persistence: TenantRepository.Create: %w", err)
	}
	return nil
}

// Get returns the tenant with the given id, or ErrNotFound if none
// exists.
func (r *PostgresTenantRepository) Get(ctx context.Context, exec Executor, id uuid.UUID) (*Tenant, error) {
	const q = `
		SELECT id, name, slug, created_at, updated_at
		FROM tenants
		WHERE id = $1`

	t := &Tenant{}
	row := exec.QueryRow(ctx, q, id)
	if err := scanTenant(row, t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("persistence: TenantRepository.Get: %w", err)
	}
	return t, nil
}

// List returns every tenant, ordered by creation time.
func (r *PostgresTenantRepository) List(ctx context.Context, exec Executor) ([]*Tenant, error) {
	const q = `
		SELECT id, name, slug, created_at, updated_at
		FROM tenants
		ORDER BY created_at ASC`

	rows, err := exec.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: TenantRepository.List: %w", err)
	}
	defer rows.Close()

	var out []*Tenant
	for rows.Next() {
		t := &Tenant{}
		if err := scanTenant(rows, t); err != nil {
			return nil, fmt.Errorf("persistence: TenantRepository.List: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: TenantRepository.List: %w", err)
	}
	return out, nil
}

// Update overwrites the mutable fields (name, slug) of the tenant
// identified by t.ID, bumps updated_at, and writes the persisted
// values back onto t. It returns ErrNotFound if no row matches t.ID.
func (r *PostgresTenantRepository) Update(ctx context.Context, exec Executor, t *Tenant) error {
	if t == nil {
		return fmt.Errorf("persistence: TenantRepository.Update: t must not be nil")
	}

	const q = `
		UPDATE tenants
		SET name = $2, slug = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, name, slug, created_at, updated_at`

	row := exec.QueryRow(ctx, q, t.ID, t.Name, t.Slug)
	if err := scanTenant(row, t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("persistence: TenantRepository.Update: %w", err)
	}
	return nil
}

// Delete removes the tenant with the given id. It returns ErrNotFound
// if no row matches.
func (r *PostgresTenantRepository) Delete(ctx context.Context, exec Executor, id uuid.UUID) error {
	const q = `DELETE FROM tenants WHERE id = $1`

	tag, err := exec.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("persistence: TenantRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, iterated with Next) so scanTenant can serve both Get/Create/
// Update and List.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTenant(row rowScanner, t *Tenant) error {
	return row.Scan(&t.ID, &t.Name, &t.Slug, &t.CreatedAt, &t.UpdatedAt)
}
