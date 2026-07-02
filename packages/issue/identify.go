package issue

import (
	"context"
	"regexp"
	"strings"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// CandidateIssue is a legal or factual question identified from case text,
// before it is persisted as an irac.IssueNode (see persist.go). It carries
// its own provenance (SourceSpans) and a Confidence score, mirroring
// packages/evidence's Classification convention of "confidence, not
// certainty" for every automated determination.
type CandidateIssue struct {
	// ID uniquely identifies this candidate issue within a single
	// extraction run. Empty until assigned by IssueIdentifier or a later
	// pipeline stage.
	ID string

	// Text is the human-readable statement of the issue (typically the
	// dispute-indicating segment text, or a decomposed sub-question — see
	// subissue.go).
	Text string

	// SourceSpans traces this issue's text back to the ingested source
	// document(s) it was drawn from.
	SourceSpans []irac.SourceSpan

	// Confidence is this candidate's confidence score, in the closed
	// interval [0, 1]. Refined across the pipeline by confidence.go.
	Confidence float64

	// ParentIssueID, when non-nil, identifies the CandidateIssue this
	// issue was decomposed from (see subissue.go). Nil means this issue
	// is not a sub-issue of another candidate.
	ParentIssueID *string
}

// IssueIdentifier performs an issue-identification pass over a batch of
// packages/segmentation Segments, returning the CandidateIssues it finds.
//
// This interface is the pluggable extension point for issue
// identification: the default implementation in this file
// (RuleBasedIdentifier) is a deterministic function of segment text
// patterns, mirroring packages/evidence's Classifier and
// packages/segmentation's "no ML models, rule based" design principle. A
// future phase can swap in a real model by implementing this same
// interface — no caller of IssueIdentifier needs to change.
type IssueIdentifier interface {
	// Identify inspects segments and returns every CandidateIssue found.
	// ctx allows implementations that call out to an external model or
	// service to respect cancellation/deadlines.
	//
	// Returns ErrNoSegments if segments is empty.
	Identify(ctx context.Context, segments []segmentation.Segment) ([]CandidateIssue, error)
}

// RuleBasedIdentifier is the default, deterministic IssueIdentifier
// implementation. It detects dispute/question-indicating language patterns
// over segment text — mirroring packages/evidence's RuleBasedClassifier
// lexical-heuristic approach.
//
// RuleBasedIdentifier performs no machine learning and calls out to no
// external service, so its output is fully reproducible given the same
// input segments.
type RuleBasedIdentifier struct{}

// NewRuleBasedIdentifier constructs a RuleBasedIdentifier. It has no
// configuration today; the constructor exists so call sites can be
// updated uniformly if configuration is added later.
func NewRuleBasedIdentifier() *RuleBasedIdentifier {
	return &RuleBasedIdentifier{}
}

// disputeMarkers are lexical cues that a segment poses or references a
// disputed legal or factual question. Ordered roughly by specificity;
// disputePattern below scores confidence per matched marker.
var disputeMarkers = []struct {
	pattern    *regexp.Regexp
	confidence float64
}{
	{regexp.MustCompile(`(?i)\bwhether\b`), 0.85},
	{regexp.MustCompile(`(?i)\bin dispute\b|\bis disputed\b|\bdisputes?\b`), 0.75},
	{regexp.MustCompile(`(?i)\bclaims? that\b`), 0.7},
	{regexp.MustCompile(`(?i)\bdenies?\b|\bdenied\b`), 0.7},
	{regexp.MustCompile(`(?i)\balleges?\b|\balleged\b`), 0.65},
	{regexp.MustCompile(`(?i)\bcontends?\b|\bcontested\b`), 0.6},
	{regexp.MustCompile(`(?i)\?\s*$`), 0.5},
}

// Identify implements IssueIdentifier. It scans each segment's text for
// dispute-indicating language patterns (e.g. "whether", "dispute", "claims
// that", "denies") and, separately, looks for contradictory statement
// pairs across adjacent segmentation.SegmentStatement segments (see
// findContradictoryPairs). Each match becomes one CandidateIssue carrying
// the originating segment's SourceSpan and a heuristic confidence.
func (r *RuleBasedIdentifier) Identify(_ context.Context, segments []segmentation.Segment) ([]CandidateIssue, error) {
	if len(segments) == 0 {
		return nil, ErrNoSegments
	}

	var out []CandidateIssue
	for _, seg := range segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}

		if marker, conf, ok := bestDisputeMarker(text); ok {
			_ = marker
			out = append(out, CandidateIssue{
				Text:        text,
				SourceSpans: []irac.SourceSpan{segmentSpan(seg)},
				Confidence:  conf,
			})
		}
	}

	out = append(out, findContradictoryPairs(segments)...)

	return out, nil
}

// bestDisputeMarker returns the highest-confidence dispute marker pattern
// matching text, if any.
func bestDisputeMarker(text string) (string, float64, bool) {
	best := 0.0
	matched := false
	for _, m := range disputeMarkers {
		if m.pattern.MatchString(text) && m.confidence > best {
			best = m.confidence
			matched = true
		}
	}
	return "", best, matched
}

// segmentSpan converts a segmentation.Segment's SourceSpan into the
// locally-defined irac.SourceSpan shape (see packages/irac/span.go's doc
// comment on why irac.SourceSpan is not directly aliased to
// packages/segmentation's type).
func segmentSpan(seg segmentation.Segment) irac.SourceSpan {
	return irac.SourceSpan{
		Start:   seg.Span.Start,
		End:     seg.Span.End,
		Page:    seg.Span.Page,
		StartMS: seg.Span.StartMS,
		EndMS:   seg.Span.EndMS,
	}
}

// contradictionCues are lightweight lexical opposites checked when
// deciding whether two adjacent statement segments contradict one another.
var contradictionCues = [][2]string{
	{"did", "did not"},
	{"was", "was not"},
	{"agreed", "denied"},
	{"agreed", "disagreed"},
	{"paid", "did not pay"},
	{"received", "did not receive"},
	{"present", "absent"},
	{"true", "false"},
	{"yes", "no"},
}

// findContradictoryPairs scans adjacent SegmentStatement segments for
// contradictory statement pairs (e.g. one party's "I paid the deposit"
// followed by another's "I did not receive any deposit"), producing one
// CandidateIssue per contradictory pair found, with SourceSpans covering
// both segments.
func findContradictoryPairs(segments []segmentation.Segment) []CandidateIssue {
	var out []CandidateIssue

	statements := make([]segmentation.Segment, 0, len(segments))
	for _, seg := range segments {
		if seg.Type == segmentation.SegmentStatement && strings.TrimSpace(seg.Text) != "" {
			statements = append(statements, seg)
		}
	}

	for i := 0; i < len(statements); i++ {
		for j := i + 1; j < len(statements); j++ {
			a, b := statements[i], statements[j]
			if a.SpeakerLabel != "" && a.SpeakerLabel == b.SpeakerLabel {
				// Same speaker contradicting themselves is not the
				// dispute-between-parties signal this heuristic targets.
				continue
			}
			if cuesContradict(a.Text, b.Text) {
				out = append(out, CandidateIssue{
					Text:        "whether " + strings.TrimSpace(a.Text) + " (disputed: " + strings.TrimSpace(b.Text) + ")",
					SourceSpans: []irac.SourceSpan{segmentSpan(a), segmentSpan(b)},
					Confidence:  0.6,
				})
			}
		}
	}
	return out
}

// cuesContradict reports whether textA and textB contain opposite cue
// terms from contradictionCues.
func cuesContradict(textA, textB string) bool {
	la, lb := strings.ToLower(textA), strings.ToLower(textB)
	for _, pair := range contradictionCues {
		if strings.Contains(la, pair[0]) && strings.Contains(lb, pair[1]) {
			return true
		}
		if strings.Contains(la, pair[1]) && strings.Contains(lb, pair[0]) {
			return true
		}
	}
	return false
}
