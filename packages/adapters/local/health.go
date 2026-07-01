package local

import (
	"context"
	"fmt"
	"net/http"

	"github.com/YASSERRMD/verdex/packages/adapters/shared"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// HealthCheck verifies that the local endpoint is reachable by calling GET
// /v1/models (the lightest endpoint in the OpenAI-compatible spec).
//
// Behaviour:
//   - If the request succeeds (HTTP 2xx) the server is considered healthy.
//   - If the transport fails (connection refused, timeout) and
//     Config.OfflineMode is false, [ErrLocalEndpointDown] is returned.
//   - If Config.OfflineMode is true the same transport failure is still
//     reported as [ErrLocalEndpointDown] because an air-gapped deployment
//     must have its local server running at all times.
//
// GGUF model size awareness:
//   Quantised GGUF models (e.g. Q4_K_M, Q5_K_M) typically load fully into
//   RAM/VRAM before the server can serve requests. A health check issued
//   immediately after server start may fail while a large model (13 B+) is
//   still being memory-mapped. Callers should implement a retry loop with an
//   appropriate backoff (e.g. 5 s × 12 attempts for a 70 B Q4 model).
func (a *LocalAdapter) HealthCheck(ctx context.Context) error {
	body, status, err := shared.DoRequest(ctx, a.client, http.MethodGet,
		a.cfg.BaseURL+"/v1/models",
		a.headers(), nil)
	if err != nil {
		if !a.cfg.OfflineMode {
			return fmt.Errorf("%w: %v", ErrLocalEndpointDown, err)
		}
		return fmt.Errorf("%w: %v", ErrLocalEndpointDown, err)
	}
	if mapErr := shared.MapHTTPStatus(status, body, "local"); mapErr != nil {
		return &provider.ProviderError{
			ProviderID: a.ID(),
			Code:       fmt.Sprintf("http_%d", status),
			Message:    fmt.Sprintf("local health check failed with HTTP %d", status),
			Underlying: ErrLocalEndpointDown,
		}
	}
	return nil
}
