package corpusupdater

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

// PostgresJobRepository is a PostgreSQL-backed JobRepository, storing
// CorpusUpdateJob rows in the `corpusupdater_jobs` table (see
// packages/persistence/migrations for the exact schema). It accepts a
// persistence.Executor per call, mirroring
// packages/privacy.PostgresInventoryRepository exactly, so callers can
// run it directly against a pool or compose it inside a transaction
// via persistence.WithTx or packages/tenancy.WithTenantScope.
type PostgresJobRepository struct {
	exec persistence.Executor
}

// NewPostgresJobRepository builds a PostgresJobRepository bound to
// exec.
func NewPostgresJobRepository(exec persistence.Executor) *PostgresJobRepository {
	return &PostgresJobRepository{exec: exec}
}

const jobColumns = `id, tenant_id, jurisdiction_code, target_corpus, source_description, status, failure_reason, created_by, created_at, updated_at`

func scanJob(row rowScanner, j *CorpusUpdateJob) error {
	return row.Scan(
		&j.ID, &j.TenantID, &j.JurisdictionCode, &j.TargetCorpus, &j.SourceDescription,
		&j.Status, &j.FailureReason, &j.CreatedBy, &j.CreatedAt, &j.UpdatedAt,
	)
}

// Create implements JobRepository.
func (r *PostgresJobRepository) Create(ctx context.Context, tenantID uuid.UUID, j *CorpusUpdateJob) error {
	if j == nil {
		return ErrInvalidJob
	}
	if j.TenantID == uuid.Nil {
		j.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, j.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO corpusupdater_jobs (id, tenant_id, jurisdiction_code, target_corpus, source_description, status, failure_reason, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + jobColumns

	row := r.exec.QueryRow(ctx, q, j.ID, j.TenantID, j.JurisdictionCode, j.TargetCorpus, j.SourceDescription,
		j.Status, j.FailureReason, j.CreatedBy, j.CreatedAt, j.UpdatedAt)
	if err := scanJob(row, j); err != nil {
		return wrapf("PostgresJobRepository.Create", err)
	}
	return nil
}

// Get implements JobRepository.
func (r *PostgresJobRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*CorpusUpdateJob, error) {
	q := `SELECT ` + jobColumns + ` FROM corpusupdater_jobs WHERE id = $1 AND tenant_id = $2`
	j := &CorpusUpdateJob{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanJob(row, j); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, wrapf("PostgresJobRepository.Get", err)
	}
	return j, nil
}

// ListByJurisdiction implements JobRepository.
func (r *PostgresJobRepository) ListByJurisdiction(ctx context.Context, tenantID uuid.UUID, jurisdictionCode string) ([]CorpusUpdateJob, error) {
	q := `SELECT ` + jobColumns + ` FROM corpusupdater_jobs WHERE tenant_id = $1 AND jurisdiction_code = $2 ORDER BY created_at ASC`
	return r.queryJobs(ctx, q, tenantID, jurisdictionCode)
}

// ListAll implements JobRepository.
func (r *PostgresJobRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]CorpusUpdateJob, error) {
	q := `SELECT ` + jobColumns + ` FROM corpusupdater_jobs WHERE tenant_id = $1 ORDER BY created_at ASC`
	return r.queryJobs(ctx, q, tenantID)
}

func (r *PostgresJobRepository) queryJobs(ctx context.Context, q string, args ...any) ([]CorpusUpdateJob, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresJobRepository.query", err)
	}
	defer rows.Close()

	out := make([]CorpusUpdateJob, 0)
	for rows.Next() {
		var j CorpusUpdateJob
		if err := scanJob(rows, &j); err != nil {
			return nil, wrapf("PostgresJobRepository.query", err)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresJobRepository.query", err)
	}
	return out, nil
}

// Update implements JobRepository.
func (r *PostgresJobRepository) Update(ctx context.Context, tenantID uuid.UUID, j *CorpusUpdateJob) error {
	if j == nil {
		return ErrInvalidJob
	}
	const q = `
		UPDATE corpusupdater_jobs
		SET status = $1, failure_reason = $2, updated_at = now()
		WHERE id = $3 AND tenant_id = $4`

	tag, err := r.exec.Exec(ctx, q, j.Status, j.FailureReason, j.ID, tenantID)
	if err != nil {
		return wrapf("PostgresJobRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrJobNotFound
	}
	return nil
}

var _ JobRepository = (*PostgresJobRepository)(nil)

// PostgresAmendmentRepository is a PostgreSQL-backed
// AmendmentRepository, storing Amendment rows in the
// `corpusupdater_amendments` table.
type PostgresAmendmentRepository struct {
	exec persistence.Executor
}

// NewPostgresAmendmentRepository builds a PostgresAmendmentRepository
// bound to exec.
func NewPostgresAmendmentRepository(exec persistence.Executor) *PostgresAmendmentRepository {
	return &PostgresAmendmentRepository{exec: exec}
}

const amendmentColumns = `id, tenant_id, job_id, target_corpus, target_id, change_type, new_text, citation, effective_date, previous_text, previous_citation, applied, rolled_back, created_by, created_at, updated_at`

func scanAmendment(row rowScanner, a *Amendment) error {
	return row.Scan(
		&a.ID, &a.TenantID, &a.JobID, &a.TargetCorpus, &a.TargetID, &a.ChangeType, &a.NewText,
		&a.Citation, &a.EffectiveDate, &a.PreviousText, &a.PreviousCitation, &a.Applied, &a.RolledBack,
		&a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
}

// Create implements AmendmentRepository.
func (r *PostgresAmendmentRepository) Create(ctx context.Context, tenantID uuid.UUID, a *Amendment) error {
	if a == nil {
		return ErrInvalidAmendment
	}
	if a.TenantID == uuid.Nil {
		a.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, a.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO corpusupdater_amendments (id, tenant_id, job_id, target_corpus, target_id, change_type, new_text, citation, effective_date, previous_text, previous_citation, applied, rolled_back, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, COALESCE(NULLIF($15, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($16, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + amendmentColumns

	row := r.exec.QueryRow(ctx, q, a.ID, a.TenantID, a.JobID, a.TargetCorpus, a.TargetID, a.ChangeType, a.NewText,
		a.Citation, a.EffectiveDate, a.PreviousText, a.PreviousCitation, a.Applied, a.RolledBack,
		a.CreatedBy, a.CreatedAt, a.UpdatedAt)
	if err := scanAmendment(row, a); err != nil {
		return wrapf("PostgresAmendmentRepository.Create", err)
	}
	return nil
}

// Get implements AmendmentRepository.
func (r *PostgresAmendmentRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Amendment, error) {
	q := `SELECT ` + amendmentColumns + ` FROM corpusupdater_amendments WHERE id = $1 AND tenant_id = $2`
	a := &Amendment{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanAmendment(row, a); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAmendmentNotFound
		}
		return nil, wrapf("PostgresAmendmentRepository.Get", err)
	}
	return a, nil
}

// ListForJob implements AmendmentRepository.
func (r *PostgresAmendmentRepository) ListForJob(ctx context.Context, tenantID, jobID uuid.UUID) ([]Amendment, error) {
	q := `SELECT ` + amendmentColumns + ` FROM corpusupdater_amendments WHERE tenant_id = $1 AND job_id = $2 ORDER BY created_at ASC`
	return r.queryAmendments(ctx, q, tenantID, jobID)
}

// ListForTarget implements AmendmentRepository.
func (r *PostgresAmendmentRepository) ListForTarget(ctx context.Context, tenantID uuid.UUID, corpus CorpusTarget, targetID string) ([]Amendment, error) {
	q := `SELECT ` + amendmentColumns + ` FROM corpusupdater_amendments WHERE tenant_id = $1 AND target_corpus = $2 AND target_id = $3 ORDER BY created_at ASC`
	return r.queryAmendments(ctx, q, tenantID, corpus, targetID)
}

func (r *PostgresAmendmentRepository) queryAmendments(ctx context.Context, q string, args ...any) ([]Amendment, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresAmendmentRepository.query", err)
	}
	defer rows.Close()

	out := make([]Amendment, 0)
	for rows.Next() {
		var a Amendment
		if err := scanAmendment(rows, &a); err != nil {
			return nil, wrapf("PostgresAmendmentRepository.query", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresAmendmentRepository.query", err)
	}
	return out, nil
}

// Update implements AmendmentRepository.
func (r *PostgresAmendmentRepository) Update(ctx context.Context, tenantID uuid.UUID, a *Amendment) error {
	if a == nil {
		return ErrInvalidAmendment
	}
	const q = `
		UPDATE corpusupdater_amendments
		SET new_text = $1, citation = $2, effective_date = $3, previous_text = $4,
		    previous_citation = $5, applied = $6, rolled_back = $7, updated_at = now()
		WHERE id = $8 AND tenant_id = $9`

	tag, err := r.exec.Exec(ctx, q, a.NewText, a.Citation, a.EffectiveDate, a.PreviousText,
		a.PreviousCitation, a.Applied, a.RolledBack, a.ID, tenantID)
	if err != nil {
		return wrapf("PostgresAmendmentRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAmendmentNotFound
	}
	return nil
}

var _ AmendmentRepository = (*PostgresAmendmentRepository)(nil)
