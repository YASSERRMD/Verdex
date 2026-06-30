package config

// This file holds a single table-driven test exercising the full
// Loader pipeline end-to-end across every precedence scenario called
// out by the Phase 002 spec: defaults-only, YAML-only, env-only,
// YAML+env (env wins), secret resolution (success/failure), redaction
// correctness, and profile layering. Scenario-specific edge cases live
// alongside their implementation (env_test.go, file_test.go,
// secrets_test.go, redact_test.go, profile_test.go); this file is the
// cross-cutting integration matrix.

import "testing"

func TestConfigPrecedenceMatrix(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		opts    func() []Option
		check   func(t *testing.T, cfg Config)
		wantErr bool
	}{
		{
			name: "defaults only",
			opts: func() []Option { return nil },
			check: func(t *testing.T, cfg Config) {
				if cfg != Default() {
					t.Errorf("got %+v, want Default()", cfg)
				}
			},
		},
		{
			name: "YAML only",
			opts: func() []Option { return []Option{WithFile("testdata/partial.yaml")} },
			check: func(t *testing.T, cfg Config) {
				if cfg.Server.Port != 9001 {
					t.Errorf("Server.Port = %d, want 9001", cfg.Server.Port)
				}
				if cfg.Observability.LogLevel != "debug" {
					t.Errorf("Observability.LogLevel = %q, want debug", cfg.Observability.LogLevel)
				}
				if cfg.Server.Host != "0.0.0.0" {
					t.Errorf("Server.Host = %q, want default", cfg.Server.Host)
				}
			},
		},
		{
			name:    "env only",
			envVars: map[string]string{"VERDEX_SERVER_PORT": "6000"},
			opts:    func() []Option { return nil },
			check: func(t *testing.T, cfg Config) {
				if cfg.Server.Port != 6000 {
					t.Errorf("Server.Port = %d, want 6000", cfg.Server.Port)
				}
			},
		},
		{
			name:    "YAML+env: env wins on shared field, YAML survives on its own field",
			envVars: map[string]string{"VERDEX_SERVER_PORT": "6000"},
			opts:    func() []Option { return []Option{WithFile("testdata/partial.yaml")} },
			check: func(t *testing.T, cfg Config) {
				if cfg.Server.Port != 6000 {
					t.Errorf("Server.Port = %d, want env-provided 6000", cfg.Server.Port)
				}
				if cfg.Observability.LogLevel != "debug" {
					t.Errorf("Observability.LogLevel = %q, want YAML-provided debug", cfg.Observability.LogLevel)
				}
			},
		},
		{
			name:    "secret resolution success",
			envVars: map[string]string{"TEST_DB_PASSWORD": "fake-secret-value", "VERDEX_DATABASE_DSN": "env://TEST_DB_PASSWORD"},
			opts:    func() []Option { return nil },
			check: func(t *testing.T, cfg Config) {
				if cfg.Database.DSN != "fake-secret-value" {
					t.Errorf("Database.DSN = %q, want resolved secret", cfg.Database.DSN)
				}
			},
		},
		{
			name:    "secret resolution failure: unset env var referenced",
			envVars: map[string]string{"VERDEX_DATABASE_DSN": "env://TOTALLY_UNSET_TEST_VAR"},
			opts:    func() []Option { return nil },
			wantErr: true,
		},
		{
			name:    "profile layering: explicit profile wins over base file, env wins over profile",
			envVars: map[string]string{"VERDEX_SERVER_PORT": "9999"},
			opts: func() []Option {
				return []Option{
					WithFile("testdata/partial.yaml"),
					WithProfile("production"),
					WithProfileDir("testdata/profiles"),
				}
			},
			check: func(t *testing.T, cfg Config) {
				if cfg.Server.Port != 9999 {
					t.Errorf("Server.Port = %d, want env-provided 9999", cfg.Server.Port)
				}
				if cfg.Deployment.Environment != "production" {
					t.Errorf("Deployment.Environment = %q, want profile-provided production", cfg.Deployment.Environment)
				}
				if cfg.Observability.LogLevel != "warn" {
					t.Errorf("Observability.LogLevel = %q, want profile-provided warn (profile over base file)", cfg.Observability.LogLevel)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg, err := NewLoader(tt.opts()...).Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			tt.check(t, cfg)
		})
	}
}

func TestRedactionCorrectnessAcrossScenarios(t *testing.T) {
	tests := []struct {
		name       string
		dsn        string
		wantString string
	}{
		{name: "literal credential", dsn: "postgres://user:pass@host/db", wantString: redactedPlaceholder},
		{name: "already-resolved secret value", dsn: "resolved-secret-xyz", wantString: redactedPlaceholder},
		{name: "empty value left empty", dsn: "", wantString: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Database.DSN = tt.dsn

			redacted := cfg.Redacted()
			if redacted.Database.DSN != tt.wantString {
				t.Errorf("Redacted().Database.DSN = %q, want %q", redacted.Database.DSN, tt.wantString)
			}

			// Non-secret fields must always pass through unchanged.
			if redacted.Deployment.Name != cfg.Deployment.Name {
				t.Errorf("Redacted() altered non-secret field Deployment.Name: got %q, want %q", redacted.Deployment.Name, cfg.Deployment.Name)
			}
		})
	}
}
