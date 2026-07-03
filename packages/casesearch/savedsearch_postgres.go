package casesearch

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresRepository is a PostgreSQL-backed SavedSearchRepository,
// storing SavedSearch rows in the `saved_searches` table (see
// packages/persistence/migrations/000010_create_saved_searches.up.sql),
// mirroring packages/signoff.PostgresRepository exactly. It accepts a
// persistence.Executor per call, so callers can run it directly against
// a pool or compose it inside a transaction via persistence.WithTx or
// packages/tenancy.WithTenantScope.
type PostgresRepository struct {
	exec persistence.Executor
}

// NewPostgresRepository builds a PostgresRepository bound to exec.
func NewPostgresRepository(exec persistence.Executor) *PostgresRepository {
	return &PostgresRepository{exec: exec}
}

// Create implements SavedSearchRepository.
func (r *PostgresRepository) Create(ctx context.Context, tenantID uuid.UUID, s *SavedSearch) error {
	if s == nil {
		return wrapf("PostgresRepository.Create", ErrNilRepository)
	}
	if s.TenantID == uuid.Nil {
		s.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, s.TenantID); err != nil {
		return err
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}

	queryJSON, err := json.Marshal(s.Query)
	if err != nil {
		return wrapf("PostgresRepository.Create", err)
	}

	const q = `
		INSERT INTO saved_searches (id, tenant_id, owner_id, name, query, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now(), now())
		RETURNING id, tenant_id, owner_id, name, query, created_at, updated_at`

	row := r.exec.QueryRow(ctx, q, s.ID, s.TenantID, s.OwnerID, s.Name, queryJSON)
	if err := scanSavedSearch(row, s); err != nil {
		return wrapf("PostgresRepository.Create", err)
	}
	return nil
}

// Get implements SavedSearchRepository.
func (r *PostgresRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*SavedSearch, error) {
	const q = `
		SELECT id, tenant_id, owner_id, name, query, created_at, updated_at
		FROM saved_searches
		WHERE id = $1 AND tenant_id = $2`

	s := &SavedSearch{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanSavedSearch(row, s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Get", err)
	}
	return s, nil
}

// ListByOwner implements SavedSearchRepository.
func (r *PostgresRepository) ListByOwner(ctx context.Context, tenantID, ownerID uuid.UUID) ([]*SavedSearch, error) {
	const q = `
		SELECT id, tenant_id, owner_id, name, query, created_at, updated_at
		FROM saved_searches
		WHERE tenant_id = $1 AND owner_id = $2
		ORDER BY created_at DESC`

	rows, err := r.exec.Query(ctx, q, tenantID, ownerID)
	if err != nil {
		return nil, wrapf("PostgresRepository.ListByOwner", err)
	}
	defer rows.Close()

	var out []*SavedSearch
	for rows.Next() {
		s := &SavedSearch{}
		if err := scanSavedSearch(rows, s); err != nil {
			return nil, wrapf("PostgresRepository.ListByOwner", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.ListByOwner", err)
	}
	return out, nil
}

// Delete implements SavedSearchRepository.
func (r *PostgresRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `DELETE FROM saved_searches WHERE id = $1 AND tenant_id = $2`

	tag, err := r.exec.Exec(ctx, q, id, tenantID)
	if err != nil {
		return wrapf("PostgresRepository.Delete", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, iterated with Next), mirroring
// packages/signoff.PostgresRepository's rowScanner convention.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSavedSearch(row rowScanner, s *SavedSearch) error {
	var queryJSON []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(&s.ID, &s.TenantID, &s.OwnerID, &s.Name, &queryJSON, &createdAt, &updatedAt); err != nil {
		return err
	}
	var q Query
	if err := json.Unmarshal(queryJSON, &q); err != nil {
		return err
	}
	s.Query = q
	s.CreatedAt = createdAt
	s.UpdatedAt = updatedAt
	return nil
}

var _ SavedSearchRepository = (*PostgresRepository)(nil)
