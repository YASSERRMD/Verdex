package issue

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/segmentation"
	"github.com/YASSERRMD/verdex/packages/timeline"
)

// IssueExtractionService orchestrates the full issue-extraction pipeline:
//
//	identify -> map claims -> dedup/merge -> decompose sub-issues
//	  -> link parties/facts -> score confidence -> apply any override
//	  -> persist -> return []irac.IssueNode
//
// This mirrors packages/evidence's EvidenceService and
// packages/segmentation's SegmentationService orchestration pattern: a
// single entry point wires together this package's otherwise
// independent, individually testable building blocks (IssueIdentifier,
// MapClaimsToIssues, Dedup, Decompose, LinkIssues, ScoreConfidence,
// ApplyOverride, PersistIssues).
type IssueExtractionService struct {
	// Identifier performs the issue-identification pass. If nil,
	// NewRuleBasedIdentifier() is used.
	Identifier IssueIdentifier

	// Store persists the extracted irac.IssueNodes. If nil, a fresh
	// graph.InMemoryGraphStore is used.
	Store graph.GraphStore
}

// NewIssueExtractionService constructs an IssueExtractionService with
// sensible defaults for every pluggable dependency left nil:
// RuleBasedIdentifier and an in-memory graph.GraphStore.
func NewIssueExtractionService() *IssueExtractionService {
	return &IssueExtractionService{
		Identifier: NewRuleBasedIdentifier(),
		Store:      graph.NewInMemoryGraphStore(),
	}
}

// ExtractRequest carries the input to
// IssueExtractionService.ExtractIssues.
type ExtractRequest struct {
	// CaseID identifies the case the extracted irac.IssueNodes belong to.
	// Required.
	CaseID string

	// Segments is the batch of segmentation.Segment values to run
	// issue identification over. Required (non-empty).
	Segments []segmentation.Segment

	// Classifications optionally supplies evidence.Classification
	// entries (e.g. from packages/evidence's EvidenceService) to map
	// against the identified issues via claim mapping (see
	// claim_map.go). May be nil/empty.
	Classifications []evidence.Classification

	// Parties optionally supplies the case's timeline.Party records for
	// party linkage (see link.go). May be nil/empty.
	Parties []timeline.Party

	// Overrides optionally supplies a ManualOverride to apply for
	// specific candidate issue IDs, keyed by the ID assigned during
	// identification/decomposition (see IDPrefix below for how IDs are
	// derived). A candidate with no entry here is persisted purely as
	// extracted.
	Overrides map[string]ManualOverride

	// IDPrefix prefixes every generated CandidateIssue/irac.IssueNode ID
	// (e.g. "case-42"). If empty, "issue" is used.
	IDPrefix string

	// CreatedAt stamps every persisted irac.IssueNode's CreatedAt and
	// Provenance.GeneratedAt. If zero, time.Now() is used.
	CreatedAt time.Time
}

// ExtractIssues runs the full pipeline over req and returns the resulting
// irac.IssueNodes, persisted via s.Store.
//
// Returns ErrNoSegments if req.Segments is empty, ErrEmptyInput if
// req.CaseID is blank, or ErrPersistFailed (wrapping the underlying store
// error) if persistence fails partway through.
func (s *IssueExtractionService) ExtractIssues(ctx context.Context, req ExtractRequest) ([]irac.IssueNode, error) {
	if req.CaseID == "" {
		return nil, ErrEmptyInput
	}
	if len(req.Segments) == 0 {
		return nil, ErrNoSegments
	}

	identifier := s.Identifier
	if identifier == nil {
		identifier = NewRuleBasedIdentifier()
	}
	store := s.Store
	if store == nil {
		store = graph.NewInMemoryGraphStore()
	}

	prefix := req.IDPrefix
	if prefix == "" {
		prefix = "issue"
	}
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	// 1. identify
	candidates, err := identifier.Identify(ctx, req.Segments)
	if err != nil {
		return nil, err
	}

	// Assign stable IDs before any downstream stage needs to reference
	// them (claim mapping/linking operate positionally on the slice, but
	// dedup/decompose and persistence need real IDs).
	candidates = assignIDs(candidates, prefix)

	// 2. map claims
	segmentText := make(map[string]string, len(req.Segments))
	for _, seg := range req.Segments {
		segmentText[seg.ID] = seg.Text
	}
	claimLinks := MapClaimsToIssues(req.Classifications, candidates, segmentText)

	// 3. dedup/merge
	candidates = Dedup(candidates)
	candidates = assignIDs(candidates, prefix)

	// 4. decompose sub-issues
	candidates = decomposeAll(candidates)

	// 5. link parties/facts
	links := LinkIssues(candidates, req.Parties, segmentText)
	linksByIndex := make(map[int]IssueLink, len(links))
	for i, l := range links {
		linksByIndex[i] = l
	}

	// 6. score confidence
	candidates = ScoreConfidence(candidates, claimLinks)

	// 7. apply any override
	for i, c := range candidates {
		if override, ok := req.Overrides[c.ID]; ok {
			overridden, err := ApplyOverride(c, override)
			if err != nil {
				return nil, err
			}
			candidates[i] = overridden.CandidateIssue
		}
	}

	// 8. persist
	nodes, err := PersistIssues(ctx, store, candidates, req.CaseID, createdAt, linksByIndex)
	if err != nil {
		return nodes, err
	}

	return nodes, nil
}

// assignIDs assigns a stable, deterministic ID to every candidate that
// does not already have one, in slice order. Sub-issues (ParentIssueID
// non-nil) and already-ID'd candidates (e.g. from a prior pass) are left
// untouched.
func assignIDs(candidates []CandidateIssue, prefix string) []CandidateIssue {
	out := make([]CandidateIssue, len(candidates))
	for i, c := range candidates {
		if c.ID == "" {
			c.ID = fmt.Sprintf("%s-%d", prefix, i)
		}
		out[i] = c
	}
	return out
}

// decomposeAll runs Decompose over every top-level (non-sub-issue)
// candidate, flattening the parent-plus-sub-issues result of each into a
// single slice.
func decomposeAll(candidates []CandidateIssue) []CandidateIssue {
	out := make([]CandidateIssue, 0, len(candidates))
	for _, c := range candidates {
		if c.ParentIssueID != nil {
			out = append(out, c)
			continue
		}
		out = append(out, Decompose(c, c.ID)...)
	}
	return out
}
