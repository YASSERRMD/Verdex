package statute

import "testing"

func TestDetectCrossReferences(t *testing.T) {
	text := `A contract is formed when an offer is accepted. See Section 1 for
definitions. Exceptions are addressed under Section 12(a).`

	refs := DetectCrossReferences("rule-2", text)
	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2: %+v", len(refs), refs)
	}
	if refs[0].Section != "1" || refs[0].Clause != "" {
		t.Errorf("refs[0] = %+v, want Section=1 Clause=empty", refs[0])
	}
	if refs[1].Section != "12" || refs[1].Clause != "a" {
		t.Errorf("refs[1] = %+v, want Section=12 Clause=a", refs[1])
	}
	for _, r := range refs {
		if r.SourceRuleID != "rule-2" {
			t.Errorf("SourceRuleID = %q, want rule-2", r.SourceRuleID)
		}
		if r.IsResolved() {
			t.Error("freshly detected reference should not be resolved yet")
		}
	}
}

func TestDetectCrossReferences_NoMatches(t *testing.T) {
	refs := DetectCrossReferences("rule-1", "no references here at all")
	if refs != nil {
		t.Errorf("refs = %v, want nil", refs)
	}
}

func TestResolveCrossReferences_WithinCorpus(t *testing.T) {
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{CaseID: "statute:AE"})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}

	// built[0] = Act 12 s.1(a), built[1] = Act 12 s.1(b), built[2] = Act 12 s.2
	refs := []CrossReference{
		{SourceRuleID: built[2].Node.ID, RawText: "Section 1(a)", Section: "1", Clause: "a"},
		{SourceRuleID: built[2].Node.ID, RawText: "Section 2", Section: "2"},
		{SourceRuleID: built[2].Node.ID, RawText: "Section 99", Section: "99"},
	}

	resolved := ResolveCrossReferences(refs, built)
	if len(resolved) != 3 {
		t.Fatalf("len(resolved) = %d, want 3", len(resolved))
	}
	if resolved[0].ResolvedRuleID != built[0].Node.ID {
		t.Errorf("resolved[0].ResolvedRuleID = %q, want %q", resolved[0].ResolvedRuleID, built[0].Node.ID)
	}
	if resolved[1].ResolvedRuleID != built[2].Node.ID {
		t.Errorf("resolved[1].ResolvedRuleID = %q, want %q", resolved[1].ResolvedRuleID, built[2].Node.ID)
	}
	if resolved[2].IsResolved() {
		t.Errorf("resolved[2] should be unresolved (Section 99 does not exist), got %+v", resolved[2])
	}
}

func TestDetectAndResolveAll_And_UnresolvedCrossReferences(t *testing.T) {
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{CaseID: "statute:AE"})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}
	// Manually inject a rule whose text references both an in-corpus
	// section and an out-of-corpus one.
	built[2].Node.Text = "A contract is formed when an offer is accepted. See Section 1 and Section 404."

	all := DetectAndResolveAll(built)
	if len(all) == 0 {
		t.Fatal("expected at least one detected cross-reference")
	}

	unresolved := UnresolvedCrossReferences(all)
	if len(unresolved) == 0 {
		t.Fatal("expected at least one unresolved cross-reference (Section 404)")
	}
	foundUnresolved404 := false
	for _, u := range unresolved {
		if u.Section == "404" {
			foundUnresolved404 = true
		}
	}
	if !foundUnresolved404 {
		t.Errorf("expected Section 404 to be unresolved, got %+v", unresolved)
	}

	resolvedCount := 0
	for _, r := range all {
		if r.IsResolved() {
			resolvedCount++
		}
	}
	if resolvedCount == 0 {
		t.Error("expected at least one resolved cross-reference (Section 1)")
	}
}
