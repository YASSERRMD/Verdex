package hybridretrieval

import "testing"

func TestDedupAndDiversify_CapsPerAnchor(t *testing.T) {
	items := []Item{
		{NodeID: "a", AnchorNodeID: "seed-1", CombinedScore: 0.9, Text: "text a"},
		{NodeID: "b", AnchorNodeID: "seed-1", CombinedScore: 0.8, Text: "text b"},
		{NodeID: "c", AnchorNodeID: "seed-1", CombinedScore: 0.7, Text: "text c"},
	}
	out := dedupAndDiversify(items, 10, 2)
	if len(out) != 2 {
		t.Fatalf("expected 2 items after capping to MaxPerAnchor=2, got %d: %+v", len(out), out)
	}
	if out[0].NodeID != "a" || out[1].NodeID != "b" {
		t.Errorf("expected the top 2 by score to survive the cap, got %+v", out)
	}
}

func TestDedupAndDiversify_CollapsesNearDuplicateText(t *testing.T) {
	items := []Item{
		{NodeID: "a", CombinedScore: 0.9, Text: "The Seller Did Not Deliver."},
		{NodeID: "b", CombinedScore: 0.8, Text: "  the seller did not deliver.  "},
	}
	out := dedupAndDiversify(items, 10, 10)
	if len(out) != 1 {
		t.Fatalf("expected near-duplicate text to collapse to 1 item, got %d: %+v", len(out), out)
	}
	if out[0].NodeID != "a" {
		t.Errorf("expected the higher-scoring duplicate to survive, got %q", out[0].NodeID)
	}
}

func TestDedupAndDiversify_RespectsTopK(t *testing.T) {
	items := []Item{
		{NodeID: "a", CombinedScore: 0.9, Text: "a"},
		{NodeID: "b", CombinedScore: 0.8, Text: "b"},
		{NodeID: "c", CombinedScore: 0.7, Text: "c"},
	}
	out := dedupAndDiversify(items, 2, 10)
	if len(out) != 2 {
		t.Fatalf("expected TopK=2 to cap results, got %d", len(out))
	}
}

func TestDedupAndDiversify_EmptyTextNeverCollides(t *testing.T) {
	items := []Item{
		{NodeID: "a", CombinedScore: 0.9, Text: ""},
		{NodeID: "b", CombinedScore: 0.8, Text: ""},
	}
	out := dedupAndDiversify(items, 10, 10)
	if len(out) != 2 {
		t.Errorf("expected empty-text items to never be treated as duplicates of each other, got %d", len(out))
	}
}

func TestNormalizeText(t *testing.T) {
	cases := map[string]string{
		"Hello   World": "hello world",
		"  trim me  ":   "trim me",
		"":              "",
	}
	for in, want := range cases {
		if got := normalizeText(in); got != want {
			t.Errorf("normalizeText(%q) = %q, want %q", in, got, want)
		}
	}
}
