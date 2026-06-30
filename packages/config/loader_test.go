package config

import (
	"testing"
)

func TestLoaderDefaultsOnly(t *testing.T) {
	cfg, err := NewLoader().Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Default()
	if cfg != want {
		t.Errorf("Load() = %+v, want defaults %+v", cfg, want)
	}
}

func TestLoaderYAMLOnly(t *testing.T) {
	cfg, err := NewLoader(WithFile("testdata/partial.yaml")).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9001 {
		t.Errorf("Server.Port = %d, want 9001", cfg.Server.Port)
	}
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("Observability.LogLevel = %q, want %q", cfg.Observability.LogLevel, "debug")
	}
	// Field omitted from YAML keeps its default.
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want default %q", cfg.Server.Host, "0.0.0.0")
	}
}

func TestLoaderEnvOnly(t *testing.T) {
	t.Setenv("VERDEX_SERVER_PORT", "7777")

	cfg, err := NewLoader().Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 7777 {
		t.Errorf("Server.Port = %d, want 7777", cfg.Server.Port)
	}
}

func TestLoaderEnvWinsOverYAML(t *testing.T) {
	t.Setenv("VERDEX_SERVER_PORT", "7777")

	cfg, err := NewLoader(WithFile("testdata/partial.yaml")).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Env must win over YAML for the field both set.
	if cfg.Server.Port != 7777 {
		t.Errorf("Server.Port = %d, want env-provided 7777", cfg.Server.Port)
	}
	// YAML-only field (not touched by env) must survive.
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("Observability.LogLevel = %q, want YAML-provided %q", cfg.Observability.LogLevel, "debug")
	}
	// Default-only field (not touched by YAML or env) must survive.
	if cfg.Database.MaxOpenConns != 10 {
		t.Errorf("Database.MaxOpenConns = %d, want default 10", cfg.Database.MaxOpenConns)
	}
}

func TestLoaderMissingFilePropagatesError(t *testing.T) {
	_, err := NewLoader(WithFile("testdata/does-not-exist.yaml")).Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for missing file")
	}
}

func TestLoaderResolvesSecretReferencesAfterMerge(t *testing.T) {
	t.Setenv("TEST_DB_PASSWORD", "fake-resolved-via-loader")
	t.Setenv("VERDEX_DATABASE_DSN", "env://TEST_DB_PASSWORD")

	cfg, err := NewLoader().Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.DSN != "fake-resolved-via-loader" {
		t.Errorf("Database.DSN = %q, want resolved secret value", cfg.Database.DSN)
	}
}

func TestLoaderPropagatesSecretResolutionFailure(t *testing.T) {
	t.Setenv("VERDEX_DATABASE_DSN", "env://TEST_DB_PASSWORD_DEFINITELY_UNSET")

	_, err := NewLoader().Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for unresolved secret reference")
	}
}

func TestLoaderWithSecretResolverOverride(t *testing.T) {
	t.Setenv("VERDEX_DATABASE_DSN", "vault://secret/data/verdex#dsn")

	custom := SecretResolverFunc(func(ref string) (string, error) {
		return "stub:" + ref, nil
	})

	cfg, err := NewLoader(WithSecretResolver(custom)).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.DSN != "stub:vault://secret/data/verdex#dsn" {
		t.Errorf("Database.DSN = %q, want stubbed resolution", cfg.Database.DSN)
	}
}
