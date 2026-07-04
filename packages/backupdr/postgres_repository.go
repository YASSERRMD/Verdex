package backupdr

import (
	"context"
	"errors"
	"time"

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

// PostgresPolicyRepository is a PostgreSQL-backed PolicyRepository,
// storing BackupPolicy rows in the `backupdr_policies` table (see
// packages/persistence/migrations/000028_create_backupdr.up.sql). It
// accepts a persistence.Executor per call, mirroring
// packages/privacy.PostgresConsentRepository exactly, so callers can
// run it directly against a pool or compose it inside a transaction
// via persistence.WithTx or packages/tenancy.WithTenantScope.
type PostgresPolicyRepository struct {
	exec persistence.Executor
}

// NewPostgresPolicyRepository builds a PostgresPolicyRepository bound
// to exec.
func NewPostgresPolicyRepository(exec persistence.Executor) *PostgresPolicyRepository {
	return &PostgresPolicyRepository{exec: exec}
}

const policyColumns = `tenant_id, class, frequency_seconds, retention_seconds, encryption_required, cross_region_required, created_by, created_at, updated_at`

func scanPolicy(row rowScanner, p *BackupPolicy) error {
	var frequencySeconds, retentionSeconds int64
	if err := row.Scan(
		&p.TenantID, &p.Class, &frequencySeconds, &retentionSeconds,
		&p.EncryptionRequired, &p.CrossRegionRequired, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return err
	}
	p.Frequency = time.Duration(frequencySeconds) * time.Second
	p.RetentionWindow = time.Duration(retentionSeconds) * time.Second
	return nil
}

// Set implements PolicyRepository, upserting on the (tenant_id, class)
// primary key.
func (r *PostgresPolicyRepository) Set(ctx context.Context, tenantID uuid.UUID, p *BackupPolicy) error {
	if p == nil {
		return ErrInvalidPolicy
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO backupdr_policies (tenant_id, class, frequency_seconds, retention_seconds, encryption_required, cross_region_required, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE(NULLIF($8, TIMESTAMPTZ '0001-01-01'), now()), now())
		ON CONFLICT (tenant_id, class) DO UPDATE SET
			frequency_seconds = EXCLUDED.frequency_seconds,
			retention_seconds = EXCLUDED.retention_seconds,
			encryption_required = EXCLUDED.encryption_required,
			cross_region_required = EXCLUDED.cross_region_required,
			updated_at = now()
		RETURNING ` + policyColumns

	row := r.exec.QueryRow(ctx, q, p.TenantID, p.Class, int64(p.Frequency/time.Second), int64(p.RetentionWindow/time.Second),
		p.EncryptionRequired, p.CrossRegionRequired, p.CreatedBy, p.CreatedAt)
	if err := scanPolicy(row, p); err != nil {
		return wrapf("PostgresPolicyRepository.Set", err)
	}
	return nil
}

// Get implements PolicyRepository.
func (r *PostgresPolicyRepository) Get(ctx context.Context, tenantID uuid.UUID, class DataClass) (*BackupPolicy, error) {
	q := `SELECT ` + policyColumns + ` FROM backupdr_policies WHERE tenant_id = $1 AND class = $2`
	p := &BackupPolicy{}
	row := r.exec.QueryRow(ctx, q, tenantID, class)
	if err := scanPolicy(row, p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, wrapf("PostgresPolicyRepository.Get", err)
	}
	return p, nil
}

// ListAll implements PolicyRepository.
func (r *PostgresPolicyRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]BackupPolicy, error) {
	q := `SELECT ` + policyColumns + ` FROM backupdr_policies WHERE tenant_id = $1 ORDER BY class ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresPolicyRepository.ListAll", err)
	}
	defer rows.Close()

	out := make([]BackupPolicy, 0)
	for rows.Next() {
		var p BackupPolicy
		if err := scanPolicy(rows, &p); err != nil {
			return nil, wrapf("PostgresPolicyRepository.ListAll", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresPolicyRepository.ListAll", err)
	}
	return out, nil
}

var _ PolicyRepository = (*PostgresPolicyRepository)(nil)

// PostgresRecordRepository is a PostgreSQL-backed RecordRepository,
// storing BackupRecord rows in the `backupdr_records` table.
type PostgresRecordRepository struct {
	exec persistence.Executor
}

// NewPostgresRecordRepository builds a PostgresRecordRepository bound
// to exec.
func NewPostgresRecordRepository(exec persistence.Executor) *PostgresRecordRepository {
	return &PostgresRecordRepository{exec: exec}
}

const recordColumns = `id, tenant_id, class, taken_at, location, reference, integrity_hash, size_bytes, encrypted, status, created_by, created_at`

func scanRecord(row rowScanner, rec *BackupRecord) error {
	return row.Scan(
		&rec.ID, &rec.TenantID, &rec.Class, &rec.TakenAt, &rec.Location, &rec.Reference,
		&rec.IntegrityHash, &rec.SizeBytes, &rec.Encrypted, &rec.Status, &rec.CreatedBy, &rec.CreatedAt,
	)
}

// Create implements RecordRepository.
func (r *PostgresRecordRepository) Create(ctx context.Context, tenantID uuid.UUID, rec *BackupRecord) error {
	if rec == nil {
		return ErrInvalidRecord
	}
	if rec.TenantID == uuid.Nil {
		rec.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rec.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO backupdr_records (id, tenant_id, class, taken_at, location, reference, integrity_hash, size_bytes, encrypted, status, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, COALESCE(NULLIF($12, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + recordColumns

	row := r.exec.QueryRow(ctx, q, rec.ID, rec.TenantID, rec.Class, rec.TakenAt, rec.Location, rec.Reference,
		rec.IntegrityHash, rec.SizeBytes, rec.Encrypted, rec.Status, rec.CreatedBy, rec.CreatedAt)
	if err := scanRecord(row, rec); err != nil {
		return wrapf("PostgresRecordRepository.Create", err)
	}
	return nil
}

// Get implements RecordRepository.
func (r *PostgresRecordRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*BackupRecord, error) {
	q := `SELECT ` + recordColumns + ` FROM backupdr_records WHERE id = $1 AND tenant_id = $2`
	rec := &BackupRecord{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanRecord(row, rec); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, wrapf("PostgresRecordRepository.Get", err)
	}
	return rec, nil
}

// ListForClass implements RecordRepository.
func (r *PostgresRecordRepository) ListForClass(ctx context.Context, tenantID uuid.UUID, class DataClass) ([]BackupRecord, error) {
	q := `SELECT ` + recordColumns + ` FROM backupdr_records WHERE tenant_id = $1 AND class = $2 ORDER BY taken_at ASC`
	return r.queryRecords(ctx, q, tenantID, class)
}

// ListAll implements RecordRepository.
func (r *PostgresRecordRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]BackupRecord, error) {
	q := `SELECT ` + recordColumns + ` FROM backupdr_records WHERE tenant_id = $1 ORDER BY taken_at ASC`
	return r.queryRecords(ctx, q, tenantID)
}

func (r *PostgresRecordRepository) queryRecords(ctx context.Context, q string, args ...any) ([]BackupRecord, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRecordRepository.query", err)
	}
	defer rows.Close()

	out := make([]BackupRecord, 0)
	for rows.Next() {
		var rec BackupRecord
		if err := scanRecord(rows, &rec); err != nil {
			return nil, wrapf("PostgresRecordRepository.query", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRecordRepository.query", err)
	}
	return out, nil
}

var _ RecordRepository = (*PostgresRecordRepository)(nil)

// PostgresDrillRepository is a PostgreSQL-backed DrillRepository,
// storing RestoreDrill rows in the `backupdr_drills` table.
type PostgresDrillRepository struct {
	exec persistence.Executor
}

// NewPostgresDrillRepository builds a PostgresDrillRepository bound to
// exec.
func NewPostgresDrillRepository(exec persistence.Executor) *PostgresDrillRepository {
	return &PostgresDrillRepository{exec: exec}
}

const drillColumns = `id, tenant_id, class, record_id, executed_at, executor, outcome, duration_ns, notes, created_at`

func scanDrill(row rowScanner, d *RestoreDrill) error {
	var durationNS int64
	if err := row.Scan(
		&d.ID, &d.TenantID, &d.Class, &d.RecordID, &d.ExecutedAt, &d.Executor,
		&d.Outcome, &durationNS, &d.Notes, &d.CreatedAt,
	); err != nil {
		return err
	}
	d.Duration = time.Duration(durationNS)
	return nil
}

// Create implements DrillRepository.
func (r *PostgresDrillRepository) Create(ctx context.Context, tenantID uuid.UUID, d *RestoreDrill) error {
	if d == nil {
		return ErrInvalidDrill
	}
	if d.TenantID == uuid.Nil {
		d.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, d.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO backupdr_drills (id, tenant_id, class, record_id, executed_at, executor, outcome, duration_ns, notes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + drillColumns

	row := r.exec.QueryRow(ctx, q, d.ID, d.TenantID, d.Class, d.RecordID, d.ExecutedAt, d.Executor,
		d.Outcome, int64(d.Duration), d.Notes, d.CreatedAt)
	if err := scanDrill(row, d); err != nil {
		return wrapf("PostgresDrillRepository.Create", err)
	}
	return nil
}

// Get implements DrillRepository.
func (r *PostgresDrillRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*RestoreDrill, error) {
	q := `SELECT ` + drillColumns + ` FROM backupdr_drills WHERE id = $1 AND tenant_id = $2`
	d := &RestoreDrill{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanDrill(row, d); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDrillNotFound
		}
		return nil, wrapf("PostgresDrillRepository.Get", err)
	}
	return d, nil
}

// ListForClass implements DrillRepository.
func (r *PostgresDrillRepository) ListForClass(ctx context.Context, tenantID uuid.UUID, class DataClass) ([]RestoreDrill, error) {
	q := `SELECT ` + drillColumns + ` FROM backupdr_drills WHERE tenant_id = $1 AND class = $2 ORDER BY executed_at ASC`
	return r.queryDrills(ctx, q, tenantID, class)
}

// ListAll implements DrillRepository.
func (r *PostgresDrillRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]RestoreDrill, error) {
	q := `SELECT ` + drillColumns + ` FROM backupdr_drills WHERE tenant_id = $1 ORDER BY executed_at ASC`
	return r.queryDrills(ctx, q, tenantID)
}

func (r *PostgresDrillRepository) queryDrills(ctx context.Context, q string, args ...any) ([]RestoreDrill, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresDrillRepository.query", err)
	}
	defer rows.Close()

	out := make([]RestoreDrill, 0)
	for rows.Next() {
		var d RestoreDrill
		if err := scanDrill(rows, &d); err != nil {
			return nil, wrapf("PostgresDrillRepository.query", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresDrillRepository.query", err)
	}
	return out, nil
}

var _ DrillRepository = (*PostgresDrillRepository)(nil)

// PostgresTargetRepository is a PostgreSQL-backed TargetRepository,
// storing Target rows in the `backupdr_targets` table.
type PostgresTargetRepository struct {
	exec persistence.Executor
}

// NewPostgresTargetRepository builds a PostgresTargetRepository bound
// to exec.
func NewPostgresTargetRepository(exec persistence.Executor) *PostgresTargetRepository {
	return &PostgresTargetRepository{exec: exec}
}

const targetColumns = `tenant_id, class, rpo_seconds, rto_seconds, created_at, updated_at`

func scanTarget(row rowScanner, t *Target) error {
	var rpoSeconds, rtoSeconds int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&t.TenantID, &t.Class, &rpoSeconds, &rtoSeconds, &createdAt, &updatedAt); err != nil {
		return err
	}
	t.RPO = time.Duration(rpoSeconds) * time.Second
	t.RTO = time.Duration(rtoSeconds) * time.Second
	return nil
}

// Set implements TargetRepository, upserting on the (tenant_id, class)
// primary key.
func (r *PostgresTargetRepository) Set(ctx context.Context, tenantID uuid.UUID, t *Target) error {
	if t == nil {
		return ErrInvalidTarget
	}
	if t.TenantID == uuid.Nil {
		t.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, t.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO backupdr_targets (tenant_id, class, rpo_seconds, rto_seconds, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
		ON CONFLICT (tenant_id, class) DO UPDATE SET
			rpo_seconds = EXCLUDED.rpo_seconds,
			rto_seconds = EXCLUDED.rto_seconds,
			updated_at = now()
		RETURNING ` + targetColumns

	row := r.exec.QueryRow(ctx, q, t.TenantID, t.Class, int64(t.RPO/time.Second), int64(t.RTO/time.Second))
	if err := scanTarget(row, t); err != nil {
		return wrapf("PostgresTargetRepository.Set", err)
	}
	return nil
}

// Get implements TargetRepository.
func (r *PostgresTargetRepository) Get(ctx context.Context, tenantID uuid.UUID, class DataClass) (*Target, error) {
	q := `SELECT ` + targetColumns + ` FROM backupdr_targets WHERE tenant_id = $1 AND class = $2`
	t := &Target{}
	row := r.exec.QueryRow(ctx, q, tenantID, class)
	if err := scanTarget(row, t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTargetNotFound
		}
		return nil, wrapf("PostgresTargetRepository.Get", err)
	}
	return t, nil
}

// ListAll implements TargetRepository.
func (r *PostgresTargetRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Target, error) {
	q := `SELECT ` + targetColumns + ` FROM backupdr_targets WHERE tenant_id = $1 ORDER BY class ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresTargetRepository.ListAll", err)
	}
	defer rows.Close()

	out := make([]Target, 0)
	for rows.Next() {
		var t Target
		if err := scanTarget(rows, &t); err != nil {
			return nil, wrapf("PostgresTargetRepository.ListAll", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresTargetRepository.ListAll", err)
	}
	return out, nil
}

var _ TargetRepository = (*PostgresTargetRepository)(nil)
