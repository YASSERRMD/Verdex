package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestFromContext_ReturnsBoundLogger(t *testing.T) {
	var buf bytes.Buffer
	base := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))
	bound := base.With("scope", "request")

	ctx := WithLogger(context.Background(), bound)

	got := FromContext(ctx, base)
	got.Info(context.Background(), "msg")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if record["scope"] != "request" {
		t.Errorf("expected bound logger to be returned, got record %v", record)
	}
}

func TestFromContext_FallsBackWithCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	base := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))

	ctx := WithCorrelationID(context.Background(), "corr-1")

	got := FromContext(ctx, base)
	got.Info(context.Background(), "msg")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if record[correlationIDLogField] != "corr-1" {
		t.Errorf("expected fallback logger to carry correlation id, got record %v", record)
	}
}

func TestFromContext_PlainFallback(t *testing.T) {
	var buf bytes.Buffer
	base := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))

	got := FromContext(context.Background(), base)
	if got != base {
		t.Error("expected the fallback logger itself when context carries nothing")
	}
}
