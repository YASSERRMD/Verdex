package fact

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/timeline"
)

// FactConstructionService orchestrates the full fact-construction
// pipeline:
//
//	build -> attach evidence ref -> attribute party -> flag dispute
//	  -> anchor temporally -> link corroboration -> score reliability
//	  -> persist -> return []irac.FactNode
//
// This mirrors packages/issue's IssueExtractionService orchestration
// pattern: a single entry point wires together this package's otherwise
// independent, individually testable building blocks (BuildFactNode,
// NewEvidenceRef, AttributeParty, DetermineDisputeStatus, AnchorToEvent,
// DetectCorroboration, ReliabilityScore, PersistFacts).
type FactConstructionService struct {
	// Store persists the constructed irac.FactNodes. If nil, a fresh
	// graph.InMemoryGraphStore is used.
	Store graph.GraphStore
}

// NewFactConstructionService constructs a FactConstructionService with a
// fresh in-memory graph.GraphStore.
func NewFactConstructionService() *FactConstructionService {
	return &FactConstructionService{Store: graph.NewInMemoryGraphStore()}
}

// SegmentInput bundles a single evidence.Classification with the
// originating segment content BuildFactNode needs but
// evidence.Classification does not itself carry (see build.go).
type SegmentInput struct {
	// Classification is the evidentiary classification for this segment.
	Classification evidence.Classification

	// Text is the originating segment's normalized text.
	Text string

	// Span locates the originating segment in the source document.
	Span SourceSpan
}

// ConstructRequest carries the input to
// FactConstructionService.ConstructFacts.
type ConstructRequest struct {
	// CaseID identifies the case the constructed irac.FactNodes belong
	// to. Required.
	CaseID string

	// Segments is the batch of classified segments to convert into fact
	// nodes. Required (non-empty).
	Segments []SegmentInput

	// Parties optionally supplies the case's timeline.Party records for
	// party attribution (see party_attribution.go). May be nil/empty.
	Parties []timeline.Party

	// Events optionally supplies the case's timeline.Event records for
	// temporal anchoring (see temporal.go). May be nil/empty.
	Events []timeline.Event

	// ApplicationIDs optionally lists the irac.ApplicationNode.ID values
	// already persisted for this case, so PersistFacts can create
	// Fact--supports-->Application edges where the fact's subject
	// matter overlaps an application's text. May be nil/empty, in which
	// case no edges are created.
	ApplicationIDs []string

	// Applications optionally supplies the text of each
	// irac.ApplicationNode in ApplicationIDs (keyed by ID), used to
	// decide which facts support which application via token-overlap
	// (see minSupportsOverlap). An ApplicationID with no entry here is
	// still eligible for edges, but only via an exact key match against
	// SupportsApplicationIDs.
	Applications map[string]string

	// IDPrefix prefixes every generated irac.FactNode ID (e.g.
	// "case-42"). If empty, "fact" is used.
	IDPrefix string

	// CreatedAt stamps every persisted irac.FactNode's CreatedAt and
	// Provenance.GeneratedAt. If zero, time.Now() is used.
	CreatedAt time.Time
}

// minSupportsOverlap is the minimum token-overlap ratio between a fact's
// text and an application's text required to create a
// Fact--supports-->Application edge for that pair.
const minSupportsOverlap = 0.2

// FactDetail bundles a single persisted irac.FactNode with every signal
// the construction pipeline derived for it: its evidence backing, party
// attribution, dispute status, temporal anchor, corroboration count, and
// reliability score. ConstructFactsDetailed returns these alongside the
// plain []irac.FactNode ConstructFacts returns, so callers that need the
// full picture do not have to recompute it from scratch.
type FactDetail struct {
	Node               irac.FactNode
	EvidenceRef        EvidenceRef
	PartyAttribution   PartyAttribution
	DisputeStatus      DisputeStatus
	TemporalAnchor     TemporalAnchor
	CorroborationCount int
	ReliabilityScore   float64
}

// ConstructFacts runs the full pipeline over req and returns the
// resulting irac.FactNodes, persisted via s.Store. It is a thin
// convenience wrapper over ConstructFactsDetailed for callers that only
// need the persisted nodes.
//
// Returns ErrEmptyInput if req.CaseID is blank, ErrClassificationInvalid
// if req.Segments is empty or every segment fails BuildFactNode, or
// ErrPersistFailed (wrapping the underlying store error) if persistence
// fails partway through.
func (s *FactConstructionService) ConstructFacts(ctx context.Context, req ConstructRequest) ([]irac.FactNode, error) {
	details, err := s.ConstructFactsDetailed(ctx, req)
	nodes := make([]irac.FactNode, len(details))
	for i, d := range details {
		nodes[i] = d.Node
	}
	if err != nil {
		return nodes, err
	}
	return nodes, nil
}

// ConstructFactsDetailed runs the full fact-construction pipeline over
// req:
//
//	build -> attach evidence ref -> attribute party -> flag dispute
//	  -> anchor temporally -> link corroboration -> score reliability
//	  -> persist
//
// returning one FactDetail per successfully built fact, in input order
// among the successfully persisted facts. See ConstructFacts's doc
// comment for the error conditions.
func (s *FactConstructionService) ConstructFactsDetailed(ctx context.Context, req ConstructRequest) ([]FactDetail, error) {
	if strings.TrimSpace(req.CaseID) == "" {
		return nil, ErrEmptyInput
	}
	if len(req.Segments) == 0 {
		return nil, ErrClassificationInvalid
	}

	store := s.Store
	if store == nil {
		store = graph.NewInMemoryGraphStore()
	}
	prefix := req.IDPrefix
	if prefix == "" {
		prefix = "fact"
	}
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	// 1. build, 2. attach evidence ref
	var nodes []irac.FactNode
	var refs []EvidenceRef
	var segmentIDs []string
	for i, seg := range req.Segments {
		id := fmt.Sprintf("%s-%d", prefix, i)
		node, err := BuildFactNode(seg.Classification, seg.Text, seg.Span, id, req.CaseID, createdAt)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
		segmentIDs = append(segmentIDs, seg.Classification.SegmentID)

		ref, err := NewEvidenceRef(node.ID, seg.Classification, seg.Classification.SegmentID)
		if err != nil {
			ref = EvidenceRef{FactID: node.ID}
		}
		refs = append(refs, ref)
	}
	if len(nodes) == 0 {
		return nil, ErrClassificationInvalid
	}

	// 3. attribute party
	attributions := make([]PartyAttribution, len(nodes))
	for i, ref := range refs {
		attributions[i] = AttributeParty(nodes[i].ID, ref.PartyRole, req.Parties)
	}

	// 4. flag dispute
	peers := make([]FactWithParty, len(nodes))
	for i, n := range nodes {
		peers[i] = FactWithParty{ID: n.ID, Text: n.Text, PartyID: attributions[i].PartyID}
	}
	disputeStatuses := make(map[string]DisputeStatus, len(nodes))
	for _, p := range peers {
		status, _ := DetermineDisputeStatus(p, peers)
		disputeStatuses[p.ID] = status
	}

	// 5. anchor temporally
	anchors := make(map[string]TemporalAnchor, len(nodes))
	for i, n := range nodes {
		anchors[n.ID] = AnchorToEvent(n.ID, n.Text, segmentIDs[i], req.Events)
	}

	// 6. link corroboration
	candidates := make([]CorroborationCandidate, len(nodes))
	for i, n := range nodes {
		candidates[i] = CorroborationCandidate{ID: n.ID, Text: n.Text, PartyID: attributions[i].PartyID}
	}
	corroborationLinks := DetectCorroboration(candidates)

	// 7. score reliability — a distinct signal from the node's raw
	// extraction Confidence (see reliability.go's doc comment), so it is
	// carried on FactDetail rather than mutating Node.Confidence.
	reliabilityScores := make(map[string]float64, len(nodes))
	corroborationCounts := make(map[string]int, len(nodes))
	for _, n := range nodes {
		count := CorroborationCount(n.ID, corroborationLinks)
		corroborationCounts[n.ID] = count
	}
	for i, n := range nodes {
		reliabilityScores[n.ID] = ReliabilityScore(ReliabilityInput{
			ClassificationConfidence: refs[i].Confidence,
			CorroborationCount:       corroborationCounts[n.ID],
			DisputeStatus:            disputeStatuses[n.ID],
		})
	}

	// 8. persist
	supports := supportsApplicationIDs(nodes, req.Applications, req.ApplicationIDs)
	persisted, err := PersistFacts(ctx, store, nodes, req.ApplicationIDs, supports)

	details := make([]FactDetail, len(persisted))
	for i, n := range persisted {
		details[i] = FactDetail{
			Node:               n,
			EvidenceRef:        refs[i],
			PartyAttribution:   attributions[i],
			DisputeStatus:      disputeStatuses[n.ID],
			TemporalAnchor:     anchors[n.ID],
			CorroborationCount: corroborationCounts[n.ID],
			ReliabilityScore:   reliabilityScores[n.ID],
		}
	}
	if err != nil {
		return details, err
	}
	return details, nil
}

// supportsApplicationIDs derives, for each fact node, which application
// IDs it should be linked to via a Fact--supports-->Application edge:
// every application whose text overlaps the fact's text at or above
// minSupportsOverlap.
func supportsApplicationIDs(nodes []irac.FactNode, applications map[string]string, applicationIDs []string) map[string][]string {
	if len(applications) == 0 || len(applicationIDs) == 0 {
		return nil
	}
	out := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		for _, appID := range applicationIDs {
			appText, ok := applications[appID]
			if !ok {
				continue
			}
			if temporalTokenOverlap(appText, n.Text) >= minSupportsOverlap {
				out[n.ID] = append(out[n.ID], appID)
			}
		}
	}
	return out
}
