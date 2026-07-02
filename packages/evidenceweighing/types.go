package evidenceweighing

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
)

// FactRef is the minimal shape this package needs for a fact node in a
// case's tree: its ID, text (used only for EvidenceKind classification —
// see classify.go), and its irac.Node.Confidence. Kept as a dedicated,
// narrow input type (rather than depending on knowledgeapi.NodeDTO or
// irac.FactNode directly) so Weigh stays easy to unit test with in-memory
// fixtures, mirroring packages/fact's ReliabilityInput convention of a
// small purpose-built struct instead of threading a full node type
// through.
type FactRef struct {
	// ID is the irac.FactNode.ID (knowledgeapi.NodeDTO.ID) this FactRef
	// describes.
	ID string

	// Text is the fact's assertion text, used only to heuristically
	// classify it as EvidenceKindTestimony or EvidenceKindDocumentary
	// (see classify.go). Not required to be non-empty; an empty Text
	// classifies as EvidenceKindUnknown.
	Text string

	// Confidence is the fact's irac.Node.Confidence /
	// knowledgeapi.NodeDTO.Confidence, in [0, 1].
	Confidence float64
}

// CitingArgument is this package's own, party-agnostic view of a single
// Argument from either firstpartyagent.ArgumentSet or
// secondpartyagent.ArgumentSet: just enough fields to drive corroboration
// counting, contradiction detection, and citation-strength scoring,
// without this package depending on which of the two (structurally
// near-identical but independent) Argument types produced it.
type CitingArgument struct {
	// ArgumentID is the originating Argument.ID.
	ArgumentID string

	// IssueNodeID is the originating Argument.IssueNodeID.
	IssueNodeID string

	// PartyID is the originating Argument.PartyID, carried as a plain
	// string so this package need not choose between
	// firstpartyagent.PartyID and secondpartyagent.PartyID (which are
	// independent, structurally identical types).
	PartyID string

	// SupportingFactIDs is the originating Argument.SupportingFactIDs.
	SupportingFactIDs []string

	// Strength is the originating Argument.Strength.
	Strength float64
}

// citingArgumentsFromFirstParty converts every Argument in set into
// CitingArguments.
func citingArgumentsFromFirstParty(set firstpartyagent.ArgumentSet) []CitingArgument {
	out := make([]CitingArgument, 0, len(set.Arguments))
	for _, a := range set.Arguments {
		out = append(out, CitingArgument{
			ArgumentID:        a.ID,
			IssueNodeID:       a.IssueNodeID,
			PartyID:           string(a.PartyID),
			SupportingFactIDs: a.SupportingFactIDs,
			Strength:          a.Strength,
		})
	}
	return out
}

// citingArgumentsFromSecondParty converts every Argument in set into
// CitingArguments.
func citingArgumentsFromSecondParty(set secondpartyagent.ArgumentSet) []CitingArgument {
	out := make([]CitingArgument, 0, len(set.Arguments))
	for _, a := range set.Arguments {
		out = append(out, CitingArgument{
			ArgumentID:        a.ID,
			IssueNodeID:       a.IssueNodeID,
			PartyID:           string(a.PartyID),
			SupportingFactIDs: a.SupportingFactIDs,
			Strength:          a.Strength,
		})
	}
	return out
}

// Contradiction records that a single fact was cited by both parties'
// arguments in support of mutually exclusive claims for the same issue:
// same IssueNodeID, same FactNodeID, but opposing PartyID values.
type Contradiction struct {
	// FactNodeID is the fact cited by both sides.
	FactNodeID string

	// IssueNodeID is the shared issue both arguments address.
	IssueNodeID string

	// ArgumentAID and ArgumentBID are the two opposing Argument.ID values
	// citing FactNodeID. ArgumentAID is always the argument encountered
	// first among the inputs supplied to Weigh, for deterministic,
	// reproducible output.
	ArgumentAID string
	ArgumentBID string

	// PartyAID and PartyBID are the two arguments' respective PartyID
	// values.
	PartyAID string
	PartyBID string
}

// Gap surfaces a defect in the evidentiary record this package can detect
// defensively at the reasoning stage: either an Argument citing a
// SupportingFactID that does not resolve to any FactRef supplied to
// Weigh, or an issue for which no Argument from either party cited any
// fact at all.
type Gap struct {
	// Kind identifies which kind of gap this is.
	Kind GapKind

	// IssueNodeID is the issue this gap relates to. Always populated for
	// GapKindUncitedIssue; populated for GapKindMissingFact whenever the
	// offending Argument's IssueNodeID is known.
	IssueNodeID string

	// ArgumentID is the Argument that cited a missing fact. Empty for
	// GapKindUncitedIssue, since no argument is at fault there.
	ArgumentID string

	// FactNodeID is the fact ID that could not be resolved. Empty for
	// GapKindUncitedIssue.
	FactNodeID string

	// Description is a human-readable explanation of the gap.
	Description string
}

// GapKind classifies a Gap.
type GapKind string

const (
	// GapKindMissingFact is a Gap where an Argument's SupportingFactIDs
	// references a fact that does not exist among the FactRefs supplied
	// to Weigh. Each agent's own grounding step (see firstpartyagent/
	// ground.go, secondpartyagent/ground.go) should already prevent
	// this, but this package treats that defensively rather than
	// trusting it, per the plan's explicit "surface gaps" requirement.
	GapKindMissingFact GapKind = "missing_fact"

	// GapKindUncitedIssue is a Gap where neither party's ArgumentSet
	// contains any Argument, for the given IssueNodeID, with at least
	// one SupportingFactID — the issue is being argued on no evidence at
	// all.
	GapKindUncitedIssue GapKind = "uncited_issue"
)

// FactWeight is the per-fact output of Weigh: a single fact's final
// weight, whether it is Contradicted, how many arguments corroborate it,
// and a human-readable Rationale explaining the score's derivation.
type FactWeight struct {
	// FactNodeID is the fact this weight describes.
	FactNodeID string `json:"fact_node_id"`

	// Weight is the fact's final [0, 1] evidentiary weight, after
	// blending Confidence/corroboration/citation-strength, applying the
	// contradiction penalty, and applying the jurisdiction profile
	// multiplier.
	Weight float64 `json:"weight"`

	// Kind is this fact's classified EvidenceKind (see classify.go),
	// recorded so a caller can see which jurisdiction multiplier applied
	// without recomputing it.
	Kind EvidenceKind `json:"kind"`

	// Contradicted is true if this fact appears in at least one
	// Contradiction.
	Contradicted bool `json:"contradicted"`

	// CorroborationCount is the number of distinct arguments (across
	// both parties) that cite this fact in their SupportingFactIDs.
	CorroborationCount int `json:"corroboration_count"`

	// Rationale is a human-readable explanation of how Weight was
	// derived: which signals contributed, and any contradiction/
	// jurisdiction adjustment applied.
	Rationale string `json:"rationale"`
}

// EvidenceWeighingResult is the full output of one Weigh call for a case:
// every FactWeight computed, every Contradiction detected, and every Gap
// surfaced.
type EvidenceWeighingResult struct {
	// CaseID is the case this result was computed for.
	CaseID string `json:"case_id"`

	// FactWeights is one FactWeight per fact referenced by either
	// party's ArgumentSet (and present among the FactRefs supplied to
	// Weigh), in no particular order.
	FactWeights []FactWeight `json:"fact_weights"`

	// Contradictions is every Contradiction detected across the two
	// ArgumentSets.
	Contradictions []Contradiction `json:"contradictions"`

	// Gaps is every Gap surfaced in the evidentiary record.
	Gaps []Gap `json:"gaps"`

	// LegalFamily is the LegalFamily whose JurisdictionProfile was
	// applied to produce FactWeights.
	LegalFamily LegalFamily `json:"legal_family"`

	// GeneratedAt records when this result was computed.
	GeneratedAt time.Time `json:"generated_at"`
}
