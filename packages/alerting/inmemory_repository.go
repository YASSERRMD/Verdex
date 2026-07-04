package alerting

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryAlertRuleRepository is a process-local AlertRuleRepository
// backed by a map guarded by a mutex, intended for tests and other
// packages' fixtures -- never for production use, mirroring
// packages/compliance.InMemoryControlRepository's role.
type InMemoryAlertRuleRepository struct {
	mu    sync.RWMutex
	rules map[uuid.UUID]*AlertRule
}

// NewInMemoryAlertRuleRepository builds an empty
// InMemoryAlertRuleRepository.
func NewInMemoryAlertRuleRepository() *InMemoryAlertRuleRepository {
	return &InMemoryAlertRuleRepository{rules: make(map[uuid.UUID]*AlertRule)}
}

// Create implements AlertRuleRepository. Returns ErrDuplicateRule if a
// rule with the same TenantID and Name already exists.
func (r *InMemoryAlertRuleRepository) Create(_ context.Context, tenantID uuid.UUID, rule *AlertRule) error {
	if rule == nil {
		return ErrInvalidRule
	}
	if rule.TenantID == uuid.Nil {
		rule.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rule.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.rules {
		if existing.TenantID == tenantID && existing.Name == rule.Name {
			return ErrDuplicateRule
		}
	}
	cp := *rule
	r.rules[rule.ID] = &cp
	return nil
}

// Get implements AlertRuleRepository.
func (r *InMemoryAlertRuleRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*AlertRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rule, ok := r.rules[id]
	if !ok || rule.TenantID != tenantID {
		return nil, ErrRuleNotFound
	}
	cp := *rule
	return &cp, nil
}

// GetByName implements AlertRuleRepository.
func (r *InMemoryAlertRuleRepository) GetByName(_ context.Context, tenantID uuid.UUID, name string) (*AlertRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rule := range r.rules {
		if rule.TenantID == tenantID && rule.Name == name {
			cp := *rule
			return &cp, nil
		}
	}
	return nil, ErrRuleNotFound
}

// List implements AlertRuleRepository.
func (r *InMemoryAlertRuleRepository) List(_ context.Context, tenantID uuid.UUID) ([]AlertRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AlertRule, 0)
	for _, rule := range r.rules {
		if rule.TenantID == tenantID {
			out = append(out, *rule)
		}
	}
	return out, nil
}

// Update implements AlertRuleRepository.
func (r *InMemoryAlertRuleRepository) Update(_ context.Context, tenantID uuid.UUID, rule *AlertRule) error {
	if rule == nil {
		return ErrInvalidRule
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.rules[rule.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrRuleNotFound
	}
	cp := *rule
	cp.TenantID = tenantID
	r.rules[rule.ID] = &cp
	return nil
}

// Delete implements AlertRuleRepository.
func (r *InMemoryAlertRuleRepository) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.rules[id]
	if !ok || existing.TenantID != tenantID {
		return ErrRuleNotFound
	}
	delete(r.rules, id)
	return nil
}

var _ AlertRuleRepository = (*InMemoryAlertRuleRepository)(nil)

// InMemoryAlertEventRepository is a process-local AlertEventRepository.
type InMemoryAlertEventRepository struct {
	mu     sync.RWMutex
	events map[uuid.UUID]*AlertEvent
}

// NewInMemoryAlertEventRepository builds an empty
// InMemoryAlertEventRepository.
func NewInMemoryAlertEventRepository() *InMemoryAlertEventRepository {
	return &InMemoryAlertEventRepository{events: make(map[uuid.UUID]*AlertEvent)}
}

// Create implements AlertEventRepository.
func (r *InMemoryAlertEventRepository) Create(_ context.Context, tenantID uuid.UUID, e *AlertEvent) error {
	if e == nil {
		return ErrInvalidEvent
	}
	if e.TenantID == uuid.Nil {
		e.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, e.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *e
	r.events[e.ID] = &cp
	return nil
}

// Get implements AlertEventRepository.
func (r *InMemoryAlertEventRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*AlertEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.events[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrEventNotFound
	}
	cp := *e
	return &cp, nil
}

// ListForRule implements AlertEventRepository.
func (r *InMemoryAlertEventRepository) ListForRule(_ context.Context, tenantID, ruleID uuid.UUID) ([]AlertEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AlertEvent, 0)
	for _, e := range r.events {
		if e.TenantID == tenantID && e.RuleID == ruleID {
			out = append(out, *e)
		}
	}
	return out, nil
}

// ListAll implements AlertEventRepository.
func (r *InMemoryAlertEventRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]AlertEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AlertEvent, 0)
	for _, e := range r.events {
		if e.TenantID == tenantID {
			out = append(out, *e)
		}
	}
	return out, nil
}

var _ AlertEventRepository = (*InMemoryAlertEventRepository)(nil)

// InMemoryEscalationPolicyRepository is a process-local
// EscalationPolicyRepository.
type InMemoryEscalationPolicyRepository struct {
	mu       sync.RWMutex
	policies map[uuid.UUID]map[string]*EscalationPolicy
}

// NewInMemoryEscalationPolicyRepository builds an empty
// InMemoryEscalationPolicyRepository.
func NewInMemoryEscalationPolicyRepository() *InMemoryEscalationPolicyRepository {
	return &InMemoryEscalationPolicyRepository{policies: make(map[uuid.UUID]map[string]*EscalationPolicy)}
}

// Set implements EscalationPolicyRepository, upserting by
// (tenantID, p.Name).
func (r *InMemoryEscalationPolicyRepository) Set(_ context.Context, tenantID uuid.UUID, p *EscalationPolicy) error {
	if p == nil {
		return ErrInvalidPolicy
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	byName, ok := r.policies[tenantID]
	if !ok {
		byName = make(map[string]*EscalationPolicy)
		r.policies[tenantID] = byName
	}
	cp := *p
	byName[p.Name] = &cp
	return nil
}

// Get implements EscalationPolicyRepository.
func (r *InMemoryEscalationPolicyRepository) Get(_ context.Context, tenantID uuid.UUID, name string) (*EscalationPolicy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	byName, ok := r.policies[tenantID]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	p, ok := byName[name]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	cp := *p
	return &cp, nil
}

// ListAll implements EscalationPolicyRepository.
func (r *InMemoryEscalationPolicyRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]EscalationPolicy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	byName, ok := r.policies[tenantID]
	if !ok {
		return []EscalationPolicy{}, nil
	}
	out := make([]EscalationPolicy, 0, len(byName))
	for _, p := range byName {
		out = append(out, *p)
	}
	return out, nil
}

var _ EscalationPolicyRepository = (*InMemoryEscalationPolicyRepository)(nil)
