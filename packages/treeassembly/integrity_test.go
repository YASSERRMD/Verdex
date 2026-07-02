package treeassembly

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestValidateIntegrity_ValidTree(t *testing.T) {
	input := syntheticInput("case-1")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	issues := ValidateIntegrity(tree)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
	if HasCriticalIntegrityFailure(issues) {
		t.Fatal("expected no critical integrity failure")
	}
}

func TestValidateIntegrity_NilTree(t *testing.T) {
	issues := ValidateIntegrity(nil)
	if len(issues) != 0 {
		t.Fatalf("expected no issues for nil tree, got %v", issues)
	}
}

func TestValidateIntegrity_DanglingEdge(t *testing.T) {
	input := syntheticInput("case-1")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree.Edges = append(tree.Edges, irac.Edge{FromID: "does-not-exist", ToID: "also-missing", Type: irac.EdgeGoverns})

	issues := ValidateIntegrity(tree)
	if len(issues) == 0 {
		t.Fatal("expected issues for dangling edge")
	}
	if !HasCriticalIntegrityFailure(issues) {
		t.Fatal("expected critical integrity failure")
	}
}

func TestHasCriticalIntegrityFailure_Table(t *testing.T) {
	tests := []struct {
		name   string
		issues []irac.ValidationIssue
		want   bool
	}{
		{"empty", []irac.ValidationIssue{}, false},
		{"nil", nil, false},
		{"one issue", []irac.ValidationIssue{{Message: "x"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasCriticalIntegrityFailure(tt.issues); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
