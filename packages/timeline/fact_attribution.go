package timeline

import (
	"strings"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// PartyFact links a Party to a factual assertion found in a single
// segmentation.Segment: what the party is recorded as having said or
// claimed, traceable back to its exact source location via Span.
//
// This mirrors packages/evidence's per-segment party attribution
// (AttributeParty), but at the granularity this package needs: a
// standalone record tying a specific Party.ID to a specific claim of fact,
// suitable for later conflict detection (see conflict.go) and claim
// linkage (see claim.go).
type PartyFact struct {
	// ID uniquely identifies this fact attribution within its case.
	ID string

	// PartyID is the Party.ID this fact is attributed to.
	PartyID string

	// SegmentID identifies the segmentation.Segment this fact was
	// extracted from.
	SegmentID string

	// Text is the normalized text of the factual assertion, typically the
	// segment's text (or a relevant excerpt of it).
	Text string

	// Span locates Text within the original source document, mirroring
	// segmentation.SourceSpan so a fact can be traced back to its exact
	// position in the original transcript or filing.
	Span segmentation.SourceSpan

	// Subject is a short, normalized token identifying what the fact is
	// about (e.g. "payment", "notice", "possession"), used by conflict
	// detection to find facts from different parties addressing the same
	// subject. Empty means no subject was determined.
	Subject string
}

// Validate checks that f has a non-empty ID, PartyID, SegmentID, and
// non-empty (non-whitespace) Text. Returns ErrInvalidClaim-adjacent
// ErrEmptyInput if Text is blank, or ErrPartyNotFound-adjacent
// ErrInvalidParty if PartyID is blank.
func (f PartyFact) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return ErrEmptyInput
	}
	if strings.TrimSpace(f.PartyID) == "" {
		return ErrInvalidParty
	}
	if strings.TrimSpace(f.SegmentID) == "" {
		return ErrEmptyInput
	}
	if strings.TrimSpace(f.Text) == "" {
		return ErrEmptyInput
	}
	return nil
}

// NewPartyFact constructs a PartyFact from a segmentation.Segment
// attributed to partyID, carrying the segment's Span and Text forward
// unchanged. subject is an optional caller-supplied normalized subject
// token (see PartyFact.Subject); pass "" when not known.
func NewPartyFact(id, partyID string, seg segmentation.Segment, subject string) PartyFact {
	return PartyFact{
		ID:        id,
		PartyID:   partyID,
		SegmentID: seg.ID,
		Text:      seg.Text,
		Span:      seg.Span,
		Subject:   subject,
	}
}
