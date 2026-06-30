package config

import "testing"

func TestValidateDefaultIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil for Default()", err)
	}
}

func TestValidateCatchesMissingRequiredFields(t *testing.T) {
	cfg := Default()
	cfg.Deployment.Name = ""
	cfg.Deployment.Environment = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing deployment fields")
	}
}

func TestValidateCatchesPortOutOfRange(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too large", 70000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Server.Port = tt.port

			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() error = nil, want error for port %d", tt.port)
			}
		})
	}
}

func TestValidateCatchesNonPositiveTimeouts(t *testing.T) {
	cfg := Default()
	cfg.Server.ReadTimeout = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for zero read_timeout")
	}
}

func TestValidateCatchesIdleExceedingOpenConns(t *testing.T) {
	cfg := Default()
	cfg.Database.MaxOpenConns = 5
	cfg.Database.MaxIdleConns = 10

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error when max_idle_conns > max_open_conns")
	}
}

func TestValidateCatchesInvalidLogLevel(t *testing.T) {
	cfg := Default()
	cfg.Observability.LogLevel = "verbose"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for invalid log level")
	}
}

func TestValidateCatchesInvalidLogFormat(t *testing.T) {
	cfg := Default()
	cfg.Observability.LogFormat = "xml"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for invalid log format")
	}
}

func TestValidateJoinsMultipleErrors(t *testing.T) {
	cfg := Default()
	cfg.Deployment.Name = ""
	cfg.Server.Port = -1
	cfg.Observability.LogLevel = "nonsense"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want joined error for multiple violations")
	}

	msg := err.Error()
	for _, want := range []string{"deployment.name", "server.port", "observability.log_level"} {
		if !containsSubstring(msg, want) {
			t.Errorf("Validate() error %q missing expected substring %q", msg, want)
		}
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLoaderPropagatesValidationFailure(t *testing.T) {
	t.Setenv("VERDEX_SERVER_PORT", "0")

	_, err := NewLoader().Load()
	if err == nil {
		t.Fatal("Load() error = nil, want validation error for invalid port")
	}
}
