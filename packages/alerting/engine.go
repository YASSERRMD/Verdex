package alerting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Engine is the alerting orchestrator: it composes the AlertRule
// catalogue, fired AlertEvent history, and EscalationPolicy storage
// into one set of tenant- and permission-scoped operations, mirroring
// packages/compliance.Engine's and packages/backupdr.Engine's shape --
// authenticate, check tenant match, check permission, mutate/evaluate,
// persist the result.
type Engine struct {
	rules    AlertRuleRepository
	events   AlertEventRepository
	policies EscalationPolicyRepository
	clock    func() time.Time
	idFunc   func() uuid.UUID
}

// NewEngine builds an Engine from its dependencies. rules, events, and
// policies must be non-nil (ErrNilStore).
func NewEngine(rules AlertRuleRepository, events AlertEventRepository, policies EscalationPolicyRepository) (*Engine, error) {
	if rules == nil || events == nil || policies == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		rules:    rules,
		events:   events,
		policies: policies,
		clock:    time.Now,
		idFunc:   uuid.New,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

func (e *Engine) newID() uuid.UUID {
	if e.idFunc != nil {
		return e.idFunc()
	}
	return uuid.New()
}

// RegisterRule catalogues a new AlertRule for tenantID, requiring
// managePermission. Returns ErrDuplicateRule if a rule with the same
// Name is already registered for this tenant.
func (e *Engine) RegisterRule(ctx context.Context, tenantID uuid.UUID, rule AlertRule) (AlertRule, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return AlertRule{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return AlertRule{}, err
	}
	if err := rule.Validate(); err != nil {
		return AlertRule{}, wrapf("RegisterRule", err)
	}

	rule.ID = e.newID()
	rule.TenantID = tenantID
	rule.CreatedBy = user.ID
	rule.CreatedAt = e.now()
	rule.UpdatedAt = rule.CreatedAt

	if err := e.rules.Create(ctx, tenantID, &rule); err != nil {
		return AlertRule{}, wrapf("RegisterRule", err)
	}
	return rule, nil
}

// GetRule returns the AlertRule identified by id for tenantID,
// requiring viewPermission.
func (e *Engine) GetRule(ctx context.Context, tenantID, id uuid.UUID) (AlertRule, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return AlertRule{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return AlertRule{}, err
	}
	rule, err := e.rules.Get(ctx, tenantID, id)
	if err != nil {
		return AlertRule{}, wrapf("GetRule", err)
	}
	return *rule, nil
}

// ListRules returns every AlertRule registered for tenantID, requiring
// viewPermission.
func (e *Engine) ListRules(ctx context.Context, tenantID uuid.UUID) ([]AlertRule, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	rules, err := e.rules.List(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListRules", err)
	}
	return rules, nil
}

// UpdateRule replaces the stored AlertRule matching rule.ID for
// tenantID, requiring managePermission.
func (e *Engine) UpdateRule(ctx context.Context, tenantID uuid.UUID, rule AlertRule) (AlertRule, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return AlertRule{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return AlertRule{}, err
	}
	if err := rule.Validate(); err != nil {
		return AlertRule{}, wrapf("UpdateRule", err)
	}
	rule.TenantID = tenantID
	rule.UpdatedAt = e.now()
	if err := e.rules.Update(ctx, tenantID, &rule); err != nil {
		return AlertRule{}, wrapf("UpdateRule", err)
	}
	return rule, nil
}

// Evaluate checks rule's Condition against currentValue and, if the
// condition is met, persists and returns a populated AlertEvent (task
// 3-5's shared mechanism). Only ConditionThresholdAbove/Below are
// evaluated here -- the three externally-evaluated ConditionKinds
// (ConditionSLOBreached, ConditionQualityRegression,
// ConditionCostThreshold) are produced by this package's dedicated
// EvaluateSLOAlert/EvaluateQualityAlert/EvaluateCostAlert functions
// instead (slo_alert.go, quality_alert.go, cost_alert.go), since those
// three conditions are not "compare a number to a threshold" but
// "consult an already-computed domain signal" -- calling Evaluate with
// one of those three ConditionKinds returns ErrNilCondition.
//
// Returns (AlertEvent{}, false, nil) when the condition is not met --
// not firing is not an error.
func (e *Engine) Evaluate(ctx context.Context, tenantID uuid.UUID, rule AlertRule, currentValue float64) (AlertEvent, bool, error) {
	if err := rule.Validate(); err != nil {
		return AlertEvent{}, false, wrapf("Evaluate", err)
	}
	if rule.Condition.Kind.externallyEvaluated() {
		return AlertEvent{}, false, wrapf("Evaluate", ErrNilCondition)
	}

	fired, detail := evaluateThreshold(rule.Condition, currentValue)
	if !fired {
		return AlertEvent{}, false, nil
	}

	event := AlertEvent{
		ID:            e.newID(),
		TenantID:      tenantID,
		RuleID:        rule.ID,
		RuleName:      rule.Name,
		Severity:      rule.Severity,
		ConditionKind: rule.Condition.Kind,
		TriggerValue:  currentValue,
		Threshold:     rule.Condition.Threshold,
		Detail:        detail,
		CreatedAt:     e.now(),
	}

	if e.events != nil {
		if err := e.events.Create(ctx, tenantID, &event); err != nil {
			return AlertEvent{}, false, wrapf("Evaluate", err)
		}
	}
	return event, true, nil
}

// evaluateThreshold performs the actual threshold comparison for
// ConditionThresholdAbove/Below, returning whether it fired and a
// human-readable detail string. Any other ConditionKind (already
// excluded by Evaluate's externallyEvaluated check before this is
// reached) reports not-fired.
func evaluateThreshold(c Condition, currentValue float64) (fired bool, detail string) {
	switch c.Kind {
	case ConditionThresholdAbove:
		fired = currentValue > c.Threshold
		detail = fmt.Sprintf("%s = %.4f exceeds threshold %.4f", c.MetricName, currentValue, c.Threshold)
	case ConditionThresholdBelow:
		fired = currentValue < c.Threshold
		detail = fmt.Sprintf("%s = %.4f is below threshold %.4f", c.MetricName, currentValue, c.Threshold)
	default:
		return false, ""
	}
	if !fired {
		detail = ""
	}
	return fired, detail
}

// ListEvents returns every fired AlertEvent for tenantID, requiring
// viewPermission.
func (e *Engine) ListEvents(ctx context.Context, tenantID uuid.UUID) ([]AlertEvent, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	events, err := e.events.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListEvents", err)
	}
	return events, nil
}

// ListEventsForRule returns every fired AlertEvent for ruleID scoped
// to tenantID, requiring viewPermission.
func (e *Engine) ListEventsForRule(ctx context.Context, tenantID, ruleID uuid.UUID) ([]AlertEvent, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	events, err := e.events.ListForRule(ctx, tenantID, ruleID)
	if err != nil {
		return nil, wrapf("ListEventsForRule", err)
	}
	return events, nil
}

// SetPolicy upserts an EscalationPolicy for tenantID, requiring
// managePermission.
func (e *Engine) SetPolicy(ctx context.Context, tenantID uuid.UUID, policy EscalationPolicy) (EscalationPolicy, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return EscalationPolicy{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return EscalationPolicy{}, err
	}
	if err := policy.Validate(); err != nil {
		return EscalationPolicy{}, wrapf("SetPolicy", err)
	}
	policy.TenantID = tenantID
	policy.CreatedBy = user.ID
	now := e.now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now

	if err := e.policies.Set(ctx, tenantID, &policy); err != nil {
		return EscalationPolicy{}, wrapf("SetPolicy", err)
	}
	return policy, nil
}

// GetPolicy returns the named EscalationPolicy for tenantID, requiring
// viewPermission.
func (e *Engine) GetPolicy(ctx context.Context, tenantID uuid.UUID, name string) (EscalationPolicy, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return EscalationPolicy{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return EscalationPolicy{}, err
	}
	policy, err := e.policies.Get(ctx, tenantID, name)
	if err != nil {
		return EscalationPolicy{}, wrapf("GetPolicy", err)
	}
	return *policy, nil
}
