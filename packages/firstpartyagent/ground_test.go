package firstpartyagent

import "testing"

func TestGroundArgument_AllIDsReal_ReturnsGroundedTrue(t *testing.T) {
	allowed := map[string]struct{}{"fact-1": {}, "rule-1": {}}
	ma := modelArgument{
		IssueNodeID:       "issue-1",
		Claim:             "claim text",
		SupportingFactIDs: []string{"fact-1"},
		SupportingRuleIDs: []string{"rule-1"},
	}

	arg, ok := groundArgument(ma, allowed)
	if !ok {
		t.Fatal("groundArgument returned ok=false, want true")
	}
	if !arg.Grounded {
		t.Fatal("Grounded = false, want true")
	}
	if len(arg.FabricatedNodeIDs) != 0 {
		t.Fatalf("FabricatedNodeIDs = %v, want empty", arg.FabricatedNodeIDs)
	}
}

func TestGroundArgument_SomeFabricated_StripsAndFlags(t *testing.T) {
	allowed := map[string]struct{}{"fact-1": {}}
	ma := modelArgument{
		IssueNodeID:       "issue-1",
		SupportingFactIDs: []string{"fact-1", "fact-fake"},
	}

	arg, ok := groundArgument(ma, allowed)
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
	allowed := map[string]struct{}{"fact-1": {}}
	ma := modelArgument{
		IssueNodeID:       "issue-1",
		SupportingFactIDs: []string{"fact-fake-1"},
		SupportingRuleIDs: []string{"rule-fake-1"},
	}

	_, ok := groundArgument(ma, allowed)
	if ok {
		t.Fatal("groundArgument returned ok=true, want false (no real IDs at all)")
	}
}

func TestGroundArgument_EmptyInput_ReturnsOkFalse(t *testing.T) {
	ma := modelArgument{IssueNodeID: "issue-1"}
	_, ok := groundArgument(ma, map[string]struct{}{"fact-1": {}})
	if ok {
		t.Fatal("groundArgument returned ok=true, want false (no supporting IDs at all)")
	}
}
