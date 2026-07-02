package precedent

import "testing"

func TestCourtLevel_Weight_Ordering(t *testing.T) {
	if CourtSupreme.Weight() <= CourtAppellate.Weight() {
		t.Errorf("CourtSupreme.Weight() = %v, want > CourtAppellate.Weight() = %v", CourtSupreme.Weight(), CourtAppellate.Weight())
	}
	if CourtAppellate.Weight() <= CourtTrial.Weight() {
		t.Errorf("CourtAppellate.Weight() = %v, want > CourtTrial.Weight() = %v", CourtAppellate.Weight(), CourtTrial.Weight())
	}
	if CourtTrial.Weight() <= CourtUnknown.Weight() {
		t.Errorf("CourtTrial.Weight() = %v, want > CourtUnknown.Weight() = %v", CourtTrial.Weight(), CourtUnknown.Weight())
	}
	if CourtUnknown.Weight() <= 0 {
		t.Errorf("CourtUnknown.Weight() = %v, want > 0", CourtUnknown.Weight())
	}
}

func TestCourtLevel_IsValid(t *testing.T) {
	tests := []struct {
		level CourtLevel
		want  bool
	}{
		{CourtSupreme, true},
		{CourtAppellate, true},
		{CourtTrial, true},
		{CourtUnknown, true},
		{CourtLevel("not-a-level"), false},
	}
	for _, tt := range tests {
		if got := tt.level.IsValid(); got != tt.want {
			t.Errorf("%q.IsValid() = %v, want %v", tt.level, got, tt.want)
		}
	}
}

func TestClassifyCourtLevel(t *testing.T) {
	tests := []struct {
		name string
		want CourtLevel
	}{
		{"House of Lords", CourtSupreme},
		{"Supreme Court of the United States", CourtSupreme},
		{"Privy Council", CourtSupreme},
		{"Court of Appeal (Civil Division)", CourtAppellate},
		{"Ninth Circuit Court of Appeals", CourtAppellate},
		{"High Court of Justice", CourtTrial},
		{"District Court", CourtTrial},
		{"", CourtUnknown},
		{"Some Obscure Tribunal", CourtUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyCourtLevel(tt.name); got != tt.want {
				t.Errorf("ClassifyCourtLevel(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestApplyCourtHierarchy(t *testing.T) {
	rule := syntheticPrecedentRule(t)
	tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{CategoryCode: "tort"})

	hierarchy := ApplyCourtHierarchy(tagged, "")
	if len(hierarchy) != 1 {
		t.Fatalf("len(hierarchy) = %d, want 1", len(hierarchy))
	}
	if hierarchy[0].CourtLevel != CourtSupreme {
		t.Errorf("CourtLevel = %q, want %q (House of Lords)", hierarchy[0].CourtLevel, CourtSupreme)
	}
}

func TestApplyCourtHierarchy_Override(t *testing.T) {
	rule := syntheticPrecedentRule(t)
	tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{CategoryCode: "tort"})

	hierarchy := ApplyCourtHierarchy(tagged, CourtTrial)
	if hierarchy[0].CourtLevel != CourtTrial {
		t.Errorf("CourtLevel = %q, want %q (override)", hierarchy[0].CourtLevel, CourtTrial)
	}
}
