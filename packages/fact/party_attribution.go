package fact

import (
	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/timeline"
)

// PartyAttribution attaches a case party (PartyID/PartyRole) to a fact
// node: who the fact is attributed to, and which side of the case that
// party is on. This bridges packages/evidence's per-segment
// PartyRole attribution (which identifies a side but not a specific
// timeline.Party record) to packages/timeline's case-level Party roster
// (which has real party IDs and names) via role matching.
type PartyAttribution struct {
	// FactID is the irac.FactNode.ID this attribution belongs to.
	FactID string

	// PartyID is the timeline.Party.ID attributed to the fact, when a
	// matching party was found in the case roster. Empty when no match
	// was found.
	PartyID string

	// PartyRole is the case-side role attributed to the fact (first,
	// second, or third party — mirroring timeline.PartyRole), carried
	// even when PartyID is empty (e.g. a side was identified by the
	// evidence classifier but no timeline.Party record for that side was
	// supplied).
	PartyRole timeline.PartyRole
}

// evidenceToTimelineRole maps evidence.PartyRole values to their
// timeline.PartyRole equivalent. evidence.PartyUnattributed has no
// timeline.PartyRole equivalent (timeline.PartyRole has no "unattributed"
// constant), so it is intentionally absent from this table — see
// AttributeParty's handling of the zero-value case below.
var evidenceToTimelineRole = map[evidence.PartyRole]timeline.PartyRole{
	evidence.PartyFirst:  timeline.PartyFirst,
	evidence.PartySecond: timeline.PartySecond,
}

// AttributeParty determines the PartyAttribution for a fact whose
// originating evidence.Classification carries partyRole, resolving the
// specific timeline.Party (if any) from parties whose Role matches.
//
// When multiple parties share the same Role, the first match (in parties'
// slice order) is used, since neither evidence.PartyRole nor
// timeline.PartyRole carries enough information to disambiguate further
// at this stage.
//
// Returns a PartyAttribution with an empty PartyID and empty PartyRole
// when partyRole is evidence.PartyUnattributed or does not map to a known
// timeline.PartyRole, and/or when no party in parties has a matching
// Role.
func AttributeParty(factID string, partyRole evidence.PartyRole, parties []timeline.Party) PartyAttribution {
	attribution := PartyAttribution{FactID: factID}

	role, ok := evidenceToTimelineRole[partyRole]
	if !ok {
		return attribution
	}
	attribution.PartyRole = role

	for _, p := range parties {
		if p.Role == role {
			attribution.PartyID = p.ID
			break
		}
	}
	return attribution
}
