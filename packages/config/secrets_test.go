package config

import (
	"strings"
	"testing"
)

func TestEnvResolverResolvesSetVar(t *testing.T) {
	t.Setenv("TEST_DB_PASSWORD", "s3cr3t-fake-value")

	r := NewDefaultResolver()
	got, err := r.Resolve("env://TEST_DB_PASSWORD")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "s3cr3t-fake-value" {
		t.Errorf("Resolve() = %q, want %q", got, "s3cr3t-fake-value")
	}
}

func TestEnvResolverFailsLoudlyOnUnsetVar(t *testing.T) {
	r := NewDefaultResolver()
	_, err := r.Resolve("env://TEST_DB_PASSWORD_DEFINITELY_UNSET")
	if err == nil {
		t.Fatal("Resolve() error = nil, want error for unset env var")
	}
	if !strings.Contains(err.Error(), "TEST_DB_PASSWORD_DEFINITELY_UNSET") {
		t.Errorf("Resolve() error = %q, want it to name the missing var", err.Error())
	}
}

func TestVaultResolverIsUnimplementedPlaceholder(t *testing.T) {
	r := NewDefaultResolver()
	_, err := r.Resolve("vault://secret/data/verdex#dsn")
	if err == nil {
		t.Fatal("Resolve() error = nil, want error: vault is a documented placeholder, not a real backend")
	}
}

func TestMultiResolverRejectsUnknownScheme(t *testing.T) {
	r := NewDefaultResolver()
	_, err := r.Resolve("ftp://example.com/secret")
	if err == nil {
		t.Fatal("Resolve() error = nil, want error for unrecognized scheme")
	}
}

func TestResolveSecretsReplacesReferencesInPlace(t *testing.T) {
	t.Setenv("TEST_DB_PASSWORD", "fake-resolved-dsn")

	cfg := Default()
	cfg.Database.DSN = "env://TEST_DB_PASSWORD"

	if err := resolveSecrets(&cfg, NewDefaultResolver()); err != nil {
		t.Fatalf("resolveSecrets() error = %v", err)
	}

	if cfg.Database.DSN != "fake-resolved-dsn" {
		t.Errorf("Database.DSN = %q, want resolved value", cfg.Database.DSN)
	}
}

func TestResolveSecretsLeavesNonReferenceValuesAlone(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://localhost/verdex"

	if err := resolveSecrets(&cfg, NewDefaultResolver()); err != nil {
		t.Fatalf("resolveSecrets() error = %v", err)
	}

	if cfg.Database.DSN != "postgres://localhost/verdex" {
		t.Errorf("Database.DSN = %q, want unchanged literal value", cfg.Database.DSN)
	}
}

func TestResolveSecretsFailsLoudlyOnUnsetEnvReference(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "env://TEST_DB_PASSWORD_DEFINITELY_UNSET"

	if err := resolveSecrets(&cfg, NewDefaultResolver()); err == nil {
		t.Fatal("resolveSecrets() error = nil, want error for unresolved env reference")
	}
}

func TestResolveSecretsCustomResolver(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "vault://secret/data/verdex#dsn"

	custom := SecretResolverFunc(func(ref string) (string, error) {
		return "stubbed:" + ref, nil
	})

	if err := resolveSecrets(&cfg, custom); err != nil {
		t.Fatalf("resolveSecrets() error = %v", err)
	}
	if cfg.Database.DSN != "stubbed:vault://secret/data/verdex#dsn" {
		t.Errorf("Database.DSN = %q, want stubbed resolution", cfg.Database.DSN)
	}
}
