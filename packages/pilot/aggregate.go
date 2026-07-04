package pilot

import (
	"context"

	"github.com/google/uuid"
)

// QualityScoreLike is the minimal read-only view of a
// packages/reasoningeval.QualityScore this package depends on, mirroring
// packages/reasoningeval.OpinionLike/GroundingReportLike's own
// decoupling rationale: this package's exported surface does not force
// every caller into reasoningeval's dependency graph (reasoningeval's
// go.mod pulls in packages/grounding, packages/synthesisagent,
// packages/treeassembly, packages/vectorindex, the neo4j driver, and
// more -- see doc/pilot.md for the full "why not import reasoningeval"
// rationale). A caller holding a real reasoningeval.QualityScore
// satisfies this interface via the adapter in
// ReasoningEvalQualityScoreAdapter below.
type QualityScoreLike interface {
	// PilotCaseIDValue returns the PilotCase ID this automated score
	// corresponds to, so AggregateQuality can join it against the
	// FeedbackEntry records collected for the same case.
	PilotCaseIDValue() uuid.UUID
	// OverallValue returns the score's overall [0,1] aggregate.
	OverallValue() float64
}

// ReasoningEvalQualityScoreAdapter adapts a caller-held
// packages/reasoningeval.QualityScore (or any equivalent value) to
// QualityScoreLike without this package importing reasoningeval's
// concrete type. A caller that has already resolved a QualityScore's
// CaseID to a PilotCase.ID (this package's FeedbackEntry/PilotFinding
// key on PilotCaseID, not the bare packages/caselifecycle.Case ID
// string reasoningeval.QualityScore.CaseID carries) constructs one of
// these per score before calling AggregateQuality.
type ReasoningEvalQualityScoreAdapter struct {
	// PilotCaseID is the PilotCase this automated score corresponds to.
	PilotCaseID uuid.UUID
	// Overall is the QualityScore's overall [0,1] aggregate (copied from
	// reasoningeval.QualityScore.Overall).
	Overall float64
}

// PilotCaseIDValue implements QualityScoreLike.
func (a ReasoningEvalQualityScoreAdapter) PilotCaseIDValue() uuid.UUID { return a.PilotCaseID }

// OverallValue implements QualityScoreLike.
func (a ReasoningEvalQualityScoreAdapter) OverallValue() float64 { return a.Overall }

var _ QualityScoreLike = ReasoningEvalQualityScoreAdapter{}

// QualitySummary is the real aggregation outcome of AggregateQuality
// (task 5): reasoning quality and trust measured over every
// FeedbackEntry collected for a PilotDeployment, combined with
// whatever automated QualityScoreLike values the caller supplies --
// exactly mirroring how packages/reasoningeval.ExpertReview and
// QualityScore are "stored side by side, never merged"
// (see doc/pilot.md).
type QualitySummary struct {
	// DeploymentID is the PilotDeployment this summary covers.
	DeploymentID uuid.UUID `json:"deployment_id"`

	// FeedbackCount is the number of FeedbackEntry records aggregated.
	FeedbackCount int `json:"feedback_count"`

	// AvgOverallFeedbackScore is the arithmetic mean of
	// FeedbackEntry.OverallScore() across every aggregated entry.
	AvgOverallFeedbackScore float64 `json:"avg_overall_feedback_score"`

	// AvgPerDimension maps each DimensionName to the arithmetic mean of
	// its rating across every FeedbackEntry that rated it.
	AvgPerDimension map[DimensionName]float64 `json:"avg_per_dimension"`

	// AvgTrust is the arithmetic mean of FeedbackEntry.Trust across
	// every aggregated entry, expressed on the same 1-5 TrustRating
	// scale (as a float, since an average of discrete ratings is not
	// itself necessarily an integer).
	AvgTrust float64 `json:"avg_trust"`

	// TrustDistribution counts how many FeedbackEntry records recorded
	// each TrustRating value, keyed by rating.
	TrustDistribution map[TrustRating]int `json:"trust_distribution"`

	// AutomatedScoreCount is the number of QualityScoreLike values
	// folded in from the caller-supplied automatedScores parameter.
	AutomatedScoreCount int `json:"automated_score_count"`

	// AvgAutomatedOverall is the arithmetic mean of
	// QualityScoreLike.OverallValue() across every supplied automated
	// score, or 0 if none were supplied.
	AvgAutomatedOverall float64 `json:"avg_automated_overall"`
}

// AggregateQuality measures reasoning quality and trust for
// deploymentID (task 5): a real aggregation over every FeedbackEntry
// collected across the deployment's PilotCases, combined with the
// automated packages/reasoningeval.QualityScore-shaped values the
// caller supplies via automatedScores (see QualityScoreLike's doc
// comment for why this package accepts scores as parameters rather
// than importing reasoningeval directly). automatedScores may be nil
// or empty -- a quality summary is still meaningful from collected
// expert feedback alone. Requires viewPermission and tenant match.
func (e *Engine) AggregateQuality(ctx context.Context, tenantID, deploymentID uuid.UUID, automatedScores []QualityScoreLike) (QualitySummary, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return QualitySummary{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return QualitySummary{}, err
	}

	entries, err := e.ListFeedbackForDeployment(ctx, tenantID, deploymentID)
	if err != nil {
		return QualitySummary{}, wrapf("AggregateQuality", err)
	}

	return BuildQualitySummary(deploymentID, entries, automatedScores), nil
}

// BuildQualitySummary is AggregateQuality's pure aggregation function,
// mirroring how packages/vulnmanagement.BuildReport and
// packages/compliance.BuildDashboard operate on already-fetched data
// rather than re-querying storage themselves -- exposed separately so
// a caller with its own already-fetched FeedbackEntry slice (e.g. a
// batch job) need not go through the Engine at all.
func BuildQualitySummary(deploymentID uuid.UUID, entries []FeedbackEntry, automatedScores []QualityScoreLike) QualitySummary {
	summary := QualitySummary{
		DeploymentID:      deploymentID,
		FeedbackCount:     len(entries),
		AvgPerDimension:   make(map[DimensionName]float64),
		TrustDistribution: map[TrustRating]int{TrustVeryLow: 0, TrustLow: 0, TrustModerate: 0, TrustHigh: 0, TrustVeryHigh: 0},
	}

	if len(entries) > 0 {
		overallSum := 0.0
		trustSum := 0
		dimensionSums := make(map[DimensionName]float64)
		dimensionCounts := make(map[DimensionName]int)

		for _, entry := range entries {
			overallSum += entry.OverallScore()
			trustSum += int(entry.Trust)
			summary.TrustDistribution[entry.Trust]++
			for _, r := range entry.Ratings {
				dimensionSums[r.Dimension] += r.Score
				dimensionCounts[r.Dimension]++
			}
		}

		summary.AvgOverallFeedbackScore = overallSum / float64(len(entries))
		summary.AvgTrust = float64(trustSum) / float64(len(entries))
		for dim, sum := range dimensionSums {
			summary.AvgPerDimension[dim] = sum / float64(dimensionCounts[dim])
		}
	}

	if len(automatedScores) > 0 {
		sum := 0.0
		for _, s := range automatedScores {
			sum += s.OverallValue()
		}
		summary.AutomatedScoreCount = len(automatedScores)
		summary.AvgAutomatedOverall = sum / float64(len(automatedScores))
	}

	return summary
}
