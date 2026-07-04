package alerting

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedAlertRuleRepository composes
// packages/tenancy.WithTenantScope with PostgresAlertRuleRepository,
// exactly as packages/compliance.TenantScopedEvidenceRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresAlertRuleRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedAlertRuleRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedAlertRuleRepository builds a
// TenantScopedAlertRuleRepository backed by pool.
func NewTenantScopedAlertRuleRepository(pool *pgxpool.Pool) *TenantScopedAlertRuleRepository {
	return &TenantScopedAlertRuleRepository{pool: pool}
}

func (r *TenantScopedAlertRuleRepository) Create(ctx context.Context, tenantID uuid.UUID, rule *AlertRule) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresAlertRuleRepository(exec).Create(ctx, tenantID, rule)
	})
}

func (r *TenantScopedAlertRuleRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*AlertRule, error) {
	var out *AlertRule
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		rule, err := NewPostgresAlertRuleRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = rule
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedAlertRuleRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*AlertRule, error) {
	var out *AlertRule
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		rule, err := NewPostgresAlertRuleRepository(exec).GetByName(ctx, tenantID, name)
		if err != nil {
			return err
		}
		out = rule
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedAlertRuleRepository) List(ctx context.Context, tenantID uuid.UUID) ([]AlertRule, error) {
	var out []AlertRule
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresAlertRuleRepository(exec).List(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedAlertRuleRepository) Update(ctx context.Context, tenantID uuid.UUID, rule *AlertRule) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresAlertRuleRepository(exec).Update(ctx, tenantID, rule)
	})
}

func (r *TenantScopedAlertRuleRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresAlertRuleRepository(exec).Delete(ctx, tenantID, id)
	})
}

var _ AlertRuleRepository = (*TenantScopedAlertRuleRepository)(nil)

// TenantScopedAlertEventRepository composes
// packages/tenancy.WithTenantScope with PostgresAlertEventRepository.
type TenantScopedAlertEventRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedAlertEventRepository builds a
// TenantScopedAlertEventRepository backed by pool.
func NewTenantScopedAlertEventRepository(pool *pgxpool.Pool) *TenantScopedAlertEventRepository {
	return &TenantScopedAlertEventRepository{pool: pool}
}

func (r *TenantScopedAlertEventRepository) Create(ctx context.Context, tenantID uuid.UUID, e *AlertEvent) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresAlertEventRepository(exec).Create(ctx, tenantID, e)
	})
}

func (r *TenantScopedAlertEventRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*AlertEvent, error) {
	var out *AlertEvent
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		e, err := NewPostgresAlertEventRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedAlertEventRepository) ListForRule(ctx context.Context, tenantID, ruleID uuid.UUID) ([]AlertEvent, error) {
	var out []AlertEvent
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresAlertEventRepository(exec).ListForRule(ctx, tenantID, ruleID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedAlertEventRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]AlertEvent, error) {
	var out []AlertEvent
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresAlertEventRepository(exec).ListAll(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ AlertEventRepository = (*TenantScopedAlertEventRepository)(nil)

// TenantScopedEscalationPolicyRepository composes
// packages/tenancy.WithTenantScope with
// PostgresEscalationPolicyRepository.
type TenantScopedEscalationPolicyRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedEscalationPolicyRepository builds a
// TenantScopedEscalationPolicyRepository backed by pool.
func NewTenantScopedEscalationPolicyRepository(pool *pgxpool.Pool) *TenantScopedEscalationPolicyRepository {
	return &TenantScopedEscalationPolicyRepository{pool: pool}
}

func (r *TenantScopedEscalationPolicyRepository) Set(ctx context.Context, tenantID uuid.UUID, p *EscalationPolicy) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresEscalationPolicyRepository(exec).Set(ctx, tenantID, p)
	})
}

func (r *TenantScopedEscalationPolicyRepository) Get(ctx context.Context, tenantID uuid.UUID, name string) (*EscalationPolicy, error) {
	var out *EscalationPolicy
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		p, err := NewPostgresEscalationPolicyRepository(exec).Get(ctx, tenantID, name)
		if err != nil {
			return err
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedEscalationPolicyRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]EscalationPolicy, error) {
	var out []EscalationPolicy
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresEscalationPolicyRepository(exec).ListAll(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ EscalationPolicyRepository = (*TenantScopedEscalationPolicyRepository)(nil)
