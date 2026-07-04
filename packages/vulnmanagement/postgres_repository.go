package vulnmanagement

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// rowScanner is the subset of pgx.Row / pgx.Rows this file's scan
// helpers depend on, mirroring packages/compliance.rowScanner and
// packages/privacy.rowScanner exactly.
type rowScanner interface {
	Scan(dest ...any) error
}

// PostgresFindingRepository is a PostgreSQL-backed FindingRepository,
// storing Finding rows in the `vulnmanagement_findings` table (see
// packages/persistence/migrations/000030_create_vulnmanagement.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/compliance.PostgresEvidenceRepository exactly.
type PostgresFindingRepository struct {
	exec persistence.Executor
}

// NewPostgresFindingRepository builds a PostgresFindingRepository
// bound to exec.
func NewPostgresFindingRepository(exec persistence.Executor) *PostgresFindingRepository {
	return &PostgresFindingRepository{exec: exec}
}

const findingColumns = `id, tenant_id, source, package, version, severity, advisory_id, title, description, status, discovered_at, created_at, updated_at`

func scanFinding(row rowScanner, f *Finding) error {
	return row.Scan(
		&f.ID, &f.TenantID, &f.Source, &f.Package, &f.Version, &f.Severity, &f.AdvisoryID,
		&f.Title, &f.Description, &f.Status, &f.DiscoveredAt, &f.CreatedAt, &f.UpdatedAt,
	)
}

// Create implements FindingRepository.
func (r *PostgresFindingRepository) Create(ctx context.Context, tenantID uuid.UUID, f *Finding) error {
	if f == nil {
		return ErrInvalidFinding
	}
	if f.TenantID == uuid.Nil {
		f.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, f.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO vulnmanagement_findings (id, tenant_id, source, package, version, severity, advisory_id, title, description, status, discovered_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, COALESCE(NULLIF($12, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($13, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + findingColumns

	row := r.exec.QueryRow(ctx, q, f.ID, f.TenantID, f.Source, f.Package, f.Version, f.Severity, f.AdvisoryID,
		f.Title, f.Description, f.Status, f.DiscoveredAt, f.CreatedAt, f.UpdatedAt)
	if err := scanFinding(row, f); err != nil {
		return wrapf("PostgresFindingRepository.Create", err)
	}
	return nil
}

// Get implements FindingRepository.
func (r *PostgresFindingRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Finding, error) {
	q := `SELECT ` + findingColumns + ` FROM vulnmanagement_findings WHERE id = $1 AND tenant_id = $2`
	f := &Finding{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanFinding(row, f); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFindingNotFound
		}
		return nil, wrapf("PostgresFindingRepository.Get", err)
	}
	return f, nil
}

// ListAll implements FindingRepository.
func (r *PostgresFindingRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Finding, error) {
	q := `SELECT ` + findingColumns + ` FROM vulnmanagement_findings WHERE tenant_id = $1 ORDER BY discovered_at ASC`
	return r.queryFindings(ctx, q, tenantID)
}

// ListBySource implements FindingRepository.
func (r *PostgresFindingRepository) ListBySource(ctx context.Context, tenantID uuid.UUID, source ScannerSource) ([]Finding, error) {
	q := `SELECT ` + findingColumns + ` FROM vulnmanagement_findings WHERE tenant_id = $1 AND source = $2 ORDER BY discovered_at ASC`
	return r.queryFindings(ctx, q, tenantID, source)
}

// ListByStatus implements FindingRepository.
func (r *PostgresFindingRepository) ListByStatus(ctx context.Context, tenantID uuid.UUID, status Status) ([]Finding, error) {
	q := `SELECT ` + findingColumns + ` FROM vulnmanagement_findings WHERE tenant_id = $1 AND status = $2 ORDER BY discovered_at ASC`
	return r.queryFindings(ctx, q, tenantID, status)
}

func (r *PostgresFindingRepository) queryFindings(ctx context.Context, q string, args ...any) ([]Finding, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresFindingRepository.query", err)
	}
	defer rows.Close()

	out := make([]Finding, 0)
	for rows.Next() {
		var f Finding
		if err := scanFinding(rows, &f); err != nil {
			return nil, wrapf("PostgresFindingRepository.query", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresFindingRepository.query", err)
	}
	return out, nil
}

// Update implements FindingRepository.
func (r *PostgresFindingRepository) Update(ctx context.Context, tenantID uuid.UUID, f *Finding) error {
	if f == nil {
		return ErrInvalidFinding
	}
	if err := requireMatchingTenant(tenantID, f.TenantID); err != nil {
		return err
	}
	const q = `
		UPDATE vulnmanagement_findings
		SET severity = $1, title = $2, description = $3, status = $4, updated_at = now()
		WHERE id = $5 AND tenant_id = $6`

	tag, err := r.exec.Exec(ctx, q, f.Severity, f.Title, f.Description, f.Status, f.ID, tenantID)
	if err != nil {
		return wrapf("PostgresFindingRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFindingNotFound
	}
	return nil
}

var _ FindingRepository = (*PostgresFindingRepository)(nil)

// PostgresTriageRepository is a PostgreSQL-backed TriageRepository,
// storing TriageDecision rows in the `vulnmanagement_triage_decisions`
// table.
type PostgresTriageRepository struct {
	exec persistence.Executor
}

// NewPostgresTriageRepository builds a PostgresTriageRepository bound
// to exec.
func NewPostgresTriageRepository(exec persistence.Executor) *PostgresTriageRepository {
	return &PostgresTriageRepository{exec: exec}
}

const triageColumns = `id, tenant_id, finding_id, from_status, to_status, notes, actor, decided_at`

func scanTriage(row rowScanner, d *TriageDecision) error {
	return row.Scan(&d.ID, &d.TenantID, &d.FindingID, &d.FromStatus, &d.ToStatus, &d.Notes, &d.Actor, &d.DecidedAt)
}

// Create implements TriageRepository.
func (r *PostgresTriageRepository) Create(ctx context.Context, tenantID uuid.UUID, d *TriageDecision) error {
	if d == nil {
		return ErrInvalidTriageDecision
	}
	if d.TenantID == uuid.Nil {
		d.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, d.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO vulnmanagement_triage_decisions (id, tenant_id, finding_id, from_status, to_status, notes, actor, decided_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + triageColumns

	row := r.exec.QueryRow(ctx, q, d.ID, d.TenantID, d.FindingID, d.FromStatus, d.ToStatus, d.Notes, d.Actor, d.DecidedAt)
	if err := scanTriage(row, d); err != nil {
		return wrapf("PostgresTriageRepository.Create", err)
	}
	return nil
}

// ListForFinding implements TriageRepository.
func (r *PostgresTriageRepository) ListForFinding(ctx context.Context, tenantID, findingID uuid.UUID) ([]TriageDecision, error) {
	q := `SELECT ` + triageColumns + ` FROM vulnmanagement_triage_decisions WHERE tenant_id = $1 AND finding_id = $2 ORDER BY decided_at ASC`
	return r.queryTriage(ctx, q, tenantID, findingID)
}

// ListAll implements TriageRepository.
func (r *PostgresTriageRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]TriageDecision, error) {
	q := `SELECT ` + triageColumns + ` FROM vulnmanagement_triage_decisions WHERE tenant_id = $1 ORDER BY decided_at ASC`
	return r.queryTriage(ctx, q, tenantID)
}

func (r *PostgresTriageRepository) queryTriage(ctx context.Context, q string, args ...any) ([]TriageDecision, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresTriageRepository.query", err)
	}
	defer rows.Close()

	out := make([]TriageDecision, 0)
	for rows.Next() {
		var d TriageDecision
		if err := scanTriage(rows, &d); err != nil {
			return nil, wrapf("PostgresTriageRepository.query", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresTriageRepository.query", err)
	}
	return out, nil
}

var _ TriageRepository = (*PostgresTriageRepository)(nil)
