// Package router provides LLM provider routing for the Verdex platform.
//
// It selects which provider(s) to call for a given task type, applies
// per-tenant routing overrides, enforces circuit breakers on failing
// providers, and supports air-gapped (local-only) mode.
//
// Basic usage:
//
//	policy := router.DefaultPolicy()
//	cfg := router.RouterConfig{
//	    Registry: providerRegistry,
//	    Policy:   policy,
//	}
//	r, err := router.NewRouter(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	resp, err := r.Chat(ctx, tenantID, chatReq)
package router
