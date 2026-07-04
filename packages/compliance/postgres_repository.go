package compliance

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// rowScanner is the subset of pgx.Row / pgx.Rows this file's scan
// helpers depend on, mirroring packages/privacy.rowScanner and
// packages/accessgovernance.rowScanner exactly.
type rowScanner interface {
	Scan(dest ...any) error
}

// PostgresControlRepository is a PostgreSQL-backed ControlRepository,
// storing Control rows in the `compliance_controls` table (see
// packages/persistence/migrations/000026_create_compliance.up.sql).
// Unlike the tenant-scoped repositories below, this repository is
// never wrapped by packages/tenancy.WithTenantScope: compliance_controls
// carries no tenant_id column and no RLS policy, since a Control is
// shared catalogue data rather than a per-tenant record (see
// ControlRepository's doc comment). It accepts a persistence.Executor
// per call, mirroring packages/privacy.PostgresInventoryRepository
// exactly.
type PostgresControlRepository struct {
	exec persistence.Executor
}

// NewPostgresControlRepository builds a PostgresControlRepository
// bound to exec.
func NewPostgresControlRepository(exec persistence.Executor) *PostgresControlRepository {
	return &PostgresControlRepository{exec: exec}
}

const controlColumns = `id, code, title, description, framework, category, mapped_to, created_by, created_at, updated_at`

func scanControl(row rowScanner, c *Control) error {
	var mappedToJSON []byte
	if err := row.Scan(
		&c.ID, &c.Code, &c.Title, &c.Description, &c.Framework, &c.Category,
		&mappedToJSON, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return err
	}
	if len(mappedToJSON) > 0 {
		if err := json.Unmarshal(mappedToJSON, &c.MappedTo); err != nil {
			return err
		}
	}
	return nil
}

// Create implements ControlRepository.
func (r *PostgresControlRepository) Create(ctx context.Context, c *Control) error {
	if c == nil {
		return ErrInvalidControl
	}
	mappedToJSON, err := json.Marshal(c.MappedTo)
	if err != nil {
		return wrapf("PostgresControlRepository.Create", err)
	}

	q := `
		INSERT INTO compliance_controls (id, code, title, description, framework, category, mapped_to, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + controlColumns

	row := r.exec.QueryRow(ctx, q, c.ID, c.Code, c.Title, c.Description, c.Framework, c.Category,
		mappedToJSON, c.CreatedBy, c.CreatedAt, c.UpdatedAt)
	if err := scanControl(row, c); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateControl
		}
		return wrapf("PostgresControlRepository.Create", err)
	}
	return nil
}

// Get implements ControlRepository.
func (r *PostgresControlRepository) Get(ctx context.Context, id uuid.UUID) (*Control, error) {
	q := `SELECT ` + controlColumns + ` FROM compliance_controls WHERE id = $1`
	c := &Control{}
	row := r.exec.QueryRow(ctx, q, id)
	if err := scanControl(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrControlNotFound
		}
		return nil, wrapf("PostgresControlRepository.Get", err)
	}
	return c, nil
}

// GetByCode implements ControlRepository.
func (r *PostgresControlRepository) GetByCode(ctx context.Context, code string) (*Control, error) {
	q := `SELECT ` + controlColumns + ` FROM compliance_controls WHERE code = $1`
	c := &Control{}
	row := r.exec.QueryRow(ctx, q, code)
	if err := scanControl(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrControlNotFound
		}
		return nil, wrapf("PostgresControlRepository.GetByCode", err)
	}
	return c, nil
}

// List implements ControlRepository.
func (r *PostgresControlRepository) List(ctx context.Context) ([]Control, error) {
	q := `SELECT ` + controlColumns + ` FROM compliance_controls ORDER BY code ASC`
	return r.queryControls(ctx, q)
}

// ListByFramework implements ControlRepository.
func (r *PostgresControlRepository) ListByFramework(ctx context.Context, framework Framework) ([]Control, error) {
	q := `SELECT ` + controlColumns + ` FROM compliance_controls WHERE framework = $1 ORDER BY code ASC`
	return r.queryControls(ctx, q, framework)
}

func (r *PostgresControlRepository) queryControls(ctx context.Context, q string, args ...any) ([]Control, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresControlRepository.query", err)
	}
	defer rows.Close()

	out := make([]Control, 0)
	for rows.Next() {
		var c Control
		if err := scanControl(rows, &c); err != nil {
			return nil, wrapf("PostgresControlRepository.query", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresControlRepository.query", err)
	}
	return out, nil
}

// Update implements ControlRepository.
func (r *PostgresControlRepository) Update(ctx context.Context, c *Control) error {
	if c == nil {
		return ErrInvalidControl
	}
	mappedToJSON, err := json.Marshal(c.MappedTo)
	if err != nil {
		return wrapf("PostgresControlRepository.Update", err)
	}
	const q = `
		UPDATE compliance_controls
		SET title = $1, description = $2, framework = $3, category = $4, mapped_to = $5, updated_at = now()
		WHERE id = $6`

	tag, err := r.exec.Exec(ctx, q, c.Title, c.Description, c.Framework, c.Category, mappedToJSON, c.ID)
	if err != nil {
		return wrapf("PostgresControlRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrControlNotFound
	}
	return nil
}

var _ ControlRepository = (*PostgresControlRepository)(nil)

// isUniqueViolation reports whether err represents a Postgres unique
// constraint violation (SQLSTATE 23505), the error compliance_controls's
// `code` UNIQUE constraint raises when Create collides with an
// existing catalogued control.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

// PostgresEvidenceRepository is a PostgreSQL-backed EvidenceRepository,
// storing ControlEvidence rows in the `compliance_control_evidence`
// table.
type PostgresEvidenceRepository struct {
	exec persistence.Executor
}

// NewPostgresEvidenceRepository builds a PostgresEvidenceRepository
// bound to exec.
func NewPostgresEvidenceRepository(exec persistence.Executor) *PostgresEvidenceRepository {
	return &PostgresEvidenceRepository{exec: exec}
}

const evidenceColumns = `id, tenant_id, control_id, kind, reference, description, collected_by, collected_at, created_at, updated_at`

func scanEvidence(row rowScanner, e *ControlEvidence) error {
	return row.Scan(
		&e.ID, &e.TenantID, &e.ControlID, &e.Kind, &e.Reference, &e.Description,
		&e.CollectedBy, &e.CollectedAt, &e.CreatedAt, &e.UpdatedAt,
	)
}

// Create implements EvidenceRepository.
func (r *PostgresEvidenceRepository) Create(ctx context.Context, tenantID uuid.UUID, e *ControlEvidence) error {
	if e == nil {
		return ErrInvalidEvidence
	}
	if e.TenantID == uuid.Nil {
		e.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, e.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO compliance_control_evidence (id, tenant_id, control_id, kind, reference, description, collected_by, collected_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + evidenceColumns

	row := r.exec.QueryRow(ctx, q, e.ID, e.TenantID, e.ControlID, e.Kind, e.Reference, e.Description,
		e.CollectedBy, e.CollectedAt, e.CreatedAt, e.UpdatedAt)
	if err := scanEvidence(row, e); err != nil {
		return wrapf("PostgresEvidenceRepository.Create", err)
	}
	return nil
}

// Get implements EvidenceRepository.
func (r *PostgresEvidenceRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ControlEvidence, error) {
	q := `SELECT ` + evidenceColumns + ` FROM compliance_control_evidence WHERE id = $1 AND tenant_id = $2`
	e := &ControlEvidence{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanEvidence(row, e); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEvidenceNotFound
		}
		return nil, wrapf("PostgresEvidenceRepository.Get", err)
	}
	return e, nil
}

// ListForControl implements EvidenceRepository.
func (r *PostgresEvidenceRepository) ListForControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]ControlEvidence, error) {
	q := `SELECT ` + evidenceColumns + ` FROM compliance_control_evidence WHERE tenant_id = $1 AND control_id = $2 ORDER BY collected_at ASC`
	return r.queryEvidence(ctx, q, tenantID, controlID)
}

// ListAll implements EvidenceRepository.
func (r *PostgresEvidenceRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ControlEvidence, error) {
	q := `SELECT ` + evidenceColumns + ` FROM compliance_control_evidence WHERE tenant_id = $1 ORDER BY collected_at ASC`
	return r.queryEvidence(ctx, q, tenantID)
}

func (r *PostgresEvidenceRepository) queryEvidence(ctx context.Context, q string, args ...any) ([]ControlEvidence, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresEvidenceRepository.query", err)
	}
	defer rows.Close()

	out := make([]ControlEvidence, 0)
	for rows.Next() {
		var e ControlEvidence
		if err := scanEvidence(rows, &e); err != nil {
			return nil, wrapf("PostgresEvidenceRepository.query", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresEvidenceRepository.query", err)
	}
	return out, nil
}

var _ EvidenceRepository = (*PostgresEvidenceRepository)(nil)

// PostgresProfileRepository is a PostgreSQL-backed ProfileRepository,
// storing Profile rows in the `compliance_profiles` table
// (one row per tenant).
type PostgresProfileRepository struct {
	exec persistence.Executor
}

// NewPostgresProfileRepository builds a PostgresProfileRepository
// bound to exec.
func NewPostgresProfileRepository(exec persistence.Executor) *PostgresProfileRepository {
	return &PostgresProfileRepository{exec: exec}
}

const profileColumns = `tenant_id, frameworks, excluded_control_ids, set_by, created_at, updated_at`

func scanProfile(row rowScanner, p *Profile) error {
	var frameworksJSON, excludedJSON []byte
	if err := row.Scan(&p.TenantID, &frameworksJSON, &excludedJSON, &p.SetBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	if len(frameworksJSON) > 0 {
		if err := json.Unmarshal(frameworksJSON, &p.Frameworks); err != nil {
			return err
		}
	}
	if len(excludedJSON) > 0 {
		if err := json.Unmarshal(excludedJSON, &p.ExcludedControlIDs); err != nil {
			return err
		}
	}
	return nil
}

// Set implements ProfileRepository, upserting tenantID's single
// Profile row.
func (r *PostgresProfileRepository) Set(ctx context.Context, tenantID uuid.UUID, p *Profile) error {
	if p == nil {
		return ErrInvalidProfile
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	frameworksJSON, err := json.Marshal(p.Frameworks)
	if err != nil {
		return wrapf("PostgresProfileRepository.Set", err)
	}
	excludedJSON, err := json.Marshal(p.ExcludedControlIDs)
	if err != nil {
		return wrapf("PostgresProfileRepository.Set", err)
	}

	q := `
		INSERT INTO compliance_profiles (tenant_id, frameworks, excluded_control_ids, set_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($6, TIMESTAMPTZ '0001-01-01'), now()))
		ON CONFLICT (tenant_id) DO UPDATE SET
			frameworks = EXCLUDED.frameworks,
			excluded_control_ids = EXCLUDED.excluded_control_ids,
			set_by = EXCLUDED.set_by,
			updated_at = now()
		RETURNING ` + profileColumns

	row := r.exec.QueryRow(ctx, q, tenantID, frameworksJSON, excludedJSON, p.SetBy, p.CreatedAt, p.UpdatedAt)
	if err := scanProfile(row, p); err != nil {
		return wrapf("PostgresProfileRepository.Set", err)
	}
	return nil
}

// Get implements ProfileRepository.
func (r *PostgresProfileRepository) Get(ctx context.Context, tenantID uuid.UUID) (*Profile, error) {
	q := `SELECT ` + profileColumns + ` FROM compliance_profiles WHERE tenant_id = $1`
	p := &Profile{}
	row := r.exec.QueryRow(ctx, q, tenantID)
	if err := scanProfile(row, p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProfileNotFound
		}
		return nil, wrapf("PostgresProfileRepository.Get", err)
	}
	return p, nil
}

var _ ProfileRepository = (*PostgresProfileRepository)(nil)
