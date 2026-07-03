package persistence

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/config"
)

func TestOpen_NilConfig(t *testing.T) {
	t.Parallel()

	_, err := Open(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
}

func TestOpen_EmptyDSN(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	_, err := Open(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected error for empty DSN, got nil")
	}
}

func TestOpen_InvalidDSN(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Database.DSN = "not-a-valid-dsn://???"
	_, err := Open(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}
}

func TestOpen_RequireTLSRejectsSSLModeDisable(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Database.DSN = "postgres://user:pass@localhost:5432/verdex?sslmode=disable"
	cfg.Database.RequireTLS = true

	_, err := Open(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected error opening with RequireTLS=true and sslmode=disable, got nil")
	}
}

func TestOpen_RequireTLSRejectsMissingSSLMode(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Database.DSN = "postgres://user:pass@localhost:5432/verdex"
	cfg.Database.RequireTLS = true

	_, err := Open(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected error opening with RequireTLS=true and no sslmode parameter, got nil")
	}
}

func TestOpen_RequireTLSFalseAllowsSSLModeDisable(t *testing.T) {
	t.Parallel()

	// RequireTLS defaults to false, so a local-development DSN with
	// sslmode=disable must not be rejected by the TLS assertion itself
	// (it may still fail later trying to actually reach a database,
	// which is fine -- this test only asserts the TLS check is not
	// what fails it).
	cfg := config.Default()
	cfg.Database.DSN = "postgres://user:pass@127.0.0.1:1/verdex?sslmode=disable&connect_timeout=1"
	cfg.Database.RequireTLS = false

	_, err := Open(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected a connection error against an unreachable port, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a non-empty connection error")
	}
}

func TestPostgres_NilReceiver(t *testing.T) {
	t.Parallel()

	var p *Postgres

	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("expected error pinging nil *Postgres, got nil")
	}

	// Close must not panic on a nil receiver or nil pool.
	p.Close()

	zero := &Postgres{}
	zero.Close()
	if err := zero.Ping(context.Background()); err == nil {
		t.Fatal("expected error pinging *Postgres with nil pool, got nil")
	}
}
