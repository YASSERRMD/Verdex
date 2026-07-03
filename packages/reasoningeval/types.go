package reasoningeval

import "time"

// DimensionName identifies one named axis of reasoning quality within a
// Rubric. Kept as a distinct string type (rather than a bare string, and
// rather than an exhaustive closed enum like packages/grounding.ClaimKind)
// because — unlike grounding's fixed claim taxonomy — a Rubric's
// dimensions are meant to be extensible: this package ships three
// built-in dimensions (DimensionGrounding, DimensionCitation,
// DimensionCoherence) but callers may register custom Dimensions for
// additional axes (e.g. a jurisdiction-specific style check) without this
// package's types changing shape.
type DimensionName string

const (
	// DimensionGrounding scores how well an Opinion's assertions are
	// grounded in the case's own facts and law, delegating to
	// packages/grounding.Report.OpinionScore.
	DimensionGrounding DimensionName = "grounding"

	// DimensionCitation scores citation fidelity, delegating to the
	// packages/citation.Finding severities carried on a
	// packages/grounding.Report's ConclusionResult.CitationFindings.
	DimensionCitation DimensionName = "citation"

	// DimensionCoherence scores structural coherence of the Opinion's own
	// text: issue coverage and conclusion completeness. Never contradicts
	// or weakens packages/guardrail's verdict-language ban — see
	// coherence.go.
	DimensionCoherence DimensionName = "coherence"
)

// DimensionScorer scores one Rubric Dimension for a single ScoreInput.
// Implementations must return a value in [0.0, 1.0] where 1.0 is a
// perfect score on that dimension and 0.0 is the worst possible score.
// Mirrors packages/eval.ScorerFn's pure-function convention, but takes a
// structured ScoreInput (an Opinion plus its grounding.Report) rather
// than two bare strings, since reasoning-quality dimensions need
// structured signals, not raw text diffing.
type DimensionScorer func(input ScoreInput) (float64, error)

// Dimension pairs a named quality axis with its relative Weight and the
// DimensionScorer that evaluates it, mirroring
// packages/eval.RubricCriteria's Name/Weight/Fn shape exactly.
type Dimension struct {
	// Name is this dimension's identifier (one of the Dimension* constants,
	// or a caller-defined custom name).
	Name DimensionName

	// Weight is the relative importance of this dimension. Must be > 0;
	// weights across a Rubric do not need to sum to 1 — Score normalises
	// them.
	Weight float64

	// Scorer evaluates this dimension against a ScoreInput.
	Scorer DimensionScorer
}

// Rubric is a structured, weighted set of quality Dimensions applied to a
// single Opinion by Score.
type Rubric struct {
	// Name identifies this rubric (e.g. "default-v1"), so a QualityScore
	// can record which rubric version produced it.
	Name string

	// Dimensions is the ordered list of weighted quality axes. Must
	// contain at least one entry.
	Dimensions []Dimension
}

// ScoreInput bundles everything a DimensionScorer needs to evaluate one
// Opinion: the Opinion itself and the grounding.Report already computed
// for it. This package never calls grounding.Check itself (that requires
// a graph.GraphStore and an authenticated context this package has no
// business holding open) — callers compute the Report once and pass it
// in, so DimensionGrounding and DimensionCitation can fold its results in
// without a second verification pass.
type ScoreInput struct {
	// CaseID identifies the case the Opinion belongs to.
	CaseID string

	// JurisdictionCode is the jurisdiction this case was decided under
	// (e.g. a jurisdiction.Jurisdiction.CountryCode plus court identifier,
	// or any caller-chosen stable key). Used to key per-jurisdiction
	// aggregation in aggregate.go.
	JurisdictionCode string

	// LegalFamily is the reasoningprofile.Family this jurisdiction
	// resolves to, if known. Optional: left empty when the caller has not
	// resolved a family, in which case Aggregate still groups by
	// JurisdictionCode but omits family-level breakdowns for this input.
	LegalFamily string

	// Opinion is the synthesized reasoning output being scored. This
	// package depends only on the fields it actually reads (CaseID,
	// Conclusions) — see score.go and coherence.go.
	Opinion OpinionLike

	// GroundingReport is the already-computed packages/grounding.Report
	// for Opinion. Required by DimensionGrounding and DimensionCitation;
	// a nil-equivalent zero-value Report (OpinionScore 0, no findings) is
	// valid input and simply scores those dimensions at their floor.
	GroundingReport GroundingReportLike
}

// QualityScore is the automated scoring outcome for a single Opinion
// against a Rubric, mirroring packages/eval.EvalResult's per-run-result
// shape.
type QualityScore struct {
	// CaseID identifies the case the scored Opinion belongs to.
	CaseID string

	// JurisdictionCode is copied from ScoreInput.JurisdictionCode.
	JurisdictionCode string

	// LegalFamily is copied from ScoreInput.LegalFamily.
	LegalFamily string

	// RubricName is the Rubric.Name that produced this score.
	RubricName string

	// RunID identifies the evaluation/model/template run this score
	// belongs to (e.g. a deployment version, prompt template version, or
	// batch identifier). Used by RegressionDetector to compare two runs.
	RunID string

	// Overall is the weighted aggregate score across all Dimensions,
	// normalised to [0.0, 1.0].
	Overall float64

	// PerDimension maps each Dimension.Name to its raw (un-weighted)
	// score, mirroring packages/eval.EvalResult.Rubric.
	PerDimension map[DimensionName]float64

	// ScoredAt records when this QualityScore was computed.
	ScoredAt time.Time
}

// OpinionLike is the minimal read-only view of a synthesisagent.Opinion
// this package depends on. Declared as an interface (rather than
// importing synthesisagent.Opinion by value) so this package's exported
// surface does not force every caller into synthesisagent's own
// dependency graph; packages/synthesisagent.Opinion satisfies this
// interface via the adapter in adapter.go.
type OpinionLike interface {
	// OpinionCaseID returns the CaseID this opinion was synthesized for.
	OpinionCaseID() string
	// ConclusionCount returns the number of conclusions in the opinion.
	ConclusionCount() int
	// ConclusionText returns the Text of the conclusion at index i.
	ConclusionText(i int) string
	// ConclusionConfidence returns the Confidence of the conclusion at
	// index i.
	ConclusionConfidence(i int) float64
	// SkippedIssueCount returns the number of issues that were skipped
	// (no valid conclusion reached).
	SkippedIssueCount() int
}

// GroundingReportLike is the minimal read-only view of a
// packages/grounding.Report this package depends on, mirroring
// OpinionLike's decoupling rationale.
type GroundingReportLike interface {
	// OpinionScoreValue returns the report's overall [0,1] grounding
	// confidence.
	OpinionScoreValue() float64
	// CitationFindingCount returns the total number of citation findings
	// across all conclusions.
	CitationFindingCount() int
	// CriticalCitationFindingCount returns the number of citation
	// findings at critical severity.
	CriticalCitationFindingCount() int
}
