package config

import "testing"

func TestRedactedMasksSecretBearingFields(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://user:hunter2@localhost/verdex"

	redacted := cfg.Redacted()

	if redacted.Database.DSN != redactedPlaceholder {
		t.Errorf("Redacted().Database.DSN = %q, want %q", redacted.Database.DSN, redactedPlaceholder)
	}
}

func TestRedactedDoesNotMutateOriginal(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://user:hunter2@localhost/verdex"

	_ = cfg.Redacted()

	if cfg.Database.DSN != "postgres://user:hunter2@localhost/verdex" {
		t.Errorf("Redacted() mutated the receiver: Database.DSN = %q", cfg.Database.DSN)
	}
}

func TestRedactedLeavesNonSecretFieldsAlone(t *testing.T) {
	cfg := Default()
	cfg.Deployment.Name = "verdex-api"

	redacted := cfg.Redacted()

	if redacted.Deployment.Name != "verdex-api" {
		t.Errorf("Redacted().Deployment.Name = %q, want unchanged %q", redacted.Deployment.Name, "verdex-api")
	}
	if redacted.Server.Host != cfg.Server.Host {
		t.Errorf("Redacted().Server.Host = %q, want unchanged %q", redacted.Server.Host, cfg.Server.Host)
	}
}

func TestRedactedSkipsEmptySecretField(t *testing.T) {
	cfg := Default() // Database.DSN is "" by default.

	redacted := cfg.Redacted()

	if redacted.Database.DSN != "" {
		t.Errorf("Redacted().Database.DSN = %q, want empty string left as-is", redacted.Database.DSN)
	}
}

func TestConfigStringDoesNotLeakSecret(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://user:hunter2@localhost/verdex"

	s := cfg.String()

	if containsSubstring(s, "hunter2") {
		t.Errorf("String() leaked secret value: %s", s)
	}
	if !containsSubstring(s, redactedPlaceholder) {
		t.Errorf("String() = %s, want it to contain redaction placeholder", s)
	}
}
