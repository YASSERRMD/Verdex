package persistence

import (
	"context"
	"testing"
)

func TestNewGraphDriver_EmptyTarget(t *testing.T) {
	t.Parallel()

	_, err := NewGraphDriver("", "neo4j", "password")
	if err == nil {
		t.Fatal("expected error for empty target, got nil")
	}
}

func TestGraphDriver_NilReceiver(t *testing.T) {
	t.Parallel()

	var g *GraphDriver

	if err := g.VerifyConnectivity(context.Background()); err == nil {
		t.Fatal("expected error verifying connectivity on nil *GraphDriver, got nil")
	}

	// Close must be a safe no-op on a nil receiver.
	if err := g.Close(context.Background()); err != nil {
		t.Fatalf("expected nil error closing nil *GraphDriver, got %v", err)
	}
}

// TestGraphChecker_NoEndpointConfigured verifies the "Phase 032 owns
// real usage" no-op path: when no Neo4j target is configured, the
// checker must gracefully report healthy rather than failing every
// readiness probe for a store most deployments don't have yet.
func TestGraphChecker_NoEndpointConfigured(t *testing.T) {
	t.Parallel()

	checker := GraphChecker("", "", "")
	if err := checker(context.Background()); err != nil {
		t.Fatalf("expected nil error when no Neo4j endpoint is configured, got %v", err)
	}
}
