package router

import (
	"errors"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// RouterConfig holds all parameters needed to construct a Router.
type RouterConfig struct {
	// Registry is the provider registry the router will use to look up
	// LLMProvider instances by ID.  Required.
	Registry *provider.Registry

	// Policy defines routing rules, fallback chains, and budget limits.
	// A zero-value RoutingPolicy is valid (no routes, no fallbacks).
	Policy RoutingPolicy

	// Selector overrides the default PolicySelector.  When nil a
	// PolicySelector backed by Policy is used.
	Selector ProviderSelector

	// CBRegistry overrides the default CircuitBreakerRegistry.  When nil a
	// new empty registry is created.
	CBRegistry *CircuitBreakerRegistry

	// Telemetry overrides the default NoOpTelemetrySink.
	Telemetry TelemetrySink
}

// NewRouter validates cfg and returns a ready-to-use Router.
func NewRouter(cfg RouterConfig) (*Router, error) {
	if cfg.Registry == nil {
		return nil, errors.New("router: RouterConfig.Registry must not be nil")
	}

	if err := cfg.Policy.Validate(); err != nil {
		return nil, err
	}

	sel := cfg.Selector
	if sel == nil {
		base := NewPolicySelector(cfg.Policy)
		sel = NewLocalOnlyEnforcer(base, cfg.Policy)
	}

	cbReg := cfg.CBRegistry
	if cbReg == nil {
		cbReg = NewCircuitBreakerRegistry()
	}

	tel := cfg.Telemetry
	if tel == nil {
		tel = NoOpTelemetrySink{}
	}

	return &Router{
		registry:   cfg.Registry,
		policy:     cfg.Policy,
		selector:   sel,
		cbRegistry: cbReg,
		telemetry:  tel,
	}, nil
}
