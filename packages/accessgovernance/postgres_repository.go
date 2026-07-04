package accessgovernance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresPolicyRepository is a PostgreSQL-backed PolicyRepository,
// storing Policy rows in the `access_policies` table (see
// packages/persistence/migrations/000022_create_accessgovernance.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/keymanagement.PostgresRepository exactly, so callers can run
// it directly against a pool or compose it inside a transaction via
// persistence.WithTx or packages/tenancy.WithTenantScope.
type PostgresPolicyRepository struct {
	exec persistence.Executor
}

// NewPostgresPolicyRepository builds a PostgresPolicyRepository bound
// to exec.
func NewPostgresPolicyRepository(exec persistence.Executor) *PostgresPolicyRepository {
	return &PostgresPolicyRepository{exec: exec}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPolicy(row rowScanner, p *Policy) error {
	var rulesJSON []byte
	if err := row.Scan(&p.ID, &p.TenantID, &p.Name, &rulesJSON, &p.Active, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	if len(rulesJSON) > 0 {
		if err := json.Unmarshal(rulesJSON, &p.Rules); err != nil {
			return err
		}
	}
	return nil
}

// Create implements PolicyRepository.
func (r *PostgresPolicyRepository) Create(ctx context.Context, tenantID uuid.UUID, p *Policy) error {
	if p == nil {
		return ErrNilPolicy
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	rulesJSON, err := json.Marshal(p.Rules)
	if err != nil {
		return wrapf("PostgresPolicyRepository.Create", err)
	}

	const q = `
		INSERT INTO access_policies (id, tenant_id, name, rules, active, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($8, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING id, tenant_id, name, rules, active, created_by, created_at, updated_at`

	row := r.exec.QueryRow(ctx, q, p.ID, p.TenantID, p.Name, rulesJSON, p.Active, p.CreatedBy, p.CreatedAt, p.UpdatedAt)
	if err := scanPolicy(row, p); err != nil {
		return wrapf("PostgresPolicyRepository.Create", err)
	}
	return nil
}

// Get implements PolicyRepository.
func (r *PostgresPolicyRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Policy, error) {
	const q = `
		SELECT id, tenant_id, name, rules, active, created_by, created_at, updated_at
		FROM access_policies WHERE id = $1 AND tenant_id = $2`

	p := &Policy{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanPolicy(row, p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, wrapf("PostgresPolicyRepository.Get", err)
	}
	return p, nil
}

// List implements PolicyRepository.
func (r *PostgresPolicyRepository) List(ctx context.Context, tenantID uuid.UUID) ([]Policy, error) {
	const q = `
		SELECT id, tenant_id, name, rules, active, created_by, created_at, updated_at
		FROM access_policies WHERE tenant_id = $1 ORDER BY created_at ASC`

	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresPolicyRepository.List", err)
	}
	defer rows.Close()

	out := make([]Policy, 0)
	for rows.Next() {
		var p Policy
		if err := scanPolicy(rows, &p); err != nil {
			return nil, wrapf("PostgresPolicyRepository.List", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresPolicyRepository.List", err)
	}
	return out, nil
}

// Update implements PolicyRepository.
func (r *PostgresPolicyRepository) Update(ctx context.Context, tenantID uuid.UUID, p *Policy) error {
	if p == nil {
		return ErrNilPolicy
	}
	rulesJSON, err := json.Marshal(p.Rules)
	if err != nil {
		return wrapf("PostgresPolicyRepository.Update", err)
	}

	const q = `
		UPDATE access_policies SET name = $1, rules = $2, active = $3, updated_at = now()
		WHERE id = $4 AND tenant_id = $5`

	tag, err := r.exec.Exec(ctx, q, p.Name, rulesJSON, p.Active, p.ID, tenantID)
	if err != nil {
		return wrapf("PostgresPolicyRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPolicyNotFound
	}
	return nil
}

var _ PolicyRepository = (*PostgresPolicyRepository)(nil)

// PostgresCaseGrantRepository is a PostgreSQL-backed
// CaseGrantRepository, storing CaseGrant rows in the
// `access_case_grants` table.
type PostgresCaseGrantRepository struct {
	exec persistence.Executor
}

// NewPostgresCaseGrantRepository builds a PostgresCaseGrantRepository
// bound to exec.
func NewPostgresCaseGrantRepository(exec persistence.Executor) *PostgresCaseGrantRepository {
	return &PostgresCaseGrantRepository{exec: exec}
}

func scanCaseGrant(row rowScanner, g *CaseGrant) error {
	var permsJSON []byte
	if err := row.Scan(
		&g.ID, &g.TenantID, &g.CaseID, &g.GranteeUserID, &permsJSON, &g.Deny,
		&g.ExpiresAt, &g.GrantedBy, &g.GrantedAt, &g.RevokedAt,
	); err != nil {
		return err
	}
	if len(permsJSON) > 0 {
		if err := json.Unmarshal(permsJSON, &g.Permissions); err != nil {
			return err
		}
	}
	return nil
}

const caseGrantColumns = `id, tenant_id, case_id, grantee_user_id, permissions, deny, expires_at, granted_by, granted_at, revoked_at`

// Create implements CaseGrantRepository.
func (r *PostgresCaseGrantRepository) Create(ctx context.Context, tenantID uuid.UUID, g *CaseGrant) error {
	if g == nil {
		return ErrInvalidGrant
	}
	if g.TenantID == uuid.Nil {
		g.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, g.TenantID); err != nil {
		return err
	}
	permsJSON, err := json.Marshal(g.Permissions)
	if err != nil {
		return wrapf("PostgresCaseGrantRepository.Create", err)
	}

	q := `
		INSERT INTO access_case_grants (id, tenant_id, case_id, grantee_user_id, permissions, deny, expires_at, granted_by, granted_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()), $10)
		RETURNING ` + caseGrantColumns

	row := r.exec.QueryRow(ctx, q, g.ID, g.TenantID, g.CaseID, g.GranteeUserID, permsJSON, g.Deny, g.ExpiresAt, g.GrantedBy, g.GrantedAt, g.RevokedAt)
	if err := scanCaseGrant(row, g); err != nil {
		return wrapf("PostgresCaseGrantRepository.Create", err)
	}
	return nil
}

// Get implements CaseGrantRepository.
func (r *PostgresCaseGrantRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*CaseGrant, error) {
	q := `SELECT ` + caseGrantColumns + ` FROM access_case_grants WHERE id = $1 AND tenant_id = $2`
	g := &CaseGrant{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanCaseGrant(row, g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGrantNotFound
		}
		return nil, wrapf("PostgresCaseGrantRepository.Get", err)
	}
	return g, nil
}

// ListForCase implements CaseGrantRepository.
func (r *PostgresCaseGrantRepository) ListForCase(ctx context.Context, tenantID, caseID uuid.UUID) ([]CaseGrant, error) {
	q := `SELECT ` + caseGrantColumns + ` FROM access_case_grants WHERE tenant_id = $1 AND case_id = $2`
	return r.queryCaseGrants(ctx, q, tenantID, caseID)
}

// ListActive implements CaseGrantRepository.
func (r *PostgresCaseGrantRepository) ListActive(ctx context.Context, tenantID uuid.UUID, now time.Time) ([]CaseGrant, error) {
	q := `SELECT ` + caseGrantColumns + ` FROM access_case_grants WHERE tenant_id = $1 AND expires_at > $2 AND revoked_at IS NULL`
	return r.queryCaseGrants(ctx, q, tenantID, now)
}

// ListAll implements CaseGrantRepository.
func (r *PostgresCaseGrantRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]CaseGrant, error) {
	q := `SELECT ` + caseGrantColumns + ` FROM access_case_grants WHERE tenant_id = $1`
	return r.queryCaseGrants(ctx, q, tenantID)
}

func (r *PostgresCaseGrantRepository) queryCaseGrants(ctx context.Context, q string, args ...any) ([]CaseGrant, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresCaseGrantRepository.query", err)
	}
	defer rows.Close()

	out := make([]CaseGrant, 0)
	for rows.Next() {
		var g CaseGrant
		if err := scanCaseGrant(rows, &g); err != nil {
			return nil, wrapf("PostgresCaseGrantRepository.query", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresCaseGrantRepository.query", err)
	}
	return out, nil
}

// Revoke implements CaseGrantRepository.
func (r *PostgresCaseGrantRepository) Revoke(ctx context.Context, tenantID, id uuid.UUID, revokedAt time.Time) error {
	const q = `UPDATE access_case_grants SET revoked_at = $1 WHERE id = $2 AND tenant_id = $3 AND revoked_at IS NULL`
	tag, err := r.exec.Exec(ctx, q, revokedAt, id, tenantID)
	if err != nil {
		return wrapf("PostgresCaseGrantRepository.Revoke", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrGrantNotFound
	}
	return nil
}

var _ CaseGrantRepository = (*PostgresCaseGrantRepository)(nil)
