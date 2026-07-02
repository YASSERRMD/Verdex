package fact

// reliabilityWeights are the fixed blend weights ReliabilityScore applies
// to its three signals (classification confidence, corroboration count,
// dispute status). Chosen so that classification confidence — the
// closest thing to a direct evidentiary signal — dominates, corroboration
// provides a meaningful but secondary boost, and an unresolved dispute
// status neither penalizes nor rewards (see disputeFactor).
const (
	confidenceWeight    = 0.5
	corroborationWeight = 0.3
	disputeWeight       = 0.2
)

// maxCorroborationForScoring caps how many corroboration links count
// toward the corroboration signal, so that reliability score gains from
// corroboration saturate rather than growing unbounded.
const maxCorroborationForScoring = 3

// ReliabilityInput bundles the three signals ReliabilityScore combines:
// the originating classification's confidence, how many independent
// facts corroborate this one, and its dispute status. Kept as a
// dedicated input type (rather than threading a full irac.FactNode plus
// side-tables through) so ReliabilityScore stays a pure function easy to
// unit test in isolation.
type ReliabilityInput struct {
	// ClassificationConfidence is the originating evidence.Classification
	// Confidence, in [0, 1] (see EvidenceRef.Confidence).
	ClassificationConfidence float64

	// CorroborationCount is the number of independent facts corroborating
	// this one (see CorroborationCount).
	CorroborationCount int

	// DisputeStatus is this fact's DisputeStatus (see
	// DetermineDisputeStatus).
	DisputeStatus DisputeStatus
}

// ReliabilityScore combines classification confidence, corroboration
// count, and dispute status into a single reliability signal in the
// closed interval [0, 1], deliberately separate from the node's raw
// irac.Node.Confidence (which reflects only extraction/classification
// confidence, not corroboration or dispute).
//
// The score is monotonic in its inputs: for a fixed confidence and
// dispute status, more corroboration never lowers the score; for a fixed
// confidence and corroboration count, Disputed never scores higher than
// Undisputed or Unknown.
func ReliabilityScore(input ReliabilityInput) float64 {
	confidence := clampUnit(input.ClassificationConfidence)

	corroboration := float64(input.CorroborationCount) / float64(maxCorroborationForScoring)
	if corroboration > 1 {
		corroboration = 1
	}
	if corroboration < 0 {
		corroboration = 0
	}

	dispute := disputeFactor(input.DisputeStatus)

	score := confidenceWeight*confidence + corroborationWeight*corroboration + disputeWeight*dispute
	return clampUnit(score)
}

// disputeFactor maps a DisputeStatus to its contribution to the dispute
// signal: Undisputed contributes full weight (1.0), Unknown contributes
// half weight (neither confirms nor undermines reliability), and Disputed
// contributes none (0.0), since a contested fact is, by definition, less
// reliable on its own than an uncontested one.
func disputeFactor(status DisputeStatus) float64 {
	switch status {
	case Undisputed:
		return 1.0
	case Unknown:
		return 0.5
	case Disputed:
		return 0.0
	default:
		return 0.5
	}
}

// clampUnit clamps v into the closed interval [0, 1].
func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
