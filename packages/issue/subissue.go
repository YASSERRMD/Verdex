package issue

import (
	"regexp"
	"strconv"
	"strings"
)

// conjunctionSplit matches an " and " (or " and whether ") conjunction
// joining two distinct legal questions within a single issue's Text, used
// by Decompose to split compound issues into a parent plus sub-issues.
// Matching is case-insensitive and requires whitespace on both sides so
// words like "demand" are not mistakenly split on.
var conjunctionSplit = regexp.MustCompile(`(?i)\s+and\s+(?:whether\s+)?`)

// minSubIssueWords is the minimum word count a conjunction-split fragment
// must have to be treated as its own distinct legal question rather than a
// trailing clause fragment (e.g. "the deposit and fees" should not split
// on "fees").
const minSubIssueWords = 3

// Decompose splits a compound CandidateIssue's Text — one containing an
// "and" conjunction joining two distinct legal questions (e.g. "whether
// the contract was breached and whether damages are owed") — into a
// parent issue plus sub-issues, each carrying ParentIssueID pointing back
// to the parent's ID.
//
// parentID is the ID to assign to the (unmodified) parent CandidateIssue;
// callers are responsible for ID assignment/uniqueness elsewhere in the
// pipeline (see service.go). If parent.Text is not decomposable (no
// conjunction split, or a fragment is too short to be its own question),
// Decompose returns only the parent unchanged, with no sub-issues.
func Decompose(parent CandidateIssue, parentID string) []CandidateIssue {
	parent.ID = parentID

	parts := conjunctionSplit.Split(parent.Text, -1)
	if len(parts) < 2 {
		return []CandidateIssue{parent}
	}

	fragments := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" || len(strings.Fields(trimmed)) < minSubIssueWords {
			// Not decomposable: at least one fragment is too short to be
			// its own legal question, so treat the whole text as a
			// single (non-compound) issue.
			return []CandidateIssue{parent}
		}
		fragments = append(fragments, trimmed)
	}

	out := make([]CandidateIssue, 0, len(fragments)+1)
	out = append(out, parent)

	for i, frag := range fragments {
		sub := CandidateIssue{
			ID:            subIssueID(parentID, i),
			Text:          frag,
			SourceSpans:   parent.SourceSpans,
			Confidence:    parent.Confidence,
			ParentIssueID: strPtr(parentID),
		}
		out = append(out, sub)
	}

	return out
}

// subIssueID derives a deterministic sub-issue ID from its parent's ID and
// its zero-based fragment index.
func subIssueID(parentID string, index int) string {
	return parentID + "-sub-" + strconv.Itoa(index)
}

// strPtr returns a pointer to s, used to populate CandidateIssue's
// *string ParentIssueID field from a local value.
func strPtr(s string) *string { return &s }
