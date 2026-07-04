package privacy

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
// helpers depend on, mirroring
// packages/accessgovernance.rowScanner exactly.
type rowScanner interface {
	Scan(dest ...any) error
}

// PostgresInventoryRepository is a PostgreSQL-backed
// InventoryRepository, storing DataInventoryEntry rows in the
// `privacy_data_inventory` table (see
// packages/persistence/migrations/000024_create_privacy.up.sql). It
// accepts a persistence.Executor per call, mirroring
// packages/accessgovernance.PostgresPolicyRepository exactly, so
// callers can run it directly against a pool or compose it inside a
// transaction via persistence.WithTx or
// packages/tenancy.WithTenantScope.
type PostgresInventoryRepository struct {
	exec persistence.Executor
}

// NewPostgresInventoryRepository builds a PostgresInventoryRepository
// bound to exec.
func NewPostgresInventoryRepository(exec persistence.Executor) *PostgresInventoryRepository {
	return &PostgresInventoryRepository{exec: exec}
}

const inventoryColumns = `id, tenant_id, category, source_tag, sensitivity, legal_basis, retention_period_seconds, description, created_by, created_at, updated_at`

func scanInventoryEntry(row rowScanner, e *DataInventoryEntry) error {
	var retentionSeconds int64
	if err := row.Scan(
		&e.ID, &e.TenantID, &e.Category, &e.SourceTag, &e.Sensitivity, &e.LegalBasis,
		&retentionSeconds, &e.Description, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return err
	}
	e.RetentionPeriod = time.Duration(retentionSeconds) * time.Second
	return nil
}

// Create implements InventoryRepository.
func (r *PostgresInventoryRepository) Create(ctx context.Context, tenantID uuid.UUID, e *DataInventoryEntry) error {
	if e == nil {
		return ErrInvalidInventoryEntry
	}
	if e.TenantID == uuid.Nil {
		e.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, e.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO privacy_data_inventory (id, tenant_id, category, source_tag, sensitivity, legal_basis, retention_period_seconds, description, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($11, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + inventoryColumns

	row := r.exec.QueryRow(ctx, q, e.ID, e.TenantID, e.Category, e.SourceTag, e.Sensitivity, e.LegalBasis,
		int64(e.RetentionPeriod/time.Second), e.Description, e.CreatedBy, e.CreatedAt, e.UpdatedAt)
	if err := scanInventoryEntry(row, e); err != nil {
		return wrapf("PostgresInventoryRepository.Create", err)
	}
	return nil
}

// Get implements InventoryRepository.
func (r *PostgresInventoryRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*DataInventoryEntry, error) {
	q := `SELECT ` + inventoryColumns + ` FROM privacy_data_inventory WHERE id = $1 AND tenant_id = $2`
	e := &DataInventoryEntry{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanInventoryEntry(row, e); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInventoryEntryNotFound
		}
		return nil, wrapf("PostgresInventoryRepository.Get", err)
	}
	return e, nil
}

// List implements InventoryRepository.
func (r *PostgresInventoryRepository) List(ctx context.Context, tenantID uuid.UUID) ([]DataInventoryEntry, error) {
	q := `SELECT ` + inventoryColumns + ` FROM privacy_data_inventory WHERE tenant_id = $1 ORDER BY created_at ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresInventoryRepository.List", err)
	}
	defer rows.Close()

	out := make([]DataInventoryEntry, 0)
	for rows.Next() {
		var e DataInventoryEntry
		if err := scanInventoryEntry(rows, &e); err != nil {
			return nil, wrapf("PostgresInventoryRepository.List", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresInventoryRepository.List", err)
	}
	return out, nil
}

// Update implements InventoryRepository.
func (r *PostgresInventoryRepository) Update(ctx context.Context, tenantID uuid.UUID, e *DataInventoryEntry) error {
	if e == nil {
		return ErrInvalidInventoryEntry
	}
	const q = `
		UPDATE privacy_data_inventory
		SET category = $1, source_tag = $2, sensitivity = $3, legal_basis = $4,
		    retention_period_seconds = $5, description = $6, updated_at = now()
		WHERE id = $7 AND tenant_id = $8`

	tag, err := r.exec.Exec(ctx, q, e.Category, e.SourceTag, e.Sensitivity, e.LegalBasis,
		int64(e.RetentionPeriod/time.Second), e.Description, e.ID, tenantID)
	if err != nil {
		return wrapf("PostgresInventoryRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInventoryEntryNotFound
	}
	return nil
}

var _ InventoryRepository = (*PostgresInventoryRepository)(nil)

// PostgresConsentRepository is a PostgreSQL-backed ConsentRepository,
// storing ConsentRecord rows in the `privacy_consent_records` table.
type PostgresConsentRepository struct {
	exec persistence.Executor
}

// NewPostgresConsentRepository builds a PostgresConsentRepository
// bound to exec.
func NewPostgresConsentRepository(exec persistence.Executor) *PostgresConsentRepository {
	return &PostgresConsentRepository{exec: exec}
}

const consentColumns = `id, tenant_id, subject_id, purpose, legal_basis, granted_at, withdrawn_at, recorded_by, notes`

func scanConsentRecord(row rowScanner, c *ConsentRecord) error {
	return row.Scan(&c.ID, &c.TenantID, &c.SubjectID, &c.Purpose, &c.LegalBasis, &c.GrantedAt, &c.WithdrawnAt, &c.RecordedBy, &c.Notes)
}

// Create implements ConsentRepository.
func (r *PostgresConsentRepository) Create(ctx context.Context, tenantID uuid.UUID, c *ConsentRecord) error {
	if c == nil {
		return ErrInvalidConsentRecord
	}
	if c.TenantID == uuid.Nil {
		c.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, c.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO privacy_consent_records (id, tenant_id, subject_id, purpose, legal_basis, granted_at, withdrawn_at, recorded_by, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + consentColumns

	row := r.exec.QueryRow(ctx, q, c.ID, c.TenantID, c.SubjectID, c.Purpose, c.LegalBasis, c.GrantedAt, c.WithdrawnAt, c.RecordedBy, c.Notes)
	if err := scanConsentRecord(row, c); err != nil {
		return wrapf("PostgresConsentRepository.Create", err)
	}
	return nil
}

// Get implements ConsentRepository.
func (r *PostgresConsentRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ConsentRecord, error) {
	q := `SELECT ` + consentColumns + ` FROM privacy_consent_records WHERE id = $1 AND tenant_id = $2`
	c := &ConsentRecord{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanConsentRecord(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConsentNotFound
		}
		return nil, wrapf("PostgresConsentRepository.Get", err)
	}
	return c, nil
}

// ListForSubject implements ConsentRepository.
func (r *PostgresConsentRepository) ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]ConsentRecord, error) {
	q := `SELECT ` + consentColumns + ` FROM privacy_consent_records WHERE tenant_id = $1 AND subject_id = $2`
	return r.queryConsentRecords(ctx, q, tenantID, subjectID)
}

// ListAll implements ConsentRepository.
func (r *PostgresConsentRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ConsentRecord, error) {
	q := `SELECT ` + consentColumns + ` FROM privacy_consent_records WHERE tenant_id = $1`
	return r.queryConsentRecords(ctx, q, tenantID)
}

func (r *PostgresConsentRepository) queryConsentRecords(ctx context.Context, q string, args ...any) ([]ConsentRecord, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresConsentRepository.query", err)
	}
	defer rows.Close()

	out := make([]ConsentRecord, 0)
	for rows.Next() {
		var c ConsentRecord
		if err := scanConsentRecord(rows, &c); err != nil {
			return nil, wrapf("PostgresConsentRepository.query", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresConsentRepository.query", err)
	}
	return out, nil
}

// Update implements ConsentRepository.
func (r *PostgresConsentRepository) Update(ctx context.Context, tenantID uuid.UUID, c *ConsentRecord) error {
	if c == nil {
		return ErrInvalidConsentRecord
	}
	const q = `UPDATE privacy_consent_records SET withdrawn_at = $1, notes = $2 WHERE id = $3 AND tenant_id = $4`
	tag, err := r.exec.Exec(ctx, q, c.WithdrawnAt, c.Notes, c.ID, tenantID)
	if err != nil {
		return wrapf("PostgresConsentRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConsentNotFound
	}
	return nil
}

var _ ConsentRepository = (*PostgresConsentRepository)(nil)

// PostgresSARRepository is a PostgreSQL-backed SARRepository, storing
// SubjectAccessRequest rows in the
// `privacy_subject_access_requests` table.
type PostgresSARRepository struct {
	exec persistence.Executor
}

// NewPostgresSARRepository builds a PostgresSARRepository bound to
// exec.
func NewPostgresSARRepository(exec persistence.Executor) *PostgresSARRepository {
	return &PostgresSARRepository{exec: exec}
}

const sarColumns = `id, tenant_id, subject_id, case_refs, status, received_at, due_at, resolved_at, resolution_notes, handled_by, created_at, updated_at`

func scanSAR(row rowScanner, s *SubjectAccessRequest) error {
	var caseRefsJSON []byte
	var handledBy uuid.NullUUID
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.SubjectID, &caseRefsJSON, &s.Status,
		&s.ReceivedAt, &s.DueAt, &s.ResolvedAt, &s.ResolutionNotes, &handledBy,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return err
	}
	if handledBy.Valid {
		s.HandledBy = handledBy.UUID
	}
	if len(caseRefsJSON) > 0 {
		if err := json.Unmarshal(caseRefsJSON, &s.CaseRefs); err != nil {
			return err
		}
	}
	return nil
}

// Create implements SARRepository.
func (r *PostgresSARRepository) Create(ctx context.Context, tenantID uuid.UUID, s *SubjectAccessRequest) error {
	if s == nil {
		return ErrInvalidSAR
	}
	if s.TenantID == uuid.Nil {
		s.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, s.TenantID); err != nil {
		return err
	}
	caseRefsJSON, err := json.Marshal(s.CaseRefs)
	if err != nil {
		return wrapf("PostgresSARRepository.Create", err)
	}

	q := `
		INSERT INTO privacy_subject_access_requests (id, tenant_id, subject_id, case_refs, status, received_at, due_at, resolved_at, resolution_notes, handled_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE(NULLIF($11, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($12, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + sarColumns

	nullHandledBy := uuid.NullUUID{UUID: s.HandledBy, Valid: s.HandledBy != uuid.Nil}
	row := r.exec.QueryRow(ctx, q, s.ID, s.TenantID, s.SubjectID, caseRefsJSON, s.Status,
		s.ReceivedAt, s.DueAt, s.ResolvedAt, s.ResolutionNotes, nullHandledBy, s.CreatedAt, s.UpdatedAt)
	if err := scanSAR(row, s); err != nil {
		return wrapf("PostgresSARRepository.Create", err)
	}
	return nil
}

// Get implements SARRepository.
func (r *PostgresSARRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*SubjectAccessRequest, error) {
	q := `SELECT ` + sarColumns + ` FROM privacy_subject_access_requests WHERE id = $1 AND tenant_id = $2`
	s := &SubjectAccessRequest{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanSAR(row, s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSARNotFound
		}
		return nil, wrapf("PostgresSARRepository.Get", err)
	}
	return s, nil
}

// ListForSubject implements SARRepository.
func (r *PostgresSARRepository) ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]SubjectAccessRequest, error) {
	q := `SELECT ` + sarColumns + ` FROM privacy_subject_access_requests WHERE tenant_id = $1 AND subject_id = $2`
	return r.querySARs(ctx, q, tenantID, subjectID)
}

// ListAll implements SARRepository.
func (r *PostgresSARRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]SubjectAccessRequest, error) {
	q := `SELECT ` + sarColumns + ` FROM privacy_subject_access_requests WHERE tenant_id = $1`
	return r.querySARs(ctx, q, tenantID)
}

func (r *PostgresSARRepository) querySARs(ctx context.Context, q string, args ...any) ([]SubjectAccessRequest, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresSARRepository.query", err)
	}
	defer rows.Close()

	out := make([]SubjectAccessRequest, 0)
	for rows.Next() {
		var s SubjectAccessRequest
		if err := scanSAR(rows, &s); err != nil {
			return nil, wrapf("PostgresSARRepository.query", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresSARRepository.query", err)
	}
	return out, nil
}

// Update implements SARRepository.
func (r *PostgresSARRepository) Update(ctx context.Context, tenantID uuid.UUID, s *SubjectAccessRequest) error {
	if s == nil {
		return ErrInvalidSAR
	}
	const q = `
		UPDATE privacy_subject_access_requests
		SET status = $1, resolved_at = $2, resolution_notes = $3, handled_by = $4, updated_at = now()
		WHERE id = $5 AND tenant_id = $6`

	nullHandledBy := uuid.NullUUID{UUID: s.HandledBy, Valid: s.HandledBy != uuid.Nil}
	tag, err := r.exec.Exec(ctx, q, s.Status, s.ResolvedAt, s.ResolutionNotes, nullHandledBy, s.ID, tenantID)
	if err != nil {
		return wrapf("PostgresSARRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSARNotFound
	}
	return nil
}

var _ SARRepository = (*PostgresSARRepository)(nil)

// PostgresErasureRepository is a PostgreSQL-backed ErasureRepository,
// storing ErasureRequest rows in the `privacy_erasure_requests`
// table.
type PostgresErasureRepository struct {
	exec persistence.Executor
}

// NewPostgresErasureRepository builds a PostgresErasureRepository
// bound to exec.
func NewPostgresErasureRepository(exec persistence.Executor) *PostgresErasureRepository {
	return &PostgresErasureRepository{exec: exec}
}

const erasureColumns = `id, tenant_id, subject_id, category, source_tag, record_ref, provenance_record_id, provenance_hash, status, requested_at, resolved_at, resolution_notes, handled_by, created_at, updated_at`

func scanErasureRequest(row rowScanner, req *ErasureRequest) error {
	var provenanceRecordID uuid.NullUUID
	var handledBy uuid.NullUUID
	if err := row.Scan(
		&req.ID, &req.TenantID, &req.SubjectID, &req.Category, &req.SourceTag, &req.RecordRef,
		&provenanceRecordID, &req.ProvenanceHash, &req.Status, &req.RequestedAt, &req.ResolvedAt,
		&req.ResolutionNotes, &handledBy, &req.CreatedAt, &req.UpdatedAt,
	); err != nil {
		return err
	}
	if provenanceRecordID.Valid {
		req.ProvenanceRecordID = provenanceRecordID.UUID
	}
	if handledBy.Valid {
		req.HandledBy = handledBy.UUID
	}
	return nil
}

// Create implements ErasureRepository.
func (r *PostgresErasureRepository) Create(ctx context.Context, tenantID uuid.UUID, req *ErasureRequest) error {
	if req == nil {
		return ErrInvalidErasureRequest
	}
	if req.TenantID == uuid.Nil {
		req.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, req.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO privacy_erasure_requests (id, tenant_id, subject_id, category, source_tag, record_ref, provenance_record_id, provenance_hash, status, requested_at, resolved_at, resolution_notes, handled_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, COALESCE(NULLIF($14, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($15, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + erasureColumns

	nullProvenanceID := uuid.NullUUID{UUID: req.ProvenanceRecordID, Valid: req.ProvenanceRecordID != uuid.Nil}
	nullHandledBy := uuid.NullUUID{UUID: req.HandledBy, Valid: req.HandledBy != uuid.Nil}
	row := r.exec.QueryRow(ctx, q, req.ID, req.TenantID, req.SubjectID, req.Category, req.SourceTag, req.RecordRef,
		nullProvenanceID, req.ProvenanceHash, req.Status, req.RequestedAt, req.ResolvedAt, req.ResolutionNotes,
		nullHandledBy, req.CreatedAt, req.UpdatedAt)
	if err := scanErasureRequest(row, req); err != nil {
		return wrapf("PostgresErasureRepository.Create", err)
	}
	return nil
}

// Get implements ErasureRepository.
func (r *PostgresErasureRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ErasureRequest, error) {
	q := `SELECT ` + erasureColumns + ` FROM privacy_erasure_requests WHERE id = $1 AND tenant_id = $2`
	req := &ErasureRequest{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanErasureRequest(row, req); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrErasureNotFound
		}
		return nil, wrapf("PostgresErasureRepository.Get", err)
	}
	return req, nil
}

// ListForSubject implements ErasureRepository.
func (r *PostgresErasureRepository) ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]ErasureRequest, error) {
	q := `SELECT ` + erasureColumns + ` FROM privacy_erasure_requests WHERE tenant_id = $1 AND subject_id = $2`
	return r.queryErasureRequests(ctx, q, tenantID, subjectID)
}

// ListAll implements ErasureRepository.
func (r *PostgresErasureRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ErasureRequest, error) {
	q := `SELECT ` + erasureColumns + ` FROM privacy_erasure_requests WHERE tenant_id = $1`
	return r.queryErasureRequests(ctx, q, tenantID)
}

func (r *PostgresErasureRepository) queryErasureRequests(ctx context.Context, q string, args ...any) ([]ErasureRequest, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresErasureRepository.query", err)
	}
	defer rows.Close()

	out := make([]ErasureRequest, 0)
	for rows.Next() {
		var req ErasureRequest
		if err := scanErasureRequest(rows, &req); err != nil {
			return nil, wrapf("PostgresErasureRepository.query", err)
		}
		out = append(out, req)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresErasureRepository.query", err)
	}
	return out, nil
}

// Update implements ErasureRepository.
func (r *PostgresErasureRepository) Update(ctx context.Context, tenantID uuid.UUID, req *ErasureRequest) error {
	if req == nil {
		return ErrInvalidErasureRequest
	}
	const q = `
		UPDATE privacy_erasure_requests
		SET status = $1, resolved_at = $2, resolution_notes = $3, handled_by = $4, updated_at = now()
		WHERE id = $5 AND tenant_id = $6`

	nullHandledBy := uuid.NullUUID{UUID: req.HandledBy, Valid: req.HandledBy != uuid.Nil}
	tag, err := r.exec.Exec(ctx, q, req.Status, req.ResolvedAt, req.ResolutionNotes, nullHandledBy, req.ID, tenantID)
	if err != nil {
		return wrapf("PostgresErasureRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrErasureNotFound
	}
	return nil
}

var _ ErasureRepository = (*PostgresErasureRepository)(nil)
