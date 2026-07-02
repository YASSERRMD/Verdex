package lawapplication_test

import (
	"reflect"
	"testing"

	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

func TestMapIssueToControllingRules_UnionOfGovernsAndArguments(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{ArgumentID: "arg-1", IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingRuleIDs: []string{"rule-2", "rule-3"}},
		{ArgumentID: "arg-2", IssueNodeID: "issue-2", PartyID: "defendant", SupportingRuleIDs: []string{"rule-9"}},
	}

	got := lawapplication.MapIssueToControllingRules("issue-1", []string{"rule-1", "rule-2"}, args)
	want := []string{"rule-1", "rule-2", "rule-3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MapIssueToControllingRules = %v, want %v", got, want)
	}
}

func TestMapIssueToControllingRules_DeduplicatesAndSorts(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{IssueNodeID: "issue-1", SupportingRuleIDs: []string{"rule-1", "rule-1"}},
	}
	got := lawapplication.MapIssueToControllingRules("issue-1", []string{"rule-1"}, args)
	want := []string{"rule-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MapIssueToControllingRules = %v, want %v", got, want)
	}
}

func TestMapIssueToControllingRules_IgnoresOtherIssues(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{IssueNodeID: "issue-OTHER", SupportingRuleIDs: []string{"rule-x"}},
	}
	got := lawapplication.MapIssueToControllingRules("issue-1", nil, args)
	if len(got) != 0 {
		t.Errorf("MapIssueToControllingRules = %v, want empty", got)
	}
}

func TestMapIssueToControllingRules_NoRulesAtAll(t *testing.T) {
	got := lawapplication.MapIssueToControllingRules("issue-1", nil, nil)
	if len(got) != 0 {
		t.Errorf("MapIssueToControllingRules = %v, want empty", got)
	}
}
