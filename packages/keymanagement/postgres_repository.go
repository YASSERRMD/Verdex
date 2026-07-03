package keymanagement

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresRepository is a PostgreSQL-backed Repository, storing
// KeyMetadata rows in the `key_metadata` table (see
// packages/persistence/migrations/000018_create_keymanagement.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/notifications.PostgresRepository exactly, so callers can
// run it directly against a pool or compose it inside a transaction
// via persistence.WithTx or packages/tenancy.WithTenantScope.
type PostgresRepository struct {
	exec persistence.Executor
}

// NewPostgresRepository builds a PostgresRepository bound to exec.
func NewPostgresRepository(exec persistence.Executor) *PostgresRepository {
	return &PostgresRepository{exec: exec}
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, iterated with Next).
type rowScanner interface {
	Scan(dest ...any) error
}

func scanKeyMetadata(row rowScanner, m *KeyMetadata) error {
	var state string
	if err := row.Scan(
		&m.ID, &m.TenantID, &m.Version, &state, &m.CreatedAt, &m.ExpiresAt, &m.WrappedKeyRef,
	); err != nil {
		return err
	}
	m.State = KeyState(state)
	return nil
}

// Create implements Repository.
func (r *PostgresRepository) Create(ctx context.Context, tenantID uuid.UUID, m *KeyMetadata) error {
	if m == nil {
		return wrapf("PostgresRepository.Create", ErrNilKeyMetadata)
	}
	if m.TenantID == uuid.Nil {
		m.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, m.TenantID); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}

	const q = `
		INSERT INTO key_metadata (id, tenant_id, version, state, created_at, expires_at, wrapped_key_ref)
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, TIMESTAMPTZ '0001-01-01'), now()), $6, $7)
		RETURNING id, tenant_id, version, state, created_at, expires_at, wrapped_key_ref`

	row := r.exec.QueryRow(ctx, q,
		m.ID, m.TenantID, m.Version, string(m.State), m.CreatedAt, m.ExpiresAt, m.WrappedKeyRef,
	)
	if err := scanKeyMetadata(row, m); err != nil {
		// A unique-constraint violation here means the tenant already
		// has an Active key — enforced at the database layer by
		// migrations/000018_create_keymanagement.up.sql's partial
		// unique index, as defense-in-depth alongside the
		// application-level check in InMemoryRepository.Create /
		// Service.Rotate. The raw pgx/Postgres error is wrapped as-is
		// rather than reclassified, since Repository's contract only
		// promises ErrNotFound/ErrCrossTenantAccess/ErrInvalidKeyState
		// for known conditions this package itself detects first.
		return wrapf("PostgresRepository.Create", err)
	}
	return nil
}

// Get implements Repository.
func (r *PostgresRepository) Get(ctx context.Context, tenantID uuid.UUID, id string) (*KeyMetadata, error) {
	const q = `
		SELECT id, tenant_id, version, state, created_at, expires_at, wrapped_key_ref
		FROM key_metadata
		WHERE id = $1 AND tenant_id = $2`

	m := &KeyMetadata{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanKeyMetadata(row, m); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Get", err)
	}
	return m, nil
}

// GetActive implements Repository.
func (r *PostgresRepository) GetActive(ctx context.Context, tenantID uuid.UUID) (*KeyMetadata, error) {
	const q = `
		SELECT id, tenant_id, version, state, created_at, expires_at, wrapped_key_ref
		FROM key_metadata
		WHERE tenant_id = $1 AND state = 'active'`

	m := &KeyMetadata{}
	row := r.exec.QueryRow(ctx, q, tenantID)
	if err := scanKeyMetadata(row, m); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoActiveKey
		}
		return nil, wrapf("PostgresRepository.GetActive", err)
	}
	return m, nil
}

// ListForTenant implements Repository.
func (r *PostgresRepository) ListForTenant(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]*KeyMetadata, error) {
	q := `
		SELECT id, tenant_id, version, state, created_at, expires_at, wrapped_key_ref
		FROM key_metadata
		WHERE tenant_id = $1`
	args := []any{tenantID}

	if filter.State != "" {
		args = append(args, string(filter.State))
		q += " AND state = $" + strconv.Itoa(len(args))
	}
	q += " ORDER BY version DESC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		q += " LIMIT $" + strconv.Itoa(len(args))
	}

	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRepository.ListForTenant", err)
	}
	defer rows.Close()

	out := make([]*KeyMetadata, 0)
	for rows.Next() {
		m := &KeyMetadata{}
		if err := scanKeyMetadata(rows, m); err != nil {
			return nil, wrapf("PostgresRepository.ListForTenant", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.ListForTenant", err)
	}
	return out, nil
}

// UpdateState implements Repository.
func (r *PostgresRepository) UpdateState(ctx context.Context, tenantID uuid.UUID, id string, newState KeyState) error {
	if !newState.IsValid() {
		return ErrInvalidKeyState
	}

	const q = `
		UPDATE key_metadata SET state = $1
		WHERE id = $2 AND tenant_id = $3`

	tag, err := r.exec.Exec(ctx, q, string(newState), id, tenantID)
	if err != nil {
		return wrapf("PostgresRepository.UpdateState", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MaxVersion implements Repository.
func (r *PostgresRepository) MaxVersion(ctx context.Context, tenantID uuid.UUID) (int, error) {
	const q = `SELECT COALESCE(MAX(version), 0) FROM key_metadata WHERE tenant_id = $1`

	var max int
	row := r.exec.QueryRow(ctx, q, tenantID)
	if err := row.Scan(&max); err != nil {
		return 0, wrapf("PostgresRepository.MaxVersion", err)
	}
	return max, nil
}

var _ Repository = (*PostgresRepository)(nil)

// PostgresAuditRepository is a PostgreSQL-backed AuditRepository,
// storing AuditEntry rows in the `key_audit_entries` table (see the
// same migration as PostgresRepository).
type PostgresAuditRepository struct {
	exec persistence.Executor
}

// NewPostgresAuditRepository builds a PostgresAuditRepository bound
// to exec.
func NewPostgresAuditRepository(exec persistence.Executor) *PostgresAuditRepository {
	return &PostgresAuditRepository{exec: exec}
}

// Record implements AuditRepository.
func (r *PostgresAuditRepository) Record(ctx context.Context, tenantID uuid.UUID, entry *AuditEntry) error {
	if entry == nil {
		return wrapf("PostgresAuditRepository.Record", ErrNilKeyMetadata)
	}
	if entry.TenantID == uuid.Nil {
		entry.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, entry.TenantID); err != nil {
		return err
	}
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}

	const q = `
		INSERT INTO key_audit_entries
			(id, tenant_id, actor, action, key_id, outcome, justification, detail, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()))`

	_, err := r.exec.Exec(ctx, q,
		entry.ID, entry.TenantID, entry.Actor, string(entry.Action), entry.KeyID,
		string(entry.Outcome), entry.Justification, entry.Detail, entry.OccurredAt,
	)
	if err != nil {
		return wrapf("PostgresAuditRepository.Record", err)
	}
	return nil
}

// ListForTenant implements AuditRepository.
func (r *PostgresAuditRepository) ListForTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]*AuditEntry, error) {
	q := `
		SELECT id, tenant_id, actor, action, key_id, outcome, justification, detail, occurred_at
		FROM key_audit_entries
		WHERE tenant_id = $1
		ORDER BY occurred_at DESC`
	args := []any{tenantID}
	if limit > 0 {
		args = append(args, limit)
		q += " LIMIT $2"
	}

	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresAuditRepository.ListForTenant", err)
	}
	defer rows.Close()

	out := make([]*AuditEntry, 0)
	for rows.Next() {
		e := &AuditEntry{}
		var action, outcome string
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.Actor, &action, &e.KeyID, &outcome,
			&e.Justification, &e.Detail, &e.OccurredAt,
		); err != nil {
			return nil, wrapf("PostgresAuditRepository.ListForTenant", err)
		}
		e.Action = AuditAction(action)
		e.Outcome = AuditOutcome(outcome)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresAuditRepository.ListForTenant", err)
	}
	return out, nil
}

var _ AuditRepository = (*PostgresAuditRepository)(nil)
