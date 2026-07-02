package secondpartyagent

import "testing"

func TestGroundArgument_AllIDsReal_ReturnsGroundedTrue(t *testing.T) {
	allowedNodes := map[string]struct{}{"fact-1": {}, "rule-1": {}}
	allowedArgs := map[string]struct{}{"opp-arg-1": {}}
	ma := modelArgument{
		IssueNodeID:       "issue-1",
		Claim:             "claim text",
		SupportingFactIDs: []string{"fact-1"},
		SupportingRuleIDs: []string{"rule-1"},
		RebutsArgumentIDs: []string{"opp-arg-1"},
	}

	arg, ok := groundArgument(ma, allowedNodes, allowedArgs)
	if !ok {
		t.Fatal("groundArgument returned ok=false, want true")
	}
	if !arg.Grounded {
		t.Fatal("Grounded = false, want true")
	}
	if len(arg.FabricatedNodeIDs) != 0 {
		t.Fatalf("FabricatedNodeIDs = %v, want empty", arg.FabricatedNodeIDs)
	}
	if len(arg.RebutsArgumentIDs) != 1 || arg.RebutsArgumentIDs[0] != "opp-arg-1" {
		t.Fatalf("RebutsArgumentIDs = %v, want [opp-arg-1]", arg.RebutsArgumentIDs)
	}
	if len(arg.FabricatedRebuttalIDs) != 0 {
		t.Fatalf("FabricatedRebuttalIDs = %v, want empty", arg.FabricatedRebuttalIDs)
	}
}

func TestGroundArgument_SomeFabricated_StripsAndFlags(t *testing.T) {
	allowedNodes := map[string]struct{}{"fact-1": {}}
	ma := modelArgument{
		IssueNodeID:       "issue-1",
		SupportingFactIDs: []string{"fact-1", "fact-fake"},
	}

	arg, ok := groundArgument(ma, allowedNodes, map[string]struct{}{})
	if !ok {
		t.Fatal("groundArgument returned ok=false, want true (one real ID remains)")
	}
	if arg.Grounded {
		t.Fatal("Grounded = true, want false")
	}
	if len(arg.SupportingFactIDs) != 1 || arg.SupportingFactIDs[0] != "fact-1" {
		t.Fatalf("SupportingFactIDs = %v, want [fact-1]", arg.SupportingFactIDs)
	}
	if len(arg.FabricatedNodeIDs) != 1 || arg.FabricatedNodeIDs[0] != "fact-fake" {
		t.Fatalf("FabricatedNodeIDs = %v, want [fact-fake]", arg.FabricatedNodeIDs)
	}
}

func TestGroundArgument_AllFabricated_ReturnsOkFalse(t *testing.T) {
	allowedNodes := map[string]struct{}{"fact-1": {}}
	ma := modelArgument{
		IssueNodeID:       "issue-1",
		SupportingFactIDs: []string{"fact-fake-1"},
		SupportingRuleIDs: []string{"rule-fake-1"},
	}

	_, ok := groundArgument(ma, allowedNodes, map[string]struct{}{})
	if ok {
		t.Fatal("groundArgument returned ok=true, want false (no real IDs at all)")
	}
}

func TestGroundArgument_EmptyInput_ReturnsOkFalse(t *testing.T) {
	ma := modelArgument{IssueNodeID: "issue-1"}
	_, ok := groundArgument(ma, map[string]struct{}{"fact-1": {}}, map[string]struct{}{})
	if ok {
		t.Fatal("groundArgument returned ok=true, want false (no supporting IDs at all)")
	}
}

// TestGroundArgument_FabricatedRebuttalID_StrippedButArgumentSurvives
// proves the rebuttal-ID grounding addition: a second-party argument
// citing a real supporting fact but a non-existent opposing-argument ID
// in RebutsArgumentIDs must survive with the fabricated rebuttal target
// stripped and recorded, exactly mirroring the fact/rule fabrication
// contract but for argument-ID grounding.
func TestGroundArgument_FabricatedRebuttalID_StrippedButArgumentSurvives(t *testing.T) {
	allowedNodes := map[string]struct{}{"fact-1": {}}
	allowedArgs := map[string]struct{}{"opp-arg-1": {}}
	ma := modelArgument{
		IssueNodeID:       "issue-1",
		Claim:             "claim text",
		SupportingFactIDs: []string{"fact-1"},
		RebutsArgumentIDs: []string{"opp-arg-1", "opp-arg-does-not-exist"},
	}

	arg, ok := groundArgument(ma, allowedNodes, allowedArgs)
	if !ok {
		t.Fatal("groundArgument returned ok=false, want true (real supporting fact remains)")
	}
	if len(arg.RebutsArgumentIDs) != 1 || arg.RebutsArgumentIDs[0] != "opp-arg-1" {
		t.Fatalf("RebutsArgumentIDs = %v, want [opp-arg-1]", arg.RebutsArgumentIDs)
	}
	if len(arg.FabricatedRebuttalIDs) != 1 || arg.FabricatedRebuttalIDs[0] != "opp-arg-does-not-exist" {
		t.Fatalf("FabricatedRebuttalIDs = %v, want [opp-arg-does-not-exist]", arg.FabricatedRebuttalIDs)
	}
	// A fabricated rebuttal ID must not affect the Grounded flag, which
	// tracks only supporting fact/rule fabrication.
	if !arg.Grounded {
		t.Fatal("Grounded = false, want true (only supporting IDs, not rebuttal IDs, affect Grounded)")
	}
}

// TestGroundArgument_AllRebuttalIDsFabricated_ArgumentStillKeptWithEmptyRebuttals
// confirms an argument is never dropped purely for having zero valid
// rebuttal targets, as long as it has real supporting evidence — rebuttal
// linkage is an enrichment, not a precondition for the argument existing.
func TestGroundArgument_AllRebuttalIDsFabricated_ArgumentStillKeptWithEmptyRebuttals(t *testing.T) {
	allowedNodes := map[string]struct{}{"fact-1": {}}
	ma := modelArgument{
		IssueNodeID:       "issue-1",
		SupportingFactIDs: []string{"fact-1"},
		RebutsArgumentIDs: []string{"opp-arg-imaginary"},
	}

	arg, ok := groundArgument(ma, allowedNodes, map[string]struct{}{})
	if !ok {
		t.Fatal("groundArgument returned ok=false, want true")
	}
	if len(arg.RebutsArgumentIDs) != 0 {
		t.Fatalf("RebutsArgumentIDs = %v, want empty", arg.RebutsArgumentIDs)
	}
	if len(arg.FabricatedRebuttalIDs) != 1 || arg.FabricatedRebuttalIDs[0] != "opp-arg-imaginary" {
		t.Fatalf("FabricatedRebuttalIDs = %v, want [opp-arg-imaginary]", arg.FabricatedRebuttalIDs)
	}
}
