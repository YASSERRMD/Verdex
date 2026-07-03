package encryption_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/encryption"
)

func TestAssertEncryptedAtRest_FailsClosedByDefault(t *testing.T) {
	cfg := encryption.AtRestConfig{}
	if err := encryption.AssertEncryptedAtRest(cfg, ""); err == nil {
		t.Fatal("AssertEncryptedAtRest() error = nil, want error for a zero-value (undeclared) AtRestConfig")
	}
}

func TestAssertEncryptedAtRest_PassesWhenDeclared(t *testing.T) {
	cfg := encryption.AtRestConfig{EncryptedAtRest: true}
	if err := encryption.AssertEncryptedAtRest(cfg, ""); err != nil {
		t.Fatalf("AssertEncryptedAtRest() error = %v, want nil when EncryptedAtRest is declared true", err)
	}
}

func TestAssertEncryptedAtRest_RequireTLSInTransitChecksDSN(t *testing.T) {
	cfg := encryption.AtRestConfig{EncryptedAtRest: true, RequireTLSInTransit: true}

	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{"sslmode require", "postgres://user:pass@host:5432/db?sslmode=require", false},
		{"sslmode verify-full", "host=db user=verdex sslmode=verify-full", false},
		{"sslmode disable", "postgres://user:pass@host:5432/db?sslmode=disable", true},
		{"sslmode allow", "host=db sslmode=allow", true},
		{"no sslmode at all", "postgres://user:pass@host:5432/db", true},
		{"empty dsn", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := encryption.AssertEncryptedAtRest(cfg, tt.dsn)
			if tt.wantErr && err == nil {
				t.Errorf("AssertEncryptedAtRest(dsn=%q) error = nil, want error", tt.dsn)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("AssertEncryptedAtRest(dsn=%q) error = %v, want nil", tt.dsn, err)
			}
		})
	}
}

func TestAssertEncryptedAtRest_TLSCheckSkippedWhenNotRequired(t *testing.T) {
	cfg := encryption.AtRestConfig{EncryptedAtRest: true, RequireTLSInTransit: false}
	if err := encryption.AssertEncryptedAtRest(cfg, "postgres://user:pass@host/db?sslmode=disable"); err != nil {
		t.Fatalf("AssertEncryptedAtRest() error = %v, want nil when RequireTLSInTransit is false regardless of DSN", err)
	}
}
