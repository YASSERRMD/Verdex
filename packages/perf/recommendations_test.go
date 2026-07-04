package perf

import "testing"

func TestGraphIndexRecommendations_WellFormed(t *testing.T) {
	recs := GraphIndexRecommendations()
	if len(recs) == 0 {
		t.Fatal("expected at least one graph index recommendation")
	}

	seenIDs := make(map[string]struct{})
	for _, r := range recs {
		if err := r.validate(); err != nil {
			t.Errorf("recommendation %+v failed validation: %v", r, err)
		}
		if r.TargetPackage != "packages/graph" {
			t.Errorf("expected every recommendation in this list to target packages/graph, got %q for %q", r.TargetPackage, r.ID)
		}
		if _, dup := seenIDs[r.ID]; dup {
			t.Errorf("duplicate recommendation ID %q", r.ID)
		}
		seenIDs[r.ID] = struct{}{}
	}
}

func TestRecommendation_ValidateRejectsBlankFields(t *testing.T) {
	base := Recommendation{
		ID:            "X-1",
		Title:         "title",
		Rationale:     "rationale",
		TargetPackage: "packages/graph",
		Impact:        PriorityLow,
		Status:        StatusProposed,
	}
	if err := base.validate(); err != nil {
		t.Fatalf("expected well-formed base recommendation to validate, got %v", err)
	}

	missingID := base
	missingID.ID = ""
	if err := missingID.validate(); err == nil {
		t.Error("expected validation error for missing ID")
	}

	missingTitle := base
	missingTitle.Title = ""
	if err := missingTitle.validate(); err == nil {
		t.Error("expected validation error for missing Title")
	}

	missingRationale := base
	missingRationale.Rationale = ""
	if err := missingRationale.validate(); err == nil {
		t.Error("expected validation error for missing Rationale")
	}

	missingTarget := base
	missingTarget.TargetPackage = ""
	if err := missingTarget.validate(); err == nil {
		t.Error("expected validation error for missing TargetPackage")
	}

	invalidImpact := base
	invalidImpact.Impact = Priority("urgent")
	if err := invalidImpact.validate(); err == nil {
		t.Error("expected validation error for invalid Impact")
	}

	invalidStatus := base
	invalidStatus.Status = RecommendationStatus("unknown")
	if err := invalidStatus.validate(); err == nil {
		t.Error("expected validation error for invalid Status")
	}
}
