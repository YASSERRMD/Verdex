package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadYAMLFileEmptyPathIsNoop(t *testing.T) {
	cfg := Default()
	before := cfg

	if err := loadYAMLFile(&cfg, ""); err != nil {
		t.Fatalf("loadYAMLFile() error = %v", err)
	}
	if cfg != before {
		t.Errorf("loadYAMLFile(\"\") mutated config: got %+v, want %+v", cfg, before)
	}
}

func TestLoadYAMLFileOverridesSetFields(t *testing.T) {
	cfg := Default()

	if err := loadYAMLFile(&cfg, "testdata/partial.yaml"); err != nil {
		t.Fatalf("loadYAMLFile() error = %v", err)
	}

	if cfg.Server.Port != 9001 {
		t.Errorf("Server.Port = %d, want 9001", cfg.Server.Port)
	}
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("Observability.LogLevel = %q, want %q", cfg.Observability.LogLevel, "debug")
	}

	// Fields the YAML file omits must retain their prior (default) values.
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want default %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Database.MaxOpenConns != 10 {
		t.Errorf("Database.MaxOpenConns = %d, want default 10", cfg.Database.MaxOpenConns)
	}
}

func TestLoadYAMLFileExampleParsesFully(t *testing.T) {
	cfg := Default()

	if err := loadYAMLFile(&cfg, "testdata/config.example.yaml"); err != nil {
		t.Fatalf("loadYAMLFile() error = %v", err)
	}

	if cfg.Deployment.Name != "verdex-api" {
		t.Errorf("Deployment.Name = %q, want %q", cfg.Deployment.Name, "verdex-api")
	}
	if cfg.Deployment.Environment != "sandbox" {
		t.Errorf("Deployment.Environment = %q, want %q", cfg.Deployment.Environment, "sandbox")
	}
	if cfg.Database.DSN != "env://VERDEX_DATABASE_DSN" {
		t.Errorf("Database.DSN = %q, want a secret reference", cfg.Database.DSN)
	}
	if cfg.Database.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("Database.ConnMaxLifetime = %v, want 30m", cfg.Database.ConnMaxLifetime)
	}
}

func TestLoadYAMLFileMissingFileReturnsError(t *testing.T) {
	cfg := Default()
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	if err := loadYAMLFile(&cfg, missing); err == nil {
		t.Fatal("loadYAMLFile() error = nil, want error for missing file")
	}
}

func TestLoadYAMLFileInvalidYAMLReturnsError(t *testing.T) {
	cfg := Default()
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("server: [this is not a mapping"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := loadYAMLFile(&cfg, bad); err == nil {
		t.Fatal("loadYAMLFile() error = nil, want error for malformed YAML")
	}
}
