package securitytesting

import (
	"context"
	"encoding/json"
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

// isUniqueViolation reports whether err represents a Postgres unique
// constraint violation (SQLSTATE 23505) -- the error
// securitytesting_run_records's id-uniqueness constraint raises when
// Create collides with an already-persisted RunRecord.ID, mirroring
// packages/compliance.isUniqueViolation exactly.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

// PostgresRunRecordRepository is a PostgreSQL-backed
// RunRecordRepository, storing RunRecord rows in the
// `securitytesting_run_records` table (see
// packages/persistence/migrations/000028_create_securitytesting.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/compliance.PostgresEvidenceRepository exactly.
type PostgresRunRecordRepository struct {
	exec persistence.Executor
}

// NewPostgresRunRecordRepository builds a PostgresRunRecordRepository
// bound to exec.
func NewPostgresRunRecordRepository(exec persistence.Executor) *PostgresRunRecordRepository {
	return &PostgresRunRecordRepository{exec: exec}
}

const runRecordColumns = `id, tenant_id, scenario_name, scenario_category, outcome, detail, evidence, run_by, ran_at`

func scanRunRecord(row rowScanner, rr *RunRecord) error {
	var evidenceJSON []byte
	var runBy *uuid.UUID
	if err := row.Scan(
		&rr.ID, &rr.TenantID, &rr.ScenarioName, &rr.ScenarioCategory,
		&rr.Result.Outcome, &rr.Result.Detail, &evidenceJSON, &runBy, &rr.RanAt,
	); err != nil {
		return err
	}
	if runBy != nil {
		rr.RunBy = *runBy
	}
	if len(evidenceJSON) > 0 {
		if err := json.Unmarshal(evidenceJSON, &rr.Result.Evidence); err != nil {
			return err
		}
	}
	return nil
}

// Create implements RunRecordRepository. Rejects a replayed
// RunRecord.ID with ErrDuplicateRunRecord, translating the underlying
// unique-constraint violation exactly as
// InMemoryRunRecordRepository.Create does, so both implementations
// enforce RunRecordRepository's append-only contract identically.
func (r *PostgresRunRecordRepository) Create(ctx context.Context, tenantID uuid.UUID, rr *RunRecord) error {
	if rr == nil {
		return ErrInvalidRunRecord
	}
	if rr.TenantID == uuid.Nil {
		rr.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rr.TenantID); err != nil {
		return err
	}
	evidenceJSON, err := json.Marshal(rr.Result.Evidence)
	if err != nil {
		return wrapf("PostgresRunRecordRepository.Create", err)
	}
	var runBy *uuid.UUID
	if rr.RunBy != uuid.Nil {
		runBy = &rr.RunBy
	}

	q := `
		INSERT INTO securitytesting_run_records (id, tenant_id, scenario_name, scenario_category, outcome, detail, evidence, run_by, ran_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + runRecordColumns

	row := r.exec.QueryRow(ctx, q, rr.ID, rr.TenantID, rr.ScenarioName, rr.ScenarioCategory,
		rr.Result.Outcome, rr.Result.Detail, evidenceJSON, runBy, rr.RanAt)
	if err := scanRunRecord(row, rr); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateRunRecord
		}
		return wrapf("PostgresRunRecordRepository.Create", err)
	}
	return nil
}

// Get implements RunRecordRepository.
func (r *PostgresRunRecordRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*RunRecord, error) {
	q := `SELECT ` + runRecordColumns + ` FROM securitytesting_run_records WHERE id = $1 AND tenant_id = $2`
	rr := &RunRecord{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanRunRecord(row, rr); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidRunRecord
		}
		return nil, wrapf("PostgresRunRecordRepository.Get", err)
	}
	return rr, nil
}

// ListAll implements RunRecordRepository.
func (r *PostgresRunRecordRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]RunRecord, error) {
	q := `SELECT ` + runRecordColumns + ` FROM securitytesting_run_records WHERE tenant_id = $1 ORDER BY ran_at ASC`
	return r.queryRunRecords(ctx, q, tenantID)
}

// ListForScenario implements RunRecordRepository.
func (r *PostgresRunRecordRepository) ListForScenario(ctx context.Context, tenantID uuid.UUID, scenarioName string) ([]RunRecord, error) {
	q := `SELECT ` + runRecordColumns + ` FROM securitytesting_run_records WHERE tenant_id = $1 AND scenario_name = $2 ORDER BY ran_at ASC`
	return r.queryRunRecords(ctx, q, tenantID, scenarioName)
}

func (r *PostgresRunRecordRepository) queryRunRecords(ctx context.Context, q string, args ...any) ([]RunRecord, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRunRecordRepository.query", err)
	}
	defer rows.Close()

	out := make([]RunRecord, 0)
	for rows.Next() {
		var rr RunRecord
		if err := scanRunRecord(rows, &rr); err != nil {
			return nil, wrapf("PostgresRunRecordRepository.query", err)
		}
		out = append(out, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRunRecordRepository.query", err)
	}
	return out, nil
}

var _ RunRecordRepository = (*PostgresRunRecordRepository)(nil)

// PostgresFindingRepository is a PostgreSQL-backed FindingRepository,
// storing Finding rows in the `securitytesting_findings` table.
type PostgresFindingRepository struct {
	exec persistence.Executor
}

// NewPostgresFindingRepository builds a PostgresFindingRepository
// bound to exec.
func NewPostgresFindingRepository(exec persistence.Executor) *PostgresFindingRepository {
	return &PostgresFindingRepository{exec: exec}
}

const findingColumns = `id, tenant_id, title, category, severity, source_scenario, source_run_id, detail, status, risk_accepted_justification, opened_by, opened_at, updated_at`

func scanFinding(row rowScanner, f *Finding) error {
	var openedBy *uuid.UUID
	if err := row.Scan(
		&f.ID, &f.TenantID, &f.Title, &f.Category, &f.Severity, &f.SourceScenario,
		&f.SourceRunID, &f.Detail, &f.Status, &f.RiskAcceptedJustification,
		&openedBy, &f.OpenedAt, &f.UpdatedAt,
	); err != nil {
		return err
	}
	if openedBy != nil {
		f.OpenedBy = *openedBy
	}
	return nil
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
	var openedBy *uuid.UUID
	if f.OpenedBy != uuid.Nil {
		openedBy = &f.OpenedBy
	}

	q := `
		INSERT INTO securitytesting_findings (id, tenant_id, title, category, severity, source_scenario, source_run_id, detail, status, risk_accepted_justification, opened_by, opened_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, COALESCE(NULLIF($12, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($13, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + findingColumns

	row := r.exec.QueryRow(ctx, q, f.ID, f.TenantID, f.Title, f.Category, f.Severity, f.SourceScenario,
		f.SourceRunID, f.Detail, f.Status, f.RiskAcceptedJustification, openedBy, f.OpenedAt, f.UpdatedAt)
	if err := scanFinding(row, f); err != nil {
		return wrapf("PostgresFindingRepository.Create", err)
	}
	return nil
}

// Get implements FindingRepository.
func (r *PostgresFindingRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Finding, error) {
	q := `SELECT ` + findingColumns + ` FROM securitytesting_findings WHERE id = $1 AND tenant_id = $2`
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
	q := `SELECT ` + findingColumns + ` FROM securitytesting_findings WHERE tenant_id = $1 ORDER BY opened_at ASC`
	return r.queryFindings(ctx, q, tenantID)
}

// ListByStatus implements FindingRepository.
func (r *PostgresFindingRepository) ListByStatus(ctx context.Context, tenantID uuid.UUID, status FindingStatus) ([]Finding, error) {
	q := `SELECT ` + findingColumns + ` FROM securitytesting_findings WHERE tenant_id = $1 AND status = $2 ORDER BY opened_at ASC`
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
		UPDATE securitytesting_findings
		SET title = $1, severity = $2, detail = $3, status = $4, risk_accepted_justification = $5, updated_at = now()
		WHERE id = $6 AND tenant_id = $7`

	tag, err := r.exec.Exec(ctx, q, f.Title, f.Severity, f.Detail, f.Status, f.RiskAcceptedJustification, f.ID, tenantID)
	if err != nil {
		return wrapf("PostgresFindingRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFindingNotFound
	}
	return nil
}

var _ FindingRepository = (*PostgresFindingRepository)(nil)
