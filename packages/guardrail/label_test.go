package guardrail_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestDraftAnalysisLabelMatchesIrac(t *testing.T) {
	if string(guardrail.DraftAnalysisLabel) != irac.DraftAnalysisLabel {
		t.Fatalf("guardrail.DraftAnalysisLabel = %q, want %q (must match irac.DraftAnalysisLabel)",
			guardrail.DraftAnalysisLabel, irac.DraftAnalysisLabel)
	}
}

func TestRequireLabel(t *testing.T) {
	tests := []struct {
		name    string
		label   string
		wantErr bool
	}{
		{name: "exact mandatory label", label: "draft_analysis", wantErr: false},
		{name: "empty label", label: "", wantErr: true},
		{name: "wrong label", label: "final_verdict", wantErr: true},
		{name: "case-mismatched label", label: "Draft_Analysis", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guardrail.RequireLabel(tt.label)
			if tt.wantErr && err == nil {
				t.Fatalf("RequireLabel(%q) = nil, want error", tt.label)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("RequireLabel(%q) = %v, want nil", tt.label, err)
			}
			if tt.wantErr && !errors.Is(err, guardrail.ErrMissingLabel) {
				t.Fatalf("RequireLabel(%q) error = %v, want errors.Is ErrMissingLabel", tt.label, err)
			}
		})
	}
}

// stubLabeled is a minimal Labeled implementation for testing
// ValidateLabeled without depending on any concrete reasoning-output
// type.
type stubLabeled string

func (s stubLabeled) Label() string { return string(s) }

func TestValidateLabeled(t *testing.T) {
	if err := guardrail.ValidateLabeled(stubLabeled("draft_analysis")); err != nil {
		t.Fatalf("ValidateLabeled(valid) = %v, want nil", err)
	}

	err := guardrail.ValidateLabeled(stubLabeled("verdict"))
	if !errors.Is(err, guardrail.ErrMissingLabel) {
		t.Fatalf("ValidateLabeled(invalid) = %v, want errors.Is ErrMissingLabel", err)
	}
}

func TestValidateLabeledNil(t *testing.T) {
	err := guardrail.ValidateLabeled(nil)
	if !errors.Is(err, guardrail.ErrMissingLabel) {
		t.Fatalf("ValidateLabeled(nil) = %v, want errors.Is ErrMissingLabel", err)
	}
}

func TestWrapConclusionNode(t *testing.T) {
	node := irac.NewConclusionNode("c1", "case-1", "the evidence weighs toward X", time.Now(), 0.7, irac.Provenance{})

	labeled := guardrail.WrapConclusionNode(node)
	if err := guardrail.ValidateLabeled(labeled); err != nil {
		t.Fatalf("ValidateLabeled(WrapConclusionNode(NewConclusionNode(...))) = %v, want nil (guardrail label is always attached)", err)
	}

	// Defensive check: a ConclusionNode that reached this boundary with
	// its label field stripped (e.g. via untrusted deserialization) must
	// still be rejected.
	node.Label = ""
	stripped := guardrail.WrapConclusionNode(node)
	if err := guardrail.ValidateLabeled(stripped); !errors.Is(err, guardrail.ErrMissingLabel) {
		t.Fatalf("ValidateLabeled(stripped-label node) = %v, want errors.Is ErrMissingLabel", err)
	}
}
