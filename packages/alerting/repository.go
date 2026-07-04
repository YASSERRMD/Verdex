package alerting

import (
	"context"

	"github.com/google/uuid"
)

// AlertRuleRepository persists AlertRule catalogue rows, scoped to a
// tenant on every call, mirroring packages/compliance.EvidenceRepository's
// and packages/backupdr.PolicyRepository's conventions.
type AlertRuleRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, r *AlertRule) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*AlertRule, error)
	GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*AlertRule, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]AlertRule, error)
	Update(ctx context.Context, tenantID uuid.UUID, r *AlertRule) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// AlertEventRepository persists fired AlertEvent history, scoped to a
// tenant.
type AlertEventRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, e *AlertEvent) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*AlertEvent, error)
	ListForRule(ctx context.Context, tenantID, ruleID uuid.UUID) ([]AlertEvent, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]AlertEvent, error)
}

// EscalationPolicyRepository persists EscalationPolicy rows, scoped to
// a tenant.
type EscalationPolicyRepository interface {
	Set(ctx context.Context, tenantID uuid.UUID, p *EscalationPolicy) error
	Get(ctx context.Context, tenantID uuid.UUID, name string) (*EscalationPolicy, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]EscalationPolicy, error)
}
