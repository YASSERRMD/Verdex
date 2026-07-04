package alerting

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
// constraint violation (SQLSTATE 23505), the error
// alerting_rules_tenant_name_unique raises when Create collides with
// an existing rule name for the tenant, mirroring
// packages/compliance.isUniqueViolation exactly.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

// PostgresAlertRuleRepository is a PostgreSQL-backed
// AlertRuleRepository, storing AlertRule rows in the `alerting_rules`
// table (see
// packages/persistence/migrations/000042_create_alerting.up.sql). It
// accepts a persistence.Executor per call, mirroring
// packages/compliance.PostgresEvidenceRepository exactly.
type PostgresAlertRuleRepository struct {
	exec persistence.Executor
}

// NewPostgresAlertRuleRepository builds a PostgresAlertRuleRepository
// bound to exec.
func NewPostgresAlertRuleRepository(exec persistence.Executor) *PostgresAlertRuleRepository {
	return &PostgresAlertRuleRepository{exec: exec}
}

const alertRuleColumns = `id, tenant_id, name, description, condition_kind, metric_name, threshold, severity, runbook_name, created_by, created_at, updated_at`

func scanAlertRule(row rowScanner, r *AlertRule) error {
	return row.Scan(
		&r.ID, &r.TenantID, &r.Name, &r.Description, &r.Condition.Kind, &r.Condition.MetricName,
		&r.Condition.Threshold, &r.Severity, &r.RunbookName, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
}

// Create implements AlertRuleRepository.
func (r *PostgresAlertRuleRepository) Create(ctx context.Context, tenantID uuid.UUID, rule *AlertRule) error {
	if rule == nil {
		return ErrInvalidRule
	}
	if rule.TenantID == uuid.Nil {
		rule.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rule.TenantID); err != nil {
		return err
	}

	q := `
		INSERT INTO alerting_rules (id, tenant_id, name, description, condition_kind, metric_name, threshold, severity, runbook_name, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE(NULLIF($11, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($12, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + alertRuleColumns

	row := r.exec.QueryRow(ctx, q, rule.ID, rule.TenantID, rule.Name, rule.Description,
		rule.Condition.Kind, rule.Condition.MetricName, rule.Condition.Threshold, rule.Severity,
		rule.RunbookName, rule.CreatedBy, rule.CreatedAt, rule.UpdatedAt)
	if err := scanAlertRule(row, rule); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateRule
		}
		return wrapf("PostgresAlertRuleRepository.Create", err)
	}
	return nil
}

// Get implements AlertRuleRepository.
func (r *PostgresAlertRuleRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*AlertRule, error) {
	q := `SELECT ` + alertRuleColumns + ` FROM alerting_rules WHERE id = $1 AND tenant_id = $2`
	rule := &AlertRule{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanAlertRule(row, rule); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRuleNotFound
		}
		return nil, wrapf("PostgresAlertRuleRepository.Get", err)
	}
	return rule, nil
}

// GetByName implements AlertRuleRepository.
func (r *PostgresAlertRuleRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*AlertRule, error) {
	q := `SELECT ` + alertRuleColumns + ` FROM alerting_rules WHERE tenant_id = $1 AND name = $2`
	rule := &AlertRule{}
	row := r.exec.QueryRow(ctx, q, tenantID, name)
	if err := scanAlertRule(row, rule); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRuleNotFound
		}
		return nil, wrapf("PostgresAlertRuleRepository.GetByName", err)
	}
	return rule, nil
}

// List implements AlertRuleRepository.
func (r *PostgresAlertRuleRepository) List(ctx context.Context, tenantID uuid.UUID) ([]AlertRule, error) {
	q := `SELECT ` + alertRuleColumns + ` FROM alerting_rules WHERE tenant_id = $1 ORDER BY name ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresAlertRuleRepository.List", err)
	}
	defer rows.Close()

	out := make([]AlertRule, 0)
	for rows.Next() {
		var rule AlertRule
		if err := scanAlertRule(rows, &rule); err != nil {
			return nil, wrapf("PostgresAlertRuleRepository.List", err)
		}
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresAlertRuleRepository.List", err)
	}
	return out, nil
}

// Update implements AlertRuleRepository.
func (r *PostgresAlertRuleRepository) Update(ctx context.Context, tenantID uuid.UUID, rule *AlertRule) error {
	if rule == nil {
		return ErrInvalidRule
	}
	const q = `
		UPDATE alerting_rules
		SET description = $1, condition_kind = $2, metric_name = $3, threshold = $4,
		    severity = $5, runbook_name = $6, updated_at = now()
		WHERE id = $7 AND tenant_id = $8`

	tag, err := r.exec.Exec(ctx, q, rule.Description, rule.Condition.Kind, rule.Condition.MetricName,
		rule.Condition.Threshold, rule.Severity, rule.RunbookName, rule.ID, tenantID)
	if err != nil {
		return wrapf("PostgresAlertRuleRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRuleNotFound
	}
	return nil
}

// Delete implements AlertRuleRepository.
func (r *PostgresAlertRuleRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `DELETE FROM alerting_rules WHERE id = $1 AND tenant_id = $2`
	tag, err := r.exec.Exec(ctx, q, id, tenantID)
	if err != nil {
		return wrapf("PostgresAlertRuleRepository.Delete", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRuleNotFound
	}
	return nil
}

var _ AlertRuleRepository = (*PostgresAlertRuleRepository)(nil)

// PostgresAlertEventRepository is a PostgreSQL-backed
// AlertEventRepository, storing AlertEvent rows in the
// `alerting_events` table.
type PostgresAlertEventRepository struct {
	exec persistence.Executor
}

// NewPostgresAlertEventRepository builds a PostgresAlertEventRepository
// bound to exec.
func NewPostgresAlertEventRepository(exec persistence.Executor) *PostgresAlertEventRepository {
	return &PostgresAlertEventRepository{exec: exec}
}

const alertEventColumns = `id, tenant_id, rule_id, rule_name, severity, condition_kind, trigger_value, threshold, detail, created_at`

func scanAlertEvent(row rowScanner, e *AlertEvent) error {
	var ruleID *uuid.UUID
	if err := row.Scan(
		&e.ID, &e.TenantID, &ruleID, &e.RuleName, &e.Severity, &e.ConditionKind,
		&e.TriggerValue, &e.Threshold, &e.Detail, &e.CreatedAt,
	); err != nil {
		return err
	}
	if ruleID != nil {
		e.RuleID = *ruleID
	}
	return nil
}

// Create implements AlertEventRepository.
func (r *PostgresAlertEventRepository) Create(ctx context.Context, tenantID uuid.UUID, e *AlertEvent) error {
	if e == nil {
		return ErrInvalidEvent
	}
	if e.TenantID == uuid.Nil {
		e.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, e.TenantID); err != nil {
		return err
	}

	var ruleID *uuid.UUID
	if e.RuleID != uuid.Nil {
		ruleID = &e.RuleID
	}

	q := `
		INSERT INTO alerting_events (id, tenant_id, rule_id, rule_name, severity, condition_kind, trigger_value, threshold, detail, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF($10, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + alertEventColumns

	row := r.exec.QueryRow(ctx, q, e.ID, e.TenantID, ruleID, e.RuleName, e.Severity, e.ConditionKind,
		e.TriggerValue, e.Threshold, e.Detail, e.CreatedAt)
	if err := scanAlertEvent(row, e); err != nil {
		return wrapf("PostgresAlertEventRepository.Create", err)
	}
	return nil
}

// Get implements AlertEventRepository.
func (r *PostgresAlertEventRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*AlertEvent, error) {
	q := `SELECT ` + alertEventColumns + ` FROM alerting_events WHERE id = $1 AND tenant_id = $2`
	e := &AlertEvent{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanAlertEvent(row, e); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, wrapf("PostgresAlertEventRepository.Get", err)
	}
	return e, nil
}

// ListForRule implements AlertEventRepository.
func (r *PostgresAlertEventRepository) ListForRule(ctx context.Context, tenantID, ruleID uuid.UUID) ([]AlertEvent, error) {
	q := `SELECT ` + alertEventColumns + ` FROM alerting_events WHERE tenant_id = $1 AND rule_id = $2 ORDER BY created_at DESC`
	return r.queryEvents(ctx, q, tenantID, ruleID)
}

// ListAll implements AlertEventRepository.
func (r *PostgresAlertEventRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]AlertEvent, error) {
	q := `SELECT ` + alertEventColumns + ` FROM alerting_events WHERE tenant_id = $1 ORDER BY created_at DESC`
	return r.queryEvents(ctx, q, tenantID)
}

func (r *PostgresAlertEventRepository) queryEvents(ctx context.Context, q string, args ...any) ([]AlertEvent, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresAlertEventRepository.query", err)
	}
	defer rows.Close()

	out := make([]AlertEvent, 0)
	for rows.Next() {
		var e AlertEvent
		if err := scanAlertEvent(rows, &e); err != nil {
			return nil, wrapf("PostgresAlertEventRepository.query", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresAlertEventRepository.query", err)
	}
	return out, nil
}

var _ AlertEventRepository = (*PostgresAlertEventRepository)(nil)

// PostgresEscalationPolicyRepository is a PostgreSQL-backed
// EscalationPolicyRepository, storing EscalationPolicy rows in the
// `alerting_escalation_policies` table (one row per tenant/name pair).
type PostgresEscalationPolicyRepository struct {
	exec persistence.Executor
}

// NewPostgresEscalationPolicyRepository builds a
// PostgresEscalationPolicyRepository bound to exec.
func NewPostgresEscalationPolicyRepository(exec persistence.Executor) *PostgresEscalationPolicyRepository {
	return &PostgresEscalationPolicyRepository{exec: exec}
}

const escalationPolicyColumns = `tenant_id, name, min_severity, tiers, created_by, created_at, updated_at`

func scanEscalationPolicy(row rowScanner, p *EscalationPolicy) error {
	var tiersJSON []byte
	if err := row.Scan(&p.TenantID, &p.Name, &p.MinSeverity, &tiersJSON, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	if len(tiersJSON) > 0 {
		if err := json.Unmarshal(tiersJSON, &p.Tiers); err != nil {
			return err
		}
	}
	return nil
}

// Set implements EscalationPolicyRepository, upserting by
// (tenantID, p.Name).
func (r *PostgresEscalationPolicyRepository) Set(ctx context.Context, tenantID uuid.UUID, p *EscalationPolicy) error {
	if p == nil {
		return ErrInvalidPolicy
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	tiersJSON, err := json.Marshal(p.Tiers)
	if err != nil {
		return wrapf("PostgresEscalationPolicyRepository.Set", err)
	}

	q := `
		INSERT INTO alerting_escalation_policies (tenant_id, name, min_severity, tiers, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, TIMESTAMPTZ '0001-01-01'), now()), COALESCE(NULLIF($7, TIMESTAMPTZ '0001-01-01'), now()))
		ON CONFLICT (tenant_id, name) DO UPDATE SET
			min_severity = EXCLUDED.min_severity,
			tiers = EXCLUDED.tiers,
			updated_at = now()
		RETURNING ` + escalationPolicyColumns

	row := r.exec.QueryRow(ctx, q, tenantID, p.Name, p.MinSeverity, tiersJSON, p.CreatedBy, p.CreatedAt, p.UpdatedAt)
	if err := scanEscalationPolicy(row, p); err != nil {
		return wrapf("PostgresEscalationPolicyRepository.Set", err)
	}
	return nil
}

// Get implements EscalationPolicyRepository.
func (r *PostgresEscalationPolicyRepository) Get(ctx context.Context, tenantID uuid.UUID, name string) (*EscalationPolicy, error) {
	q := `SELECT ` + escalationPolicyColumns + ` FROM alerting_escalation_policies WHERE tenant_id = $1 AND name = $2`
	p := &EscalationPolicy{}
	row := r.exec.QueryRow(ctx, q, tenantID, name)
	if err := scanEscalationPolicy(row, p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, wrapf("PostgresEscalationPolicyRepository.Get", err)
	}
	return p, nil
}

// ListAll implements EscalationPolicyRepository.
func (r *PostgresEscalationPolicyRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]EscalationPolicy, error) {
	q := `SELECT ` + escalationPolicyColumns + ` FROM alerting_escalation_policies WHERE tenant_id = $1 ORDER BY name ASC`
	rows, err := r.exec.Query(ctx, q, tenantID)
	if err != nil {
		return nil, wrapf("PostgresEscalationPolicyRepository.ListAll", err)
	}
	defer rows.Close()

	out := make([]EscalationPolicy, 0)
	for rows.Next() {
		var p EscalationPolicy
		if err := scanEscalationPolicy(rows, &p); err != nil {
			return nil, wrapf("PostgresEscalationPolicyRepository.ListAll", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresEscalationPolicyRepository.ListAll", err)
	}
	return out, nil
}

var _ EscalationPolicyRepository = (*PostgresEscalationPolicyRepository)(nil)
