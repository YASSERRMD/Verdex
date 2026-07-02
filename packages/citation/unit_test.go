package citation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestFromItem(t *testing.T) {
	item := hybridretrieval.Item{
		NodeID:        "rule-1",
		NodeType:      irac.NodeRule,
		Text:          "no person shall...",
		AnchorNodeID:  "issue-1",
		CombinedScore: 0.75,
	}

	unit := citation.FromItem("case-1", item)

	if unit.NodeID != "rule-1" {
		t.Errorf("NodeID = %q, want rule-1", unit.NodeID)
	}
	if unit.CaseID != "case-1" {
		t.Errorf("CaseID = %q, want case-1", unit.CaseID)
	}
	if unit.NodeType != irac.NodeRule {
		t.Errorf("NodeType = %q, want %q", unit.NodeType, irac.NodeRule)
	}
	if unit.Text != item.Text {
		t.Errorf("Text = %q, want %q", unit.Text, item.Text)
	}
	if unit.AnchorNodeID != "issue-1" {
		t.Errorf("AnchorNodeID = %q, want issue-1", unit.AnchorNodeID)
	}
	if unit.CombinedScore != 0.75 {
		t.Errorf("CombinedScore = %v, want 0.75", unit.CombinedScore)
	}
	if unit.HasCitation() {
		t.Error("HasCitation() = true before resolution, want false")
	}
	if unit.HasSpans() {
		t.Error("HasSpans() = true with no spans set, want false")
	}
}

func TestFromItems(t *testing.T) {
	items := []hybridretrieval.Item{
		{NodeID: "a"},
		{NodeID: "b"},
		{NodeID: "c"},
	}

	units := citation.FromItems("case-1", items)

	if len(units) != 3 {
		t.Fatalf("len(units) = %d, want 3", len(units))
	}
	for i, u := range units {
		if u.NodeID != items[i].NodeID {
			t.Errorf("units[%d].NodeID = %q, want %q", i, u.NodeID, items[i].NodeID)
		}
		if u.CaseID != "case-1" {
			t.Errorf("units[%d].CaseID = %q, want case-1", i, u.CaseID)
		}
	}
}

func TestCitedUnitHasSpansAndCitation(t *testing.T) {
	unit := citation.CitedUnit{
		Spans:    irac.Spans{{Start: 0, End: 5}},
		Citation: "Act 1, s.1",
	}
	if !unit.HasSpans() {
		t.Error("HasSpans() = false, want true")
	}
	if !unit.HasCitation() {
		t.Error("HasCitation() = false, want true")
	}
}

func TestOriginIsValid(t *testing.T) {
	cases := []struct {
		origin citation.Origin
		want   bool
	}{
		{citation.OriginUnknown, true},
		{citation.OriginStatute, true},
		{citation.OriginPrecedent, true},
		{citation.Origin("bogus"), false},
	}
	for _, c := range cases {
		if got := c.origin.IsValid(); got != c.want {
			t.Errorf("Origin(%q).IsValid() = %v, want %v", c.origin, got, c.want)
		}
	}
}
