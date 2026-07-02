package timeline

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// TimelineService orchestrates the full party/timeline pipeline for a
// case:
//
//	extract events from segments -> attribute party facts
//	  -> assemble timeline -> detect conflicts -> link claims
//	  -> persist -> return assembled Timeline
//
// This mirrors packages/evidence's EvidenceService and
// packages/category's CategoryService orchestration pattern: a single
// entry point wiring together this package's otherwise independent,
// individually testable building blocks (ExtractEvent, NewPartyFact,
// AssembleTimeline, DetectConflicts, TimelineStore).
type TimelineService struct {
	// Store persists the resulting CaseGraph. If nil,
	// NewInMemoryTimelineStore() is used.
	Store TimelineStore
}

// NewTimelineService constructs a TimelineService with sensible defaults
// for every pluggable dependency left nil: InMemoryTimelineStore.
func NewTimelineService() *TimelineService {
	return &TimelineService{Store: NewInMemoryTimelineStore()}
}

// SegmentAttribution pairs a segmentation.Segment with the Party.ID it is
// attributed to (empty when unattributed), the input shape
// BuildTimeline's caller supplies for each segment to be woven into the
// case's events and facts.
type SegmentAttribution struct {
	// Segment is the source segment.
	Segment segmentation.Segment

	// PartyID is the Party.ID this segment is attributed to. Empty means
	// no party attribution is available for this segment; the segment
	// still contributes an Event, just with an empty Event.PartyID and no
	// PartyFact.
	PartyID string

	// Subject is an optional normalized subject token for the resulting
	// PartyFact (see PartyFact.Subject), used by conflict detection to
	// match facts about the same subject from different parties. Empty
	// means no subject was determined for this segment.
	Subject string
}

// BuildRequest carries the input to TimelineService.BuildTimeline.
type BuildRequest struct {
	// CaseID identifies the case being assembled.
	CaseID string

	// Parties is the full set of parties known for the case.
	Parties []Party

	// Segments is the batch of segment attributions to extract events and
	// facts from, in any order (AssembleTimeline re-orders them).
	Segments []SegmentAttribution

	// Claims is the set of party-claim linkages to validate and persist
	// alongside the assembled timeline. Each Claim's EventIDs/FactIDs
	// should reference IDs generated deterministically as
	// "<CaseID>-event-<n>" / "<CaseID>-fact-<n>" per Segments' input
	// order (see BuildTimeline's doc for the exact scheme), or callers can
	// inspect the returned CaseGraph.Events/Facts to discover the actual
	// generated IDs before constructing Claims in a two-pass workflow.
	Claims []Claim

	// Relationships is the set of party relationships to persist alongside
	// the assembled timeline.
	Relationships []Relationship
}

// BuildResult is the output of TimelineService.BuildTimeline: the
// assembled Timeline plus the full persisted CaseGraph it was derived
// from.
type BuildResult struct {
	Timeline Timeline
	Graph    CaseGraph
}

// BuildTimeline runs the full pipeline for req.CaseID: extract an Event
// (via ExtractEvent) and, for attributed segments, a PartyFact (via
// NewPartyFact) from every entry in req.Segments; assemble the resulting
// events into a Timeline (via AssembleTimeline); detect conflicts among
// the extracted facts (via DetectConflicts, using event dates as the
// same-date gate); validate and attach req.Claims and req.Relationships;
// persist the full CaseGraph via Store; and return the assembled Timeline
// alongside the persisted graph.
//
// Generated Event and PartyFact IDs follow the deterministic scheme
// "<CaseID>-event-<n>" / "<CaseID>-fact-<n>", where n is the segment's
// zero-based index within req.Segments — so repeated calls with the same
// input produce the same IDs.
//
// Returns ErrCaseNotFound-adjacent ErrEmptyInput if req.CaseID is empty,
// ErrInvalidParty if any req.Parties entry fails Party.Validate, and
// ErrInvalidClaim if any req.Claims entry fails Claim.Validate or
// references an Event/PartyFact ID not produced from req.Segments.
func (s *TimelineService) BuildTimeline(ctx context.Context, req BuildRequest) (BuildResult, error) {
	if req.CaseID == "" {
		return BuildResult{}, ErrEmptyInput
	}
	for _, p := range req.Parties {
		if err := p.Validate(); err != nil {
			return BuildResult{}, err
		}
	}

	store := s.Store
	if store == nil {
		store = NewInMemoryTimelineStore()
	}

	events := make([]Event, 0, len(req.Segments))
	facts := make([]PartyFact, 0, len(req.Segments))
	dateForSegment := make(map[string]*time.Time, len(req.Segments))

	for i, sa := range req.Segments {
		eventID := fmt.Sprintf("%s-event-%d", req.CaseID, i)
		ev := ExtractEvent(eventID, sa.Segment, sa.PartyID)
		events = append(events, ev)
		dateForSegment[sa.Segment.ID] = ev.OccurredAt

		if sa.PartyID != "" {
			factID := fmt.Sprintf("%s-fact-%d", req.CaseID, i)
			facts = append(facts, NewPartyFact(factID, sa.PartyID, sa.Segment, sa.Subject))
		}
	}

	tl := AssembleTimeline(req.CaseID, events)
	conflicts := DetectConflicts(facts, EventsSameOrOverlappingDate(dateForSegment))

	knownEventIDs := make(map[string]bool, len(events))
	for _, ev := range events {
		knownEventIDs[ev.ID] = true
	}
	knownFactIDs := make(map[string]bool, len(facts))
	for _, f := range facts {
		knownFactIDs[f.ID] = true
	}
	for _, c := range req.Claims {
		if err := c.Validate(); err != nil {
			return BuildResult{}, err
		}
		if err := ValidateClaimLinkage(c, knownEventIDs, knownFactIDs); err != nil {
			return BuildResult{}, err
		}
	}
	for _, r := range req.Relationships {
		if err := r.Validate(); err != nil {
			return BuildResult{}, err
		}
	}

	graph := CaseGraph{
		CaseID:        req.CaseID,
		Parties:       req.Parties,
		Facts:         facts,
		Events:        tl.Events,
		Claims:        req.Claims,
		Conflicts:     conflicts,
		Relationships: req.Relationships,
	}

	if err := store.SaveGraph(ctx, graph); err != nil {
		return BuildResult{}, err
	}

	return BuildResult{Timeline: tl, Graph: graph}, nil
}
