package observability

import (
	"context"
	"testing"
)

func TestNewCorrelationID_Unique(t *testing.T) {
	a := NewCorrelationID()
	b := NewCorrelationID()
	if a == "" || b == "" {
		t.Fatal("expected non-empty correlation IDs")
	}
	if a == b {
		t.Fatal("expected distinct correlation IDs across calls")
	}
}

func TestCorrelationIDFromContext_Absent(t *testing.T) {
	_, ok := CorrelationIDFromContext(context.Background())
	if ok {
		t.Fatal("expected no correlation ID on a bare context")
	}
}

func TestWithCorrelationID_RoundTrip(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "abc-123")
	got, ok := CorrelationIDFromContext(ctx)
	if !ok {
		t.Fatal("expected correlation ID to be present")
	}
	if got != "abc-123" {
		t.Errorf("got %q, want %q", got, "abc-123")
	}
}

func TestEnsureCorrelationID_GeneratesWhenAbsent(t *testing.T) {
	ctx, id := EnsureCorrelationID(context.Background())
	if id == "" {
		t.Fatal("expected a generated correlation ID")
	}
	got, ok := CorrelationIDFromContext(ctx)
	if !ok || got != id {
		t.Fatalf("expected context to carry generated ID %q, got %q (ok=%v)", id, got, ok)
	}
}

func TestEnsureCorrelationID_PreservesExisting(t *testing.T) {
	original := WithCorrelationID(context.Background(), "existing-id")
	ctx, id := EnsureCorrelationID(original)
	if id != "existing-id" {
		t.Errorf("id = %q, want %q", id, "existing-id")
	}
	if ctx != original {
		t.Error("expected EnsureCorrelationID to return the original context unchanged when an ID is already present")
	}
}
