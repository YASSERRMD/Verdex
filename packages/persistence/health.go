package persistence

import (
	"context"
	"fmt"
)

// Checker matches packages/observability's health-check function
// signature (observability.Checker is `func(ctx context.Context)
// error`); it is defined structurally here rather than imported so
// this package does not need to depend on packages/observability just
// for a function type. Any *persistence.Postgres health check can be
// passed directly as an observability.NamedChecker.Checker.
type Checker func(ctx context.Context) error

// PostgresChecker returns a Checker that pings pg. It reports
// unhealthy if pg is nil or its pool is uninitialized, and otherwise
// defers to Postgres.Ping, which honors ctx cancellation/timeout.
func PostgresChecker(pg *Postgres) Checker {
	return func(ctx context.Context) error {
		if pg == nil {
			return fmt.Errorf("persistence: PostgresChecker: postgres is not initialized")
		}
		if err := pg.Ping(ctx); err != nil {
			return fmt.Errorf("persistence: PostgresChecker: %w", err)
		}
		return nil
	}
}
