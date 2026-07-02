package statute

import (
	"regexp"
	"strings"
)

// StatuteNode is one node in an act's structural tree: an Act at the
// root, containing Sections, each of which may contain Clauses. The same
// shape is reused at every level (mirroring packages/category's
// Taxonomy convention of a small recursive shape rather than three
// distinct Go types) so callers can walk the tree uniformly; Level
// distinguishes which tier a given node occupies.
type StatuteNode struct {
	// Level identifies this node's position in the Act -> Section ->
	// Clause hierarchy.
	Level StatuteLevel `json:"level"`

	// Number is the machine-readable identifier at this level (e.g. the
	// act number, "12" for a section, "(a)" for a clause).
	Number string `json:"number"`

	// Title is the human-readable heading at this level, when present
	// (sections and acts commonly have titles; clauses often do not).
	Title string `json:"title,omitempty"`

	// Text is this node's own body text, excluding the text of its
	// Children (a Section with Clauses carries only its own
	// chapeau/lead-in text here, if any).
	Text string `json:"text"`

	// Children lists the nodes nested directly beneath this one (Sections
	// under an Act, Clauses under a Section). Leaf nodes have no
	// Children.
	Children []*StatuteNode `json:"children,omitempty"`
}

// StatuteLevel identifies which tier of the Act -> Section -> Clause
// hierarchy a StatuteNode occupies.
type StatuteLevel string

const (
	// LevelAct is the top-level statute/act node.
	LevelAct StatuteLevel = "act"

	// LevelSection is a section nested directly under an Act.
	LevelSection StatuteLevel = "section"

	// LevelClause is a clause nested directly under a Section.
	LevelClause StatuteLevel = "clause"
)

// IsLeaf reports whether n has no Children.
func (n *StatuteNode) IsLeaf() bool {
	return len(n.Children) == 0
}

// Walk visits n and every descendant, depth-first, pre-order, calling fn
// on each. Walk stops early (without visiting remaining nodes) if fn
// returns false.
func (n *StatuteNode) Walk(fn func(*StatuteNode) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for _, c := range n.Children {
		c.Walk(fn)
	}
}

// Leaves returns every leaf StatuteNode reachable from n, in document
// order. If n itself is a leaf (including a bare Act with no Sections),
// the returned slice contains only n.
func (n *StatuteNode) Leaves() []*StatuteNode {
	var out []*StatuteNode
	n.Walk(func(cur *StatuteNode) bool {
		if cur.IsLeaf() {
			out = append(out, cur)
		}
		return true
	})
	return out
}

// sectionHeaderRe matches a line introducing a new section, e.g.
// "Section 12. Definitions" or "Section 12: Definitions" or bare
// "Section 12".
var sectionHeaderRe = regexp.MustCompile(`(?i)^Section\s+([^\s.]+)\.?\s*[:.\-]?\s*(.*)$`)

// clauseHeaderRe matches a line introducing a new clause within a
// section, e.g. "(a) the parties shall..." or "(1) ...".
var clauseHeaderRe = regexp.MustCompile(`^\(([a-zA-Z0-9]+)\)\s*(.*)$`)

// ParseHierarchy parses raw.Body into a StatuteNode tree rooted at the
// act itself. Lines matching sectionHeaderRe start a new LevelSection
// child of the act; within a section, lines matching clauseHeaderRe
// start a new LevelClause child of that section. Any other line is
// appended to the Text of whichever node (act, section, or clause) is
// currently open.
//
// Returns ErrMalformedCorpus if raw.ActNumber is empty.
func ParseHierarchy(raw RawStatute) (*StatuteNode, error) {
	if strings.TrimSpace(raw.ActNumber) == "" {
		return nil, ErrMalformedCorpus
	}

	act := &StatuteNode{
		Level:  LevelAct,
		Number: raw.ActNumber,
		Title:  raw.ActTitle,
	}

	var actText, sectionText, clauseText []string
	var currentSection *StatuteNode
	var currentClause *StatuteNode

	flushClause := func() {
		if currentClause == nil {
			return
		}
		currentClause.Text = strings.TrimSpace(strings.Join(clauseText, "\n"))
		clauseText = nil
		if currentSection != nil {
			currentSection.Children = append(currentSection.Children, currentClause)
		}
		currentClause = nil
	}
	flushSection := func() {
		flushClause()
		if currentSection == nil {
			return
		}
		currentSection.Text = strings.TrimSpace(strings.Join(sectionText, "\n"))
		sectionText = nil
		act.Children = append(act.Children, currentSection)
		currentSection = nil
	}

	lines := strings.Split(raw.Body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if m := sectionHeaderRe.FindStringSubmatch(trimmed); m != nil {
			flushSection()
			currentSection = &StatuteNode{
				Level:  LevelSection,
				Number: m[1],
				Title:  strings.TrimSpace(m[2]),
			}
			continue
		}

		if currentSection != nil {
			if m := clauseHeaderRe.FindStringSubmatch(trimmed); m != nil {
				flushClause()
				currentClause = &StatuteNode{
					Level:  LevelClause,
					Number: m[1],
				}
				clauseText = append(clauseText, m[2])
				continue
			}
			if currentClause != nil {
				clauseText = append(clauseText, trimmed)
				continue
			}
			sectionText = append(sectionText, trimmed)
			continue
		}

		actText = append(actText, trimmed)
	}
	flushSection()

	act.Text = strings.TrimSpace(strings.Join(actText, "\n"))
	return act, nil
}
