package config

import "testing"

func TestLoaderNoProfileIsNoop(t *testing.T) {
	cfg, err := NewLoader(WithProfileDir("testdata/profiles")).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Default()
	if cfg != want {
		t.Errorf("Load() with no profile = %+v, want defaults %+v", cfg, want)
	}
}

func TestLoaderExplicitProfileOverlay(t *testing.T) {
	cfg, err := NewLoader(
		WithProfile("sandbox"),
		WithProfileDir("testdata/profiles"),
	).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Deployment.Environment != "sandbox" {
		t.Errorf("Deployment.Environment = %q, want %q", cfg.Deployment.Environment, "sandbox")
	}
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("Observability.LogLevel = %q, want %q", cfg.Observability.LogLevel, "debug")
	}
	// Fields the profile doesn't mention keep their defaults.
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want default 8080", cfg.Server.Port)
	}
}

func TestLoaderProfileSelectedViaEnvVar(t *testing.T) {
	t.Setenv(ProfileEnvVar, "production")

	cfg, err := NewLoader(WithProfileDir("testdata/profiles")).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Deployment.Environment != "production" {
		t.Errorf("Deployment.Environment = %q, want %q", cfg.Deployment.Environment, "production")
	}
	if cfg.Server.Port != 443 {
		t.Errorf("Server.Port = %d, want 443", cfg.Server.Port)
	}
}

func TestLoaderExplicitProfileOverridesEnvVar(t *testing.T) {
	t.Setenv(ProfileEnvVar, "production")

	cfg, err := NewLoader(
		WithProfile("sandbox"),
		WithProfileDir("testdata/profiles"),
	).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Deployment.Environment != "sandbox" {
		t.Errorf("Deployment.Environment = %q, want WithProfile to win over VERDEX_PROFILE (%q)", cfg.Deployment.Environment, "sandbox")
	}
}

func TestLoaderProfileLayersOnTopOfYAMLFile(t *testing.T) {
	cfg, err := NewLoader(
		WithFile("testdata/partial.yaml"), // sets server.port=9001, observability.log_level=debug
		WithProfile("production"),         // sets server.port=443, observability.log_level=warn
		WithProfileDir("testdata/profiles"),
	).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Profile overrides the base file where both set a value.
	if cfg.Server.Port != 443 {
		t.Errorf("Server.Port = %d, want profile-provided 443", cfg.Server.Port)
	}
	if cfg.Observability.LogLevel != "warn" {
		t.Errorf("Observability.LogLevel = %q, want profile-provided %q", cfg.Observability.LogLevel, "warn")
	}
}

func TestLoaderEnvWinsOverProfile(t *testing.T) {
	t.Setenv("VERDEX_SERVER_PORT", "5555")

	cfg, err := NewLoader(
		WithProfile("production"), // sets server.port=443
		WithProfileDir("testdata/profiles"),
	).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 5555 {
		t.Errorf("Server.Port = %d, want env-provided 5555 (env must win over profile)", cfg.Server.Port)
	}
}

func TestLoaderUnknownProfileFailsLoudly(t *testing.T) {
	_, err := NewLoader(
		WithProfile("does-not-exist"),
		WithProfileDir("testdata/profiles"),
	).Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for unknown profile name")
	}
}

func TestDefaultProfileDirDerivation(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		want     string
	}{
		{"empty base path", "", "profiles"},
		{"base path in subdir", "config/base.yaml", "config/profiles"},
		{"base path at root", "base.yaml", "profiles"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultProfileDir(tt.basePath); got != tt.want {
				t.Errorf("defaultProfileDir(%q) = %q, want %q", tt.basePath, got, tt.want)
			}
		})
	}
}
