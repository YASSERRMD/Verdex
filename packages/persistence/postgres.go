package persistence

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/encryption"
)

// Postgres wraps a pooled PostgreSQL connection and owns its
// lifecycle: construction from Verdex config, context-aware
// liveness checks, and shutdown.
type Postgres struct {
	pool *pgxpool.Pool
}

// Open builds a connection pool from cfg.Database and verifies
// connectivity with a single ping before returning. The returned
// *Postgres must be closed with Close when no longer needed.
//
// Pool sizing and connection lifetime come from cfg.Database:
// MaxOpenConns bounds the pool's maximum size, MaxIdleConns is used
// as the pool's minimum warm size, and ConnMaxLifetime bounds how
// long any single connection is reused before being recycled.
//
// If cfg.Database.RequireTLS is true, Open first verifies (via
// packages/encryption.AssertEncryptedAtRest) that the DSN itself
// requests an encrypted connection -- e.g. Postgres's sslmode=require
// or stronger -- and refuses to open a pool otherwise. RequireTLS
// defaults to false so that local-development and test DSNs (which
// commonly use sslmode=disable against a loopback database, see
// integration_test.go) keep working unmodified; a production
// deployment profile is expected to set it true.
func Open(ctx context.Context, cfg *config.Config) (*Postgres, error) {
	if cfg == nil {
		return nil, fmt.Errorf("persistence: Open: cfg must not be nil")
	}
	if cfg.Database.DSN == "" {
		return nil, fmt.Errorf("persistence: Open: cfg.Database.DSN must not be empty")
	}
	if cfg.Database.RequireTLS {
		atRestCfg := encryption.AtRestConfig{EncryptedAtRest: true, RequireTLSInTransit: true}
		if err := encryption.AssertEncryptedAtRest(atRestCfg, cfg.Database.DSN); err != nil {
			return nil, fmt.Errorf("persistence: Open: %w", err)
		}
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("persistence: Open: parse DSN: %w", err)
	}

	if cfg.Database.MaxOpenConns > 0 && cfg.Database.MaxOpenConns <= math.MaxInt32 {
		poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns > 0 && cfg.Database.MaxIdleConns <= math.MaxInt32 {
		poolCfg.MinConns = int32(cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("persistence: Open: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("persistence: Open: ping: %w", err)
	}

	return &Postgres{pool: pool}, nil
}

// Pool returns the underlying pgxpool.Pool for callers (repositories,
// migration drivers, health checkers) that need direct access.
func (p *Postgres) Pool() *pgxpool.Pool {
	return p.pool
}

// Ping verifies the pool can reach the database, honoring ctx
// cancellation/timeout.
func (p *Postgres) Ping(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("persistence: Ping: pool is not initialized")
	}
	return p.pool.Ping(ctx)
}

// Close releases all pooled connections. It is safe to call on a nil
// receiver or an already-closed pool.
func (p *Postgres) Close() {
	if p == nil || p.pool == nil {
		return
	}
	p.pool.Close()
}
