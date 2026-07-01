package router

import (
	"context"
	"strings"
)

// LocalOnlyEnforcer wraps another ProviderSelector and filters its output to
// only providers whose ID begins with "local:" when the policy has
// AirGappedOnly set.
//
// If the policy does not require air-gapped mode the inner selector's result
// is returned unchanged.
type LocalOnlyEnforcer struct {
	inner  ProviderSelector
	policy RoutingPolicy
}

// NewLocalOnlyEnforcer wraps inner with an air-gapped filter driven by policy.
func NewLocalOnlyEnforcer(inner ProviderSelector, policy RoutingPolicy) *LocalOnlyEnforcer {
	return &LocalOnlyEnforcer{inner: inner, policy: policy}
}

// Select delegates to the inner selector and, when AirGappedOnly is true,
// removes any provider whose ID does not start with "local:".  It returns
// ErrAirGappedViolation if filtering removes all candidates.
func (e *LocalOnlyEnforcer) Select(ctx context.Context, task TaskType, tenantID string) ([]string, error) {
	ids, err := e.inner.Select(ctx, task, tenantID)
	if err != nil {
		return nil, err
	}

	if !e.policy.AirGappedOnly {
		return ids, nil
	}

	var local []string
	for _, id := range ids {
		if strings.HasPrefix(id, "local:") {
			local = append(local, id)
		}
	}

	if len(local) == 0 {
		return nil, ErrAirGappedViolation
	}
	return local, nil
}
