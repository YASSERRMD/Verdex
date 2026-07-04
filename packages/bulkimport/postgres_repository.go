package bulkimport

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// rowScanner is the subset of pgx.Row / pgx.Rows this file's scan
// helpers depend on, mirroring packages/privacy.rowScanner and
// packages/compliance's equivalent exactly.
type rowScanner interface {
	Scan(dest ...any) error
}

// PostgresJobRepository is a PostgreSQL-backed JobRepository, storing
// ImportJob rows in the `bulkimport_jobs` table (see
// packages/persistence/migrations for the create-table migration). It
// accepts a persistence.Executor per call, mirroring
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

const jobColumns = `id, tenant_id, source_description, status, total_records, processed_records,
	failed_records, skipped_records, imported_records, cursor, created_by, created_at, updated_at,
	started_at, finished_at, failure_reason`

func scanJob(row rowScanner, j *ImportJob) error {
	var startedAt, finishedAt *time.Time
	if err := row.Scan(
		&j.ID, &j.TenantID, &j.SourceDescription, &j.Status, &j.TotalRecords, &j.ProcessedRecords,
		&j.FailedRecords, &j.SkippedRecords, &j.ImportedRecords, &j.Cursor, &j.CreatedBy, &j.CreatedAt, &j.UpdatedAt,
		&startedAt, &finishedAt, &j.FailureReason,
	); err != nil {
		return err
	}
	j.StartedAt = startedAt
	j.FinishedAt = finishedAt
	return nil
}

// Create implements JobRepository.
func (r *PostgresJobRepository) Create(ctx context.Context, tenantID uuid.UUID, j *ImportJob) error {
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
		INSERT INTO bulkimport_jobs (id, tenant_id, source_description, status, total_records, processed_records,
			failed_records, skipped_records, imported_records, cursor, created_by, created_at, updated_at,
			started_at, finished_at, failure_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			COALESCE(NULLIF($12, TIMESTAMPTZ '0001-01-01'), now()),
			COALESCE(NULLIF($13, TIMESTAMPTZ '0001-01-01'), now()),
			$14, $15, $16)
		RETURNING ` + jobColumns

	row := r.exec.QueryRow(ctx, q, j.ID, j.TenantID, j.SourceDescription, j.Status, j.TotalRecords, j.ProcessedRecords,
		j.FailedRecords, j.SkippedRecords, j.ImportedRecords, j.Cursor, j.CreatedBy, j.CreatedAt, j.UpdatedAt,
		j.StartedAt, j.FinishedAt, j.FailureReason)
	if err := scanJob(row, j); err != nil {
		return wrapf("PostgresJobRepository.Create", err)
	}
	return nil
}

// Get implements JobRepository.
func (r *PostgresJobRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportJob, error) {
	q := `SELECT ` + jobColumns + ` FROM bulkimport_jobs WHERE id = $1 AND tenant_id = $2`
	j := &ImportJob{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanJob(row, j); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, wrapf("PostgresJobRepository.Get", err)
	}
	return j, nil
}

// List implements JobRepository.
func (r *PostgresJobRepository) List(ctx context.Context, tenantID uuid.UUID) ([]ImportJob, error) {
	q := `SELECT ` + jobColumns + ` FROM bulkimport_jobs WHERE tenant_id = $1 ORDER BY created_at ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresJobRepository.List", err)
	}
	defer rows.Close()

	out := make([]ImportJob, 0)
	for rows.Next() {
		var j ImportJob
		if err := scanJob(rows, &j); err != nil {
			return nil, wrapf("PostgresJobRepository.List", err)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresJobRepository.List", err)
	}
	return out, nil
}

// Update implements JobRepository.
func (r *PostgresJobRepository) Update(ctx context.Context, tenantID uuid.UUID, j *ImportJob) error {
	if j == nil {
		return ErrInvalidJob
	}
	const q = `
		UPDATE bulkimport_jobs
		SET status = $1, processed_records = $2, failed_records = $3, skipped_records = $4,
		    imported_records = $5, cursor = $6, updated_at = now(), started_at = $7,
		    finished_at = $8, failure_reason = $9
		WHERE id = $10 AND tenant_id = $11`

	tag, err := r.exec.Exec(ctx, q, j.Status, j.ProcessedRecords, j.FailedRecords, j.SkippedRecords,
		j.ImportedRecords, j.Cursor, j.StartedAt, j.FinishedAt, j.FailureReason, j.ID, tenantID)
	if err != nil {
		return wrapf("PostgresJobRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrJobNotFound
	}
	return nil
}

var _ JobRepository = (*PostgresJobRepository)(nil)

// PostgresRecordRepository is a PostgreSQL-backed RecordRepository,
// storing ImportRecord rows in the `bulkimport_records` table.
type PostgresRecordRepository struct {
	exec persistence.Executor
}

// NewPostgresRecordRepository builds a PostgresRecordRepository bound
// to exec.
func NewPostgresRecordRepository(exec persistence.Executor) *PostgresRecordRepository {
	return &PostgresRecordRepository{exec: exec}
}

const recordColumns = `id, tenant_id, job_id, source_index, payload_ref, case_number, jurisdiction,
	party_names, dedup_key, validation_status, validation_errors, outcome, outcome_reason,
	created_case_id, created_at, updated_at`

func scanRecord(row rowScanner, rec *ImportRecord) error {
	var partyNamesJSON, validationErrorsJSON []byte
	var createdCaseID uuid.NullUUID
	if err := row.Scan(
		&rec.ID, &rec.TenantID, &rec.JobID, &rec.SourceIndex, &rec.PayloadRef, &rec.CaseNumber, &rec.Jurisdiction,
		&partyNamesJSON, &rec.DedupKey, &rec.ValidationStatus, &validationErrorsJSON, &rec.Outcome, &rec.OutcomeReason,
		&createdCaseID, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return err
	}
	if createdCaseID.Valid {
		rec.CreatedCaseID = createdCaseID.UUID
	}
	if len(partyNamesJSON) > 0 {
		if err := json.Unmarshal(partyNamesJSON, &rec.PartyNames); err != nil {
			return err
		}
	}
	if len(validationErrorsJSON) > 0 {
		if err := json.Unmarshal(validationErrorsJSON, &rec.ValidationErrors); err != nil {
			return err
		}
	}
	return nil
}

// Create implements RecordRepository.
func (r *PostgresRecordRepository) Create(ctx context.Context, tenantID uuid.UUID, rec *ImportRecord) error {
	if rec == nil {
		return ErrInvalidRecord
	}
	if rec.TenantID == uuid.Nil {
		rec.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rec.TenantID); err != nil {
		return err
	}

	partyNamesJSON, err := json.Marshal(rec.PartyNames)
	if err != nil {
		return wrapf("PostgresRecordRepository.Create", err)
	}
	validationErrorsJSON, err := json.Marshal(rec.ValidationErrors)
	if err != nil {
		return wrapf("PostgresRecordRepository.Create", err)
	}

	q := `
		INSERT INTO bulkimport_records (id, tenant_id, job_id, source_index, payload_ref, case_number, jurisdiction,
			party_names, dedup_key, validation_status, validation_errors, outcome, outcome_reason,
			created_case_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			COALESCE(NULLIF($15, TIMESTAMPTZ '0001-01-01'), now()),
			COALESCE(NULLIF($16, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + recordColumns

	nullCreatedCaseID := uuid.NullUUID{UUID: rec.CreatedCaseID, Valid: rec.CreatedCaseID != uuid.Nil}
	row := r.exec.QueryRow(ctx, q, rec.ID, rec.TenantID, rec.JobID, rec.SourceIndex, rec.PayloadRef, rec.CaseNumber, rec.Jurisdiction,
		partyNamesJSON, rec.DedupKey, rec.ValidationStatus, validationErrorsJSON, rec.Outcome, rec.OutcomeReason,
		nullCreatedCaseID, rec.CreatedAt, rec.UpdatedAt)
	if err := scanRecord(row, rec); err != nil {
		return wrapf("PostgresRecordRepository.Create", err)
	}
	return nil
}

// Get implements RecordRepository.
func (r *PostgresRecordRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportRecord, error) {
	q := `SELECT ` + recordColumns + ` FROM bulkimport_records WHERE id = $1 AND tenant_id = $2`
	rec := &ImportRecord{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanRecord(row, rec); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, wrapf("PostgresRecordRepository.Get", err)
	}
	return rec, nil
}

// ListForJob implements RecordRepository.
func (r *PostgresRecordRepository) ListForJob(ctx context.Context, tenantID, jobID uuid.UUID) ([]ImportRecord, error) {
	q := `SELECT ` + recordColumns + ` FROM bulkimport_records WHERE tenant_id = $1 AND job_id = $2 ORDER BY source_index ASC`
	rows, err := r.exec.Query(ctx, q, tenantID, jobID)
	if err != nil {
		return nil, wrapf("PostgresRecordRepository.ListForJob", err)
	}
	defer rows.Close()

	out := make([]ImportRecord, 0)
	for rows.Next() {
		var rec ImportRecord
		if err := scanRecord(rows, &rec); err != nil {
			return nil, wrapf("PostgresRecordRepository.ListForJob", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRecordRepository.ListForJob", err)
	}
	return out, nil
}

// FindByDedupKey implements RecordRepository.
func (r *PostgresRecordRepository) FindByDedupKey(ctx context.Context, tenantID, jobID uuid.UUID, key string) (*ImportRecord, error) {
	if key == "" {
		return nil, ErrRecordNotFound
	}
	q := `SELECT ` + recordColumns + ` FROM bulkimport_records
		WHERE tenant_id = $1 AND job_id = $2 AND dedup_key = $3 AND outcome = $4
		ORDER BY source_index ASC LIMIT 1`
	rec := &ImportRecord{}
	row := r.exec.QueryRow(ctx, q, tenantID, jobID, key, OutcomeImported)
	if err := scanRecord(row, rec); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, wrapf("PostgresRecordRepository.FindByDedupKey", err)
	}
	return rec, nil
}

// Update implements RecordRepository.
func (r *PostgresRecordRepository) Update(ctx context.Context, tenantID uuid.UUID, rec *ImportRecord) error {
	if rec == nil {
		return ErrInvalidRecord
	}
	validationErrorsJSON, err := json.Marshal(rec.ValidationErrors)
	if err != nil {
		return wrapf("PostgresRecordRepository.Update", err)
	}

	const q = `
		UPDATE bulkimport_records
		SET validation_status = $1, validation_errors = $2, outcome = $3, outcome_reason = $4,
		    created_case_id = $5, updated_at = now()
		WHERE id = $6 AND tenant_id = $7`

	nullCreatedCaseID := uuid.NullUUID{UUID: rec.CreatedCaseID, Valid: rec.CreatedCaseID != uuid.Nil}
	tag, err := r.exec.Exec(ctx, q, rec.ValidationStatus, validationErrorsJSON, rec.Outcome, rec.OutcomeReason,
		nullCreatedCaseID, rec.ID, tenantID)
	if err != nil {
		return wrapf("PostgresRecordRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRecordNotFound
	}
	return nil
}

var _ RecordRepository = (*PostgresRecordRepository)(nil)
