package localization

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// InMemoryPreferenceRepository is a process-local PreferenceRepository
// backed by a map guarded by a mutex, intended for tests and other
// packages' fixtures -- never for production use, mirroring
// packages/privacy.InMemoryInventoryRepository's role exactly.
type InMemoryPreferenceRepository struct {
	mu    sync.RWMutex
	byKey map[preferenceKey]*Preference
}

// preferenceKey is the (tenant, user) composite key a Preference is
// upserted/looked-up by.
type preferenceKey struct {
	tenantID uuid.UUID
	userID   uuid.UUID
}

// NewInMemoryPreferenceRepository builds an empty
// InMemoryPreferenceRepository.
func NewInMemoryPreferenceRepository() *InMemoryPreferenceRepository {
	return &InMemoryPreferenceRepository{byKey: make(map[preferenceKey]*Preference)}
}

// Upsert implements PreferenceRepository. Timestamps
// (CreatedAt/UpdatedAt) are expected to already be set by the caller
// (Engine.SetPreference stamps them via its own clock, mirroring
// packages/compliance.Engine.SetProfile's convention) -- this
// repository only preserves a stable ID across repeat upserts for the
// same (tenant, user) pair, it does not independently manage time.
func (r *InMemoryPreferenceRepository) Upsert(_ context.Context, tenantID uuid.UUID, p *Preference) error {
	if p == nil {
		return ErrInvalidPreference
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	key := preferenceKey{tenantID: tenantID, userID: p.UserID}
	if existing, ok := r.byKey[key]; ok {
		p.ID = existing.ID
	} else if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	cp := *p
	r.byKey[key] = &cp
	return nil
}

// Get implements PreferenceRepository.
func (r *InMemoryPreferenceRepository) Get(_ context.Context, tenantID, userID uuid.UUID) (*Preference, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byKey[preferenceKey{tenantID: tenantID, userID: userID}]
	if !ok {
		return nil, ErrPreferenceNotFound
	}
	cp := *p
	return &cp, nil
}

// Delete implements PreferenceRepository.
func (r *InMemoryPreferenceRepository) Delete(_ context.Context, tenantID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := preferenceKey{tenantID: tenantID, userID: userID}
	if _, ok := r.byKey[key]; !ok {
		return ErrPreferenceNotFound
	}
	delete(r.byKey, key)
	return nil
}

var _ PreferenceRepository = (*InMemoryPreferenceRepository)(nil)

// PostgresPreferenceRepository is a PostgreSQL-backed
// PreferenceRepository, storing Preference rows in the
// `localization_preferences` table (see
// packages/persistence/migrations/000036_create_localization.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/privacy.PostgresInventoryRepository exactly, so callers can
// run it directly against a pool or compose it inside a transaction
// via persistence.WithTx or packages/tenancy.WithTenantScope (see
// TenantScopedPreferenceRepository).
type PostgresPreferenceRepository struct {
	exec persistence.Executor
}

// NewPostgresPreferenceRepository builds a PostgresPreferenceRepository
// bound to exec.
func NewPostgresPreferenceRepository(exec persistence.Executor) *PostgresPreferenceRepository {
	return &PostgresPreferenceRepository{exec: exec}
}

const preferenceColumns = `id, tenant_id, user_id, locale, created_at, updated_at`

// rowScanner is the subset of pgx.Row / pgx.Rows this file's scan
// helper depends on, mirroring
// packages/privacy.rowScanner exactly.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanPreference(row rowScanner, p *Preference) error {
	return row.Scan(&p.ID, &p.TenantID, &p.UserID, &p.Locale, &p.CreatedAt, &p.UpdatedAt)
}

// Upsert implements PreferenceRepository. A user has at most one
// Preference row per tenant, so this issues an
// INSERT ... ON CONFLICT (tenant_id, user_id) DO UPDATE rather than
// requiring the caller to know whether a row already exists.
//
// Timestamps are expected to already be set by the caller
// (Engine.SetPreference stamps them via its own clock, mirroring
// packages/compliance.Engine.SetProfile's convention); a caller that
// leaves them zero (e.g. a direct repository-level test) gets
// now()-defaulted values via the same
// COALESCE(NULLIF($n, TIMESTAMPTZ '0001-01-01'), now()) idiom
// packages/privacy.PostgresInventoryRepository.Create already uses.
func (r *PostgresPreferenceRepository) Upsert(ctx context.Context, tenantID uuid.UUID, p *Preference) error {
	if p == nil {
		return ErrInvalidPreference
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	q := `
		INSERT INTO localization_preferences (id, tenant_id, user_id, locale, created_at, updated_at)
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($6, TIMESTAMPTZ '0001-01-01'), now()))
		ON CONFLICT (tenant_id, user_id) DO UPDATE
			SET locale = EXCLUDED.locale, updated_at = COALESCE(NULLIF($6, TIMESTAMPTZ '0001-01-01'), now())
		RETURNING ` + preferenceColumns

	row := r.exec.QueryRow(ctx, q, p.ID, p.TenantID, p.UserID, string(p.Locale), p.CreatedAt, p.UpdatedAt)
	if err := scanPreference(row, p); err != nil {
		return wrapf("PostgresPreferenceRepository.Upsert", err)
	}
	return nil
}

// Get implements PreferenceRepository.
func (r *PostgresPreferenceRepository) Get(ctx context.Context, tenantID, userID uuid.UUID) (*Preference, error) {
	q := `SELECT ` + preferenceColumns + ` FROM localization_preferences WHERE tenant_id = $1 AND user_id = $2`
	p := &Preference{}
	row := r.exec.QueryRow(ctx, q, tenantID, userID)
	if err := scanPreference(row, p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPreferenceNotFound
		}
		return nil, wrapf("PostgresPreferenceRepository.Get", err)
	}
	return p, nil
}

// Delete implements PreferenceRepository.
func (r *PostgresPreferenceRepository) Delete(ctx context.Context, tenantID, userID uuid.UUID) error {
	q := `DELETE FROM localization_preferences WHERE tenant_id = $1 AND user_id = $2`
	tag, err := r.exec.Exec(ctx, q, tenantID, userID)
	if err != nil {
		return wrapf("PostgresPreferenceRepository.Delete", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPreferenceNotFound
	}
	return nil
}

var _ PreferenceRepository = (*PostgresPreferenceRepository)(nil)
