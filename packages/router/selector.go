package router

import (
	"context"
	"strings"
)

// ProviderSelector chooses an ordered list of provider IDs to try for a given
// task type and tenant.
type ProviderSelector interface {
	// Select returns an ordered slice of provider IDs to attempt.  The
	// caller tries them in order, skipping those whose circuit breaker is
	// open, until one succeeds.
	//
	// It returns ErrNoProvidersAvailable if no provider can be determined
	// for the combination of task and tenant.
	Select(ctx context.Context, task TaskType, tenantID string) ([]string, error)
}

// PolicySelector implements ProviderSelector using a RoutingPolicy.
type PolicySelector struct {
	policy RoutingPolicy
}

// NewPolicySelector creates a PolicySelector from the given policy.
func NewPolicySelector(policy RoutingPolicy) *PolicySelector {
	return &PolicySelector{policy: policy}
}

// Select returns the ordered provider chain for (task, tenantID).
//
// Resolution order:
//  1. Per-tenant override for this task type.
//  2. Global task route for this task type.
//  3. Global fallback chain.
//
// If AirGappedOnly is set, the result is filtered to providers whose ID
// starts with "local:".
func (s *PolicySelector) Select(_ context.Context, task TaskType, tenantID string) ([]string, error) {
	var chain []string

	// 1. Check per-tenant overrides.
	if overrides, ok := s.policy.TenantOverrides[tenantID]; ok {
		if taskChain, ok := overrides[task]; ok && len(taskChain) > 0 {
			chain = taskChain
		}
	}

	// 2. Fall back to global task route.
	if len(chain) == 0 {
		if taskChain, ok := s.policy.TaskRoutes[task]; ok && len(taskChain) > 0 {
			chain = taskChain
		}
	}

	// 3. Fall back to global fallback chain.
	if len(chain) == 0 {
		chain = s.policy.FallbackChain
	}

	// 4. Apply air-gapped filter.
	if s.policy.AirGappedOnly {
		chain = filterLocalProviders(chain)
		if len(chain) == 0 {
			return nil, ErrAirGappedViolation
		}
	}

	if len(chain) == 0 {
		return nil, ErrNoProvidersAvailable
	}

	// Return a defensive copy so the caller cannot mutate policy internals.
	out := make([]string, len(chain))
	copy(out, chain)
	return out, nil
}

// filterLocalProviders returns only those IDs that begin with "local:".
func filterLocalProviders(ids []string) []string {
	var local []string
	for _, id := range ids {
		if strings.HasPrefix(id, "local:") {
			local = append(local, id)
		}
	}
	return local
}
