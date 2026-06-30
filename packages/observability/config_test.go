package observability

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/config"
)

func TestNewLoggerFromConfig_UsesObservabilitySettings(t *testing.T) {
	cfg := config.Default()
	cfg.Observability.LogLevel = "debug"
	cfg.Observability.LogFormat = "console"

	var buf bytes.Buffer
	logger, err := NewLoggerFromConfig(&cfg, WithOutput(&buf))
	if err != nil {
		t.Fatalf("NewLoggerFromConfig: %v", err)
	}

	logger.Debug(context.Background(), "debug message visible")
	out := buf.String()
	if !strings.Contains(out, "debug message visible") {
		t.Fatalf("expected debug level to be enabled, got: %s", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected console (non-JSON) output, got: %s", out)
	}
}

func TestNewLoggerFromConfig_DefaultsAreJSONInfo(t *testing.T) {
	cfg := config.Default()

	var buf bytes.Buffer
	logger, err := NewLoggerFromConfig(&cfg, WithOutput(&buf))
	if err != nil {
		t.Fatalf("NewLoggerFromConfig: %v", err)
	}

	logger.Debug(context.Background(), "should be filtered")
	if buf.Len() != 0 {
		t.Fatalf("expected debug to be filtered at default info level, got: %s", buf.String())
	}

	logger.Info(context.Background(), "should appear")
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Errorf("expected default JSON output, got: %s", buf.String())
	}
}

func TestNewLoggerFromConfig_InvalidLevel(t *testing.T) {
	cfg := config.Default()
	cfg.Observability.LogLevel = "not-a-level"

	if _, err := NewLoggerFromConfig(&cfg); err == nil {
		t.Fatal("expected an error for an invalid log level")
	}
}

func TestNewLoggerFromConfig_InvalidFormat(t *testing.T) {
	cfg := config.Default()
	cfg.Observability.LogFormat = "not-a-format"

	if _, err := NewLoggerFromConfig(&cfg); err == nil {
		t.Fatal("expected an error for an invalid log format")
	}
}

func TestNewLoggerFromConfig_NilConfig(t *testing.T) {
	if _, err := NewLoggerFromConfig(nil); err == nil {
		t.Fatal("expected an error for a nil config")
	}
}
