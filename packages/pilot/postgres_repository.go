package pilot

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
// helpers depend on, mirroring packages/compliance.rowScanner and
// packages/vulnmanagement's equivalent exactly.
type rowScanner interface {
	Scan(dest ...any) error
}

// PostgresDeploymentRepository is a PostgreSQL-backed
// DeploymentRepository, storing PilotDeployment rows in the
// `pilot_deployments` table (see
// packages/persistence/migrations/000044_create_pilot.up.sql). It
// accepts a persistence.Executor per call, mirroring
// packages/compliance.PostgresEvidenceRepository exactly.
type PostgresDeploymentRepository struct {
	exec persistence.Executor
}

// NewPostgresDeploymentRepository builds a PostgresDeploymentRepository
// bound to exec.
func NewPostgresDeploymentRepository(exec persistence.Executor) *PostgresDeploymentRepository {
	return &PostgresDeploymentRepository{exec: exec}
}

const deploymentColumns = `id, tenant_id, name, jurisdiction_code, status, start_date, end_date, created_by, created_at, updated_at`

func scanDeployment(row rowScanner, d *PilotDeployment) error {
	var endDate *time.Time
	if err := row.Scan(
		&d.ID, &d.TenantID, &d.Name, &d.JurisdictionCode, &d.Status,
		&d.StartDate, &endDate, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return err
	}
	if endDate != nil {
		d.EndDate = *endDate
	}
	return nil
}

// Create implements DeploymentRepository.
func (r *PostgresDeploymentRepository) Create(ctx context.Context, tenantID uuid.UUID, d *PilotDeployment) error {
	if d == nil {
		return ErrInvalidDeployment
	}
	if d.TenantID == uuid.Nil {
		d.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, d.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO pilot_deployments (id, tenant_id, name, jurisdiction_code, status, start_date, end_date, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + deploymentColumns

	var endDate *time.Time
	if !d.EndDate.IsZero() {
		endDate = &d.EndDate
	}
	row := r.exec.QueryRow(ctx, q, d.ID, d.TenantID, d.Name, d.JurisdictionCode, d.Status,
		d.StartDate, endDate, d.CreatedBy, d.CreatedAt, d.UpdatedAt)
	if err := scanDeployment(row, d); err != nil {
		return wrapf("PostgresDeploymentRepository.Create", err)
	}
	return nil
}

// Get implements DeploymentRepository.
func (r *PostgresDeploymentRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotDeployment, error) {
	q := `SELECT ` + deploymentColumns + ` FROM pilot_deployments WHERE id = $1 AND tenant_id = $2`
	d := &PilotDeployment{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanDeployment(row, d); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeploymentNotFound
		}
		return nil, wrapf("PostgresDeploymentRepository.Get", err)
	}
	return d, nil
}

// ListAll implements DeploymentRepository.
func (r *PostgresDeploymentRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]PilotDeployment, error) {
	q := `SELECT ` + deploymentColumns + ` FROM pilot_deployments WHERE tenant_id = $1 ORDER BY created_at ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresDeploymentRepository.ListAll", err)
	}
	defer rows.Close()

	out := make([]PilotDeployment, 0)
	for rows.Next() {
		var d PilotDeployment
		if err := scanDeployment(rows, &d); err != nil {
			return nil, wrapf("PostgresDeploymentRepository.ListAll", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresDeploymentRepository.ListAll", err)
	}
	return out, nil
}

// Update implements DeploymentRepository.
func (r *PostgresDeploymentRepository) Update(ctx context.Context, tenantID uuid.UUID, d *PilotDeployment) error {
	if d == nil {
		return ErrInvalidDeployment
	}
	const q = `
		UPDATE pilot_deployments
		SET name = $1, jurisdiction_code = $2, status = $3, start_date = $4, end_date = $5, updated_at = now()
		WHERE id = $6 AND tenant_id = $7`

	var endDate *time.Time
	if !d.EndDate.IsZero() {
		endDate = &d.EndDate
	}
	tag, err := r.exec.Exec(ctx, q, d.Name, d.JurisdictionCode, d.Status, d.StartDate, endDate, d.ID, tenantID)
	if err != nil {
		return wrapf("PostgresDeploymentRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDeploymentNotFound
	}
	return nil
}

var _ DeploymentRepository = (*PostgresDeploymentRepository)(nil)

// PostgresCaseRepository is a PostgreSQL-backed CaseRepository,
// storing PilotCase rows in the `pilot_cases` table.
type PostgresCaseRepository struct {
	exec persistence.Executor
}

// NewPostgresCaseRepository builds a PostgresCaseRepository bound to
// exec.
func NewPostgresCaseRepository(exec persistence.Executor) *PostgresCaseRepository {
	return &PostgresCaseRepository{exec: exec}
}

const caseColumns = `id, tenant_id, deployment_id, case_id, supervisor_user_id, outcome_observed, assigned_at, observed_at, created_at, updated_at`

func scanCase(row rowScanner, c *PilotCase) error {
	return row.Scan(
		&c.ID, &c.TenantID, &c.DeploymentID, &c.CaseID, &c.SupervisorUserID,
		&c.OutcomeObserved, &c.AssignedAt, &c.ObservedAt, &c.CreatedAt, &c.UpdatedAt,
	)
}

// Create implements CaseRepository.
func (r *PostgresCaseRepository) Create(ctx context.Context, tenantID uuid.UUID, c *PilotCase) error {
	if c == nil {
		return ErrInvalidCase
	}
	if c.TenantID == uuid.Nil {
		c.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, c.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO pilot_cases (id, tenant_id, deployment_id, case_id, supervisor_user_id, outcome_observed, assigned_at, observed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + caseColumns

	row := r.exec.QueryRow(ctx, q, c.ID, c.TenantID, c.DeploymentID, c.CaseID, c.SupervisorUserID,
		c.OutcomeObserved, c.AssignedAt, c.ObservedAt, c.CreatedAt, c.UpdatedAt)
	if err := scanCase(row, c); err != nil {
		return wrapf("PostgresCaseRepository.Create", err)
	}
	return nil
}

// Get implements CaseRepository.
func (r *PostgresCaseRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotCase, error) {
	q := `SELECT ` + caseColumns + ` FROM pilot_cases WHERE id = $1 AND tenant_id = $2`
	c := &PilotCase{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanCase(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCaseNotFound
		}
		return nil, wrapf("PostgresCaseRepository.Get", err)
	}
	return c, nil
}

// ListForDeployment implements CaseRepository.
func (r *PostgresCaseRepository) ListForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]PilotCase, error) {
	q := `SELECT ` + caseColumns + ` FROM pilot_cases WHERE tenant_id = $1 AND deployment_id = $2 ORDER BY assigned_at ASC`
	rows, err := r.exec.Query(ctx, q, tenantID, deploymentID)
	if err != nil {
		return nil, wrapf("PostgresCaseRepository.ListForDeployment", err)
	}
	defer rows.Close()

	out := make([]PilotCase, 0)
	for rows.Next() {
		var c PilotCase
		if err := scanCase(rows, &c); err != nil {
			return nil, wrapf("PostgresCaseRepository.ListForDeployment", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresCaseRepository.ListForDeployment", err)
	}
	return out, nil
}

// Update implements CaseRepository.
func (r *PostgresCaseRepository) Update(ctx context.Context, tenantID uuid.UUID, c *PilotCase) error {
	if c == nil {
		return ErrInvalidCase
	}
	const q = `
		UPDATE pilot_cases
		SET supervisor_user_id = $1, outcome_observed = $2, observed_at = $3, updated_at = now()
		WHERE id = $4 AND tenant_id = $5`

	tag, err := r.exec.Exec(ctx, q, c.SupervisorUserID, c.OutcomeObserved, c.ObservedAt, c.ID, tenantID)
	if err != nil {
		return wrapf("PostgresCaseRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCaseNotFound
	}
	return nil
}

var _ CaseRepository = (*PostgresCaseRepository)(nil)

// PostgresFeedbackRepository is a PostgreSQL-backed FeedbackRepository,
// storing FeedbackEntry rows in the `pilot_feedback_entries` table.
type PostgresFeedbackRepository struct {
	exec persistence.Executor
}

// NewPostgresFeedbackRepository builds a PostgresFeedbackRepository
// bound to exec.
func NewPostgresFeedbackRepository(exec persistence.Executor) *PostgresFeedbackRepository {
	return &PostgresFeedbackRepository{exec: exec}
}

const feedbackColumns = `id, tenant_id, pilot_case_id, reviewer_user_id, ratings, trust, comments, submitted_at, created_at, updated_at`

func scanFeedback(row rowScanner, f *FeedbackEntry) error {
	var ratingsJSON []byte
	if err := row.Scan(
		&f.ID, &f.TenantID, &f.PilotCaseID, &f.ReviewerUserID, &ratingsJSON,
		&f.Trust, &f.Comments, &f.SubmittedAt, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return err
	}
	if len(ratingsJSON) > 0 {
		if err := json.Unmarshal(ratingsJSON, &f.Ratings); err != nil {
			return err
		}
	}
	return nil
}

// Create implements FeedbackRepository.
func (r *PostgresFeedbackRepository) Create(ctx context.Context, tenantID uuid.UUID, f *FeedbackEntry) error {
	if f == nil {
		return ErrInvalidFeedback
	}
	if f.TenantID == uuid.Nil {
		f.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, f.TenantID); err != nil {
		return err
	}
	ratingsJSON, err := json.Marshal(f.Ratings)
	if err != nil {
		return wrapf("PostgresFeedbackRepository.Create", err)
	}

	q := `
		INSERT INTO pilot_feedback_entries (id, tenant_id, pilot_case_id, reviewer_user_id, ratings, trust, comments, submitted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE(NULLIF($9, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + feedbackColumns

	row := r.exec.QueryRow(ctx, q, f.ID, f.TenantID, f.PilotCaseID, f.ReviewerUserID, ratingsJSON,
		f.Trust, f.Comments, f.SubmittedAt, f.CreatedAt, f.UpdatedAt)
	if err := scanFeedback(row, f); err != nil {
		return wrapf("PostgresFeedbackRepository.Create", err)
	}
	return nil
}

// Get implements FeedbackRepository.
func (r *PostgresFeedbackRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*FeedbackEntry, error) {
	q := `SELECT ` + feedbackColumns + ` FROM pilot_feedback_entries WHERE id = $1 AND tenant_id = $2`
	f := &FeedbackEntry{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanFeedback(row, f); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFeedbackNotFound
		}
		return nil, wrapf("PostgresFeedbackRepository.Get", err)
	}
	return f, nil
}

// ListForCase implements FeedbackRepository.
func (r *PostgresFeedbackRepository) ListForCase(ctx context.Context, tenantID, pilotCaseID uuid.UUID) ([]FeedbackEntry, error) {
	q := `SELECT ` + feedbackColumns + ` FROM pilot_feedback_entries WHERE tenant_id = $1 AND pilot_case_id = $2 ORDER BY submitted_at ASC`
	return r.queryFeedback(ctx, q, tenantID, pilotCaseID)
}

// ListForDeployment implements FeedbackRepository.
func (r *PostgresFeedbackRepository) ListForDeployment(ctx context.Context, tenantID uuid.UUID, pilotCaseIDs []uuid.UUID) ([]FeedbackEntry, error) {
	if len(pilotCaseIDs) == 0 {
		return []FeedbackEntry{}, nil
	}
	q := `SELECT ` + feedbackColumns + ` FROM pilot_feedback_entries WHERE tenant_id = $1 AND pilot_case_id = ANY($2) ORDER BY submitted_at ASC`
	return r.queryFeedback(ctx, q, tenantID, pilotCaseIDs)
}

// ListAll implements FeedbackRepository.
func (r *PostgresFeedbackRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]FeedbackEntry, error) {
	q := `SELECT ` + feedbackColumns + ` FROM pilot_feedback_entries WHERE tenant_id = $1 ORDER BY submitted_at ASC`
	return r.queryFeedback(ctx, q, tenantID)
}

func (r *PostgresFeedbackRepository) queryFeedback(ctx context.Context, q string, args ...any) ([]FeedbackEntry, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresFeedbackRepository.query", err)
	}
	defer rows.Close()

	out := make([]FeedbackEntry, 0)
	for rows.Next() {
		var f FeedbackEntry
		if err := scanFeedback(rows, &f); err != nil {
			return nil, wrapf("PostgresFeedbackRepository.query", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresFeedbackRepository.query", err)
	}
	return out, nil
}

var _ FeedbackRepository = (*PostgresFeedbackRepository)(nil)

// PostgresFindingRepository is a PostgreSQL-backed FindingRepository,
// storing PilotFinding rows in the `pilot_findings` table.
type PostgresFindingRepository struct {
	exec persistence.Executor
}

// NewPostgresFindingRepository builds a PostgresFindingRepository
// bound to exec.
func NewPostgresFindingRepository(exec persistence.Executor) *PostgresFindingRepository {
	return &PostgresFindingRepository{exec: exec}
}

const findingColumns = `id, tenant_id, deployment_id, source_feedback_ids, title, description, priority, status, triage_notes, triaged_by, triaged_at, discovered_at, created_at, updated_at`

func scanFinding(row rowScanner, f *PilotFinding) error {
	var sourceIDsJSON []byte
	if err := row.Scan(
		&f.ID, &f.TenantID, &f.DeploymentID, &sourceIDsJSON, &f.Title, &f.Description,
		&f.Priority, &f.Status, &f.TriageNotes, &f.TriagedBy, &f.TriagedAt,
		&f.DiscoveredAt, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return err
	}
	if len(sourceIDsJSON) > 0 {
		if err := json.Unmarshal(sourceIDsJSON, &f.SourceFeedbackIDs); err != nil {
			return err
		}
	}
	return nil
}

// Create implements FindingRepository.
func (r *PostgresFindingRepository) Create(ctx context.Context, tenantID uuid.UUID, f *PilotFinding) error {
	if f == nil {
		return ErrInvalidFinding
	}
	if f.TenantID == uuid.Nil {
		f.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, f.TenantID); err != nil {
		return err
	}
	sourceIDsJSON, err := json.Marshal(f.SourceFeedbackIDs)
	if err != nil {
		return wrapf("PostgresFindingRepository.Create", err)
	}

	q := `
		INSERT INTO pilot_findings (id, tenant_id, deployment_id, source_feedback_ids, title, description, priority, status, triage_notes, triaged_by, triaged_at, discovered_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE(NULLIF($13, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($14, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + findingColumns

	row := r.exec.QueryRow(ctx, q, f.ID, f.TenantID, f.DeploymentID, sourceIDsJSON, f.Title, f.Description,
		f.Priority, f.Status, f.TriageNotes, f.TriagedBy, f.TriagedAt, f.DiscoveredAt, f.CreatedAt, f.UpdatedAt)
	if err := scanFinding(row, f); err != nil {
		return wrapf("PostgresFindingRepository.Create", err)
	}
	return nil
}

// Get implements FindingRepository.
func (r *PostgresFindingRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotFinding, error) {
	q := `SELECT ` + findingColumns + ` FROM pilot_findings WHERE id = $1 AND tenant_id = $2`
	f := &PilotFinding{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanFinding(row, f); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFindingNotFound
		}
		return nil, wrapf("PostgresFindingRepository.Get", err)
	}
	return f, nil
}

// ListForDeployment implements FindingRepository.
func (r *PostgresFindingRepository) ListForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]PilotFinding, error) {
	q := `SELECT ` + findingColumns + ` FROM pilot_findings WHERE tenant_id = $1 AND deployment_id = $2 ORDER BY discovered_at ASC`
	rows, err := r.exec.Query(ctx, q, tenantID, deploymentID)
	if err != nil {
		return nil, wrapf("PostgresFindingRepository.ListForDeployment", err)
	}
	defer rows.Close()

	out := make([]PilotFinding, 0)
	for rows.Next() {
		var f PilotFinding
		if err := scanFinding(rows, &f); err != nil {
			return nil, wrapf("PostgresFindingRepository.ListForDeployment", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresFindingRepository.ListForDeployment", err)
	}
	return out, nil
}

// Update implements FindingRepository.
func (r *PostgresFindingRepository) Update(ctx context.Context, tenantID uuid.UUID, f *PilotFinding) error {
	if f == nil {
		return ErrInvalidFinding
	}
	const q = `
		UPDATE pilot_findings
		SET title = $1, description = $2, priority = $3, status = $4, triage_notes = $5, triaged_by = $6, triaged_at = $7, updated_at = now()
		WHERE id = $8 AND tenant_id = $9`

	tag, err := r.exec.Exec(ctx, q, f.Title, f.Description, f.Priority, f.Status, f.TriageNotes,
		f.TriagedBy, f.TriagedAt, f.ID, tenantID)
	if err != nil {
		return wrapf("PostgresFindingRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFindingNotFound
	}
	return nil
}

var _ FindingRepository = (*PostgresFindingRepository)(nil)

// PostgresRefinementRepository is a PostgreSQL-backed
// RefinementRepository, storing RefinementRecord rows in the
// `pilot_refinement_records` table.
type PostgresRefinementRepository struct {
	exec persistence.Executor
}

// NewPostgresRefinementRepository builds a PostgresRefinementRepository
// bound to exec.
func NewPostgresRefinementRepository(exec persistence.Executor) *PostgresRefinementRepository {
	return &PostgresRefinementRepository{exec: exec}
}

const refinementColumns = `id, tenant_id, finding_id, description, verified_fixed, verification_note, applied_by, applied_at, verified_by, verified_at, created_at, updated_at`

func scanRefinement(row rowScanner, r *RefinementRecord) error {
	return row.Scan(
		&r.ID, &r.TenantID, &r.FindingID, &r.Description, &r.VerifiedFixed, &r.VerificationNote,
		&r.AppliedBy, &r.AppliedAt, &r.VerifiedBy, &r.VerifiedAt, &r.CreatedAt, &r.UpdatedAt,
	)
}

// Create implements RefinementRepository.
func (r *PostgresRefinementRepository) Create(ctx context.Context, tenantID uuid.UUID, rec *RefinementRecord) error {
	if rec == nil {
		return ErrInvalidRefinement
	}
	if rec.TenantID == uuid.Nil {
		rec.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rec.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO pilot_refinement_records (id, tenant_id, finding_id, description, verified_fixed, verification_note, applied_by, applied_at, verified_by, verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE(NULLIF($11, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($12, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + refinementColumns

	row := r.exec.QueryRow(ctx, q, rec.ID, rec.TenantID, rec.FindingID, rec.Description, rec.VerifiedFixed,
		rec.VerificationNote, rec.AppliedBy, rec.AppliedAt, rec.VerifiedBy, rec.VerifiedAt, rec.CreatedAt, rec.UpdatedAt)
	if err := scanRefinement(row, rec); err != nil {
		return wrapf("PostgresRefinementRepository.Create", err)
	}
	return nil
}

// Get implements RefinementRepository.
func (r *PostgresRefinementRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*RefinementRecord, error) {
	q := `SELECT ` + refinementColumns + ` FROM pilot_refinement_records WHERE id = $1 AND tenant_id = $2`
	rec := &RefinementRecord{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanRefinement(row, rec); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefinementNotFound
		}
		return nil, wrapf("PostgresRefinementRepository.Get", err)
	}
	return rec, nil
}

// ListForFinding implements RefinementRepository.
func (r *PostgresRefinementRepository) ListForFinding(ctx context.Context, tenantID, findingID uuid.UUID) ([]RefinementRecord, error) {
	q := `SELECT ` + refinementColumns + ` FROM pilot_refinement_records WHERE tenant_id = $1 AND finding_id = $2 ORDER BY applied_at ASC`
	return r.queryRefinements(ctx, q, tenantID, findingID)
}

// ListForDeployment implements RefinementRepository.
func (r *PostgresRefinementRepository) ListForDeployment(ctx context.Context, tenantID uuid.UUID, findingIDs []uuid.UUID) ([]RefinementRecord, error) {
	if len(findingIDs) == 0 {
		return []RefinementRecord{}, nil
	}
	q := `SELECT ` + refinementColumns + ` FROM pilot_refinement_records WHERE tenant_id = $1 AND finding_id = ANY($2) ORDER BY applied_at ASC`
	return r.queryRefinements(ctx, q, tenantID, findingIDs)
}

func (r *PostgresRefinementRepository) queryRefinements(ctx context.Context, q string, args ...any) ([]RefinementRecord, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRefinementRepository.query", err)
	}
	defer rows.Close()

	out := make([]RefinementRecord, 0)
	for rows.Next() {
		var rec RefinementRecord
		if err := scanRefinement(rows, &rec); err != nil {
			return nil, wrapf("PostgresRefinementRepository.query", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRefinementRepository.query", err)
	}
	return out, nil
}

// Update implements RefinementRepository.
func (r *PostgresRefinementRepository) Update(ctx context.Context, tenantID uuid.UUID, rec *RefinementRecord) error {
	if rec == nil {
		return ErrInvalidRefinement
	}
	const q = `
		UPDATE pilot_refinement_records
		SET description = $1, verified_fixed = $2, verification_note = $3, verified_by = $4, verified_at = $5, updated_at = now()
		WHERE id = $6 AND tenant_id = $7`

	tag, err := r.exec.Exec(ctx, q, rec.Description, rec.VerifiedFixed, rec.VerificationNote,
		rec.VerifiedBy, rec.VerifiedAt, rec.ID, tenantID)
	if err != nil {
		return wrapf("PostgresRefinementRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRefinementNotFound
	}
	return nil
}

var _ RefinementRepository = (*PostgresRefinementRepository)(nil)
