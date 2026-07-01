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
