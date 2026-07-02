package statute

import "testing"

func TestParseHierarchy_ActSectionClause(t *testing.T) {
	raw := RawStatute{
		ActNumber: "12",
		ActTitle:  "Contracts Act",
		Body: `Section 1. Definitions
(a) "party" means a natural or legal person.
(b) "contract" means an agreement enforceable by law.
Section 2. Formation
A contract is formed when an offer is accepted.
`,
	}

	act, err := ParseHierarchy(raw)
	if err != nil {
		t.Fatalf("ParseHierarchy() error = %v", err)
	}
	if act.Level != LevelAct || act.Number != "12" || act.Title != "Contracts Act" {
		t.Errorf("act = %+v, want Level=act Number=12 Title=Contracts Act", act)
	}
	if len(act.Children) != 2 {
		t.Fatalf("len(act.Children) = %d, want 2", len(act.Children))
	}

	section1 := act.Children[0]
	if section1.Level != LevelSection || section1.Number != "1" || section1.Title != "Definitions" {
		t.Errorf("section1 = %+v, want Level=section Number=1 Title=Definitions", section1)
	}
	if len(section1.Children) != 2 {
		t.Fatalf("len(section1.Children) = %d, want 2", len(section1.Children))
	}
	if section1.Children[0].Number != "a" || section1.Children[1].Number != "b" {
		t.Errorf("clause numbers = %q, %q, want a, b", section1.Children[0].Number, section1.Children[1].Number)
	}
	if section1.Children[0].Text == "" {
		t.Error("clause (a) text is empty")
	}

	section2 := act.Children[1]
	if section2.Level != LevelSection || section2.Number != "2" || section2.Title != "Formation" {
		t.Errorf("section2 = %+v, want Level=section Number=2 Title=Formation", section2)
	}
	if !section2.IsLeaf() {
		t.Error("section2 should be a leaf (no clauses)")
	}
	if section2.Text == "" {
		t.Error("section2 text is empty")
	}
}

func TestParseHierarchy_BareActNoSections(t *testing.T) {
	raw := RawStatute{ActNumber: "5", ActTitle: "Bare Act", Body: "This act has no sections, just prose."}
	act, err := ParseHierarchy(raw)
	if err != nil {
		t.Fatalf("ParseHierarchy() error = %v", err)
	}
	if !act.IsLeaf() {
		t.Error("bare act should be a leaf")
	}
	if act.Text == "" {
		t.Error("bare act text should not be empty")
	}
}

func TestParseHierarchy_EmptyActNumber(t *testing.T) {
	_, err := ParseHierarchy(RawStatute{ActNumber: "", Body: "Section 1. X"})
	if err == nil {
		t.Fatal("ParseHierarchy() error = nil, want error for empty ActNumber")
	}
}

func TestStatuteNode_WalkAndLeaves(t *testing.T) {
	act, err := ParseHierarchy(RawStatute{
		ActNumber: "1",
		Body: `Section 1. A
(a) clause a text
(b) clause b text
Section 2. B
body text for the second section`,
	})
	if err != nil {
		t.Fatalf("ParseHierarchy() error = %v", err)
	}

	var visited []string
	act.Walk(func(n *StatuteNode) bool {
		visited = append(visited, string(n.Level)+":"+n.Number)
		return true
	})
	want := []string{"act:1", "section:1", "clause:a", "clause:b", "section:2"}
	if len(visited) != len(want) {
		t.Fatalf("visited = %v, want %v", visited, want)
	}
	for i := range want {
		if visited[i] != want[i] {
			t.Errorf("visited[%d] = %q, want %q", i, visited[i], want[i])
		}
	}

	leaves := act.Leaves()
	if len(leaves) != 3 {
		t.Fatalf("len(leaves) = %d, want 3 (clause a, clause b, section 2)", len(leaves))
	}
}
