package config

import (
	"testing"
	"time"
)

func TestLoadEnvOverridesNestedFields(t *testing.T) {
	t.Setenv("VERDEX_SERVER_PORT", "9090")
	t.Setenv("VERDEX_SERVER_READ_TIMEOUT", "2s")
	t.Setenv("VERDEX_DEPLOYMENT_NAME", "verdex-api")
	t.Setenv("VERDEX_OBSERVABILITY_LOG_LEVEL", "debug")

	cfg := Default()
	if err := loadEnv(&cfg); err != nil {
		t.Fatalf("loadEnv() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 2*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want 2s", cfg.Server.ReadTimeout)
	}
	if cfg.Deployment.Name != "verdex-api" {
		t.Errorf("Deployment.Name = %q, want %q", cfg.Deployment.Name, "verdex-api")
	}
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("Observability.LogLevel = %q, want %q", cfg.Observability.LogLevel, "debug")
	}

	// Untouched fields must retain their defaults.
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want default %q", cfg.Server.Host, "0.0.0.0")
	}
}

func TestLoadEnvLeavesUnsetFieldsAlone(t *testing.T) {
	cfg := Default()
	before := cfg

	if err := loadEnv(&cfg); err != nil {
		t.Fatalf("loadEnv() error = %v", err)
	}

	if cfg != before {
		t.Errorf("loadEnv() mutated config with no env vars set: got %+v, want %+v", cfg, before)
	}
}

func TestLoadEnvInvalidIntReturnsError(t *testing.T) {
	t.Setenv("VERDEX_SERVER_PORT", "not-a-number")

	cfg := Default()
	if err := loadEnv(&cfg); err == nil {
		t.Fatal("loadEnv() error = nil, want error for invalid integer")
	}
}

func TestLoadEnvInvalidDurationReturnsError(t *testing.T) {
	t.Setenv("VERDEX_SERVER_READ_TIMEOUT", "not-a-duration")

	cfg := Default()
	if err := loadEnv(&cfg); err == nil {
		t.Fatal("loadEnv() error = nil, want error for invalid duration")
	}
}
