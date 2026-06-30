package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    Level
		wantErr bool
	}{
		{"debug", LevelDebug, false},
		{"DEBUG", LevelDebug, false},
		{"info", LevelInfo, false},
		{"", LevelInfo, false},
		{"warn", LevelWarn, false},
		{"warning", LevelWarn, false},
		{"error", LevelError, false},
		{"bogus", LevelInfo, true},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLevel(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"json", FormatJSON, false},
		{"", FormatJSON, false},
		{"console", FormatConsole, false},
		{"text", FormatConsole, false},
		{"bogus", FormatJSON, true},
	}
	for _, tt := range tests {
		got, err := ParseFormat(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseFormat(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("ParseFormat(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))

	logger.Info(context.Background(), "hello world", "key", "value")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if record["msg"] != "hello world" {
		t.Errorf("msg = %v, want %q", record["msg"], "hello world")
	}
	if record["key"] != "value" {
		t.Errorf("key = %v, want %q", record["key"], "value")
	}
	if record["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", record["level"])
	}
}

func TestLogger_ConsoleOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithLevel(LevelDebug), WithFormat(FormatConsole), WithOutput(&buf))

	logger.Warn(context.Background(), "careful now", "n", 42)

	out := buf.String()
	if !strings.Contains(out, "careful now") {
		t.Errorf("output missing message: %s", out)
	}
	if !strings.Contains(out, "n=42") {
		t.Errorf("output missing field: %s", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("console output looks like JSON: %s", out)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithLevel(LevelWarn), WithFormat(FormatJSON), WithOutput(&buf))

	logger.Info(context.Background(), "should be dropped")
	if buf.Len() != 0 {
		t.Fatalf("expected info log to be filtered out, got: %s", buf.String())
	}

	logger.Warn(context.Background(), "should appear")
	if buf.Len() == 0 {
		t.Fatalf("expected warn log to be emitted")
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	base := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))
	child := base.With("tenant_id", "acme")

	child.Info(context.Background(), "scoped message")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if record["tenant_id"] != "acme" {
		t.Errorf("tenant_id = %v, want acme", record["tenant_id"])
	}
}

func TestLogger_Enabled(t *testing.T) {
	logger := New(WithLevel(LevelWarn))
	ctx := context.Background()

	if logger.Enabled(ctx, LevelDebug) {
		t.Error("debug should not be enabled at warn level")
	}
	if !logger.Enabled(ctx, LevelError) {
		t.Error("error should be enabled at warn level")
	}
}

func TestLogger_AllLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))
	ctx := context.Background()

	logger.Debug(ctx, "debug msg")
	logger.Info(ctx, "info msg")
	logger.Warn(ctx, "warn msg")
	logger.Error(ctx, "error msg")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 log lines, got %d: %v", len(lines), lines)
	}
}
