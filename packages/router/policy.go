package router

import (
	"errors"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// TaskType is re-exported from the provider package for convenience so
// callers of this package do not need to import provider directly just for
// task classification.
type TaskType = provider.TaskType

const (
	TaskChat    = provider.TaskChat
	TaskEmbed   = provider.TaskEmbed
	TaskReason  = provider.TaskReason
	TaskExtract = provider.TaskExtract
)

// RoutingPolicy describes how the router selects providers for each task type.
type RoutingPolicy struct {
	// TaskRoutes maps a TaskType to an ordered list of provider IDs.  The
	// router tries each provider in order, skipping those whose circuit
	// breaker is open.
	TaskRoutes map[TaskType][]string

	// FallbackChain is the ordered list of provider IDs to try when no
	// task-specific route matches (or when all task-specific providers fail).
	FallbackChain []string

	// TenantOverrides allows per-tenant customisation of task routes.  The
	// outer key is the tenant ID; the inner map mirrors TaskRoutes.
	TenantOverrides map[string]map[TaskType][]string

	// AirGappedOnly, when true, restricts routing to providers whose IDs
	// begin with the "local:" prefix.
	AirGappedOnly bool

	// MaxCostPerRequest, when non-nil, causes the router to return
	// ErrBudgetExceeded if the estimated cost of a request would exceed
	// this value (in USD).  Currently informational — enforcement is done
	// by the caller.
	MaxCostPerRequest *float64
}

// DefaultPolicy returns a sensible zero-configuration RoutingPolicy that
// routes all task types to an empty chain and imposes no budget limit.
func DefaultPolicy() RoutingPolicy {
	return RoutingPolicy{
		TaskRoutes:      make(map[TaskType][]string),
		FallbackChain:   []string{},
		TenantOverrides: make(map[string]map[TaskType][]string),
	}
}

// Validate checks the policy for obvious configuration errors.
func (p RoutingPolicy) Validate() error {
	var errs []error

	for task, chain := range p.TaskRoutes {
		if len(chain) == 0 {
			errs = append(errs, fmt.Errorf("task route for %q has an empty provider chain", task))
		}
		for i, id := range chain {
			if id == "" {
				errs = append(errs, fmt.Errorf("task route for %q: provider ID at index %d is empty", task, i))
			}
		}
	}

	for i, id := range p.FallbackChain {
		if id == "" {
			errs = append(errs, fmt.Errorf("fallback chain: provider ID at index %d is empty", i))
		}
	}

	for tenantID, overrides := range p.TenantOverrides {
		if tenantID == "" {
			errs = append(errs, errors.New("tenant override has an empty tenant ID key"))
		}
		for task, chain := range overrides {
			for i, id := range chain {
				if id == "" {
					errs = append(errs, fmt.Errorf("tenant %q override for task %q: provider ID at index %d is empty", tenantID, task, i))
				}
			}
		}
	}

	if p.MaxCostPerRequest != nil && *p.MaxCostPerRequest < 0 {
		errs = append(errs, errors.New("MaxCostPerRequest must be non-negative"))
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
