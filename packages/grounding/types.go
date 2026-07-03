package grounding

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/citation"
)

// ClaimKind classifies what kind of assertion a Claim represents, which
// in turn determines how Check verifies it.
type ClaimKind string

const (
	// ClaimReference is a claim that a TentativeConclusion's text relies
	// on a specific supporting fact or rule node
	// (synthesisagent.TentativeConclusion.SupportingFactIDs/
	// SupportingRuleIDs). Verified by cross-checking the referenced node
	// ID actually exists in the case's tree (reference.go).
	ClaimReference ClaimKind = "reference"

	// ClaimCitation is a claim that a controlling rule carries a specific,
	// existing citation. Never produced by ExtractClaims (extract.go) —
	// citation verification runs directly over a conclusion's
	// SupportingRuleIDs against a graph.GraphStore via packages/citation
	// (citations.go), independent of the pure Claim-extraction pipeline
	// the other three kinds go through. Retained as a ClaimKind value for
	// completeness/future use (e.g. a future extractor that pulls
	// inline citation text out of conclusion prose itself).
	ClaimCitation ClaimKind = "citation"

	// ClaimNumeric is a numeric figure mentioned in a conclusion's prose
	// (an amount, a count, a percentage). Verified by cross-checking the
	// figure appears in at least one of the conclusion's supporting fact
	// nodes (consistency.go).
	ClaimNumeric ClaimKind = "numeric"

	// ClaimDate is a calendar date mentioned in a conclusion's prose.
	// Verified the same way as ClaimNumeric (consistency.go).
	ClaimDate ClaimKind = "date"
)

// allClaimKinds is the exhaustive set of recognized ClaimKind values.
var allClaimKinds = map[ClaimKind]struct{}{
	ClaimReference: {},
	ClaimCitation:  {},
	ClaimNumeric:   {},
	ClaimDate:      {},
}

// IsValid reports whether k is one of the recognized ClaimKind constants.
func (k ClaimKind) IsValid() bool {
	_, ok := allClaimKinds[k]
	return ok
}

// Claim is a single, atomic assertion extracted from a
// synthesisagent.Opinion, pending verification against the case's tree.
// Extract (extract.go) is the only producer of Claim values; every field
// here is populated before a Claim is ever handed to a verify* function.
type Claim struct {
	// IssueNodeID is the irac.IssueNode.ID of the TentativeConclusion this
	// claim was extracted from (TentativeConclusion.IssueNodeID).
	IssueNodeID string

	// Kind classifies what this claim asserts and how it must be
	// verified.
	Kind ClaimKind

	// Value is the claim's content: a node ID for ClaimReference/
	// ClaimCitation, the literal numeral text for ClaimNumeric, or the
	// literal date text for ClaimDate.
	Value string

	// SourceText is the surrounding conclusion prose this claim was
	// extracted from, kept for a human reviewer to see the claim in
	// context rather than as a bare value.
	SourceText string
}

// VerificationOutcome classifies the result of verifying a single Claim.
type VerificationOutcome string

const (
	// OutcomeGrounded means the claim was independently confirmed against
	// the case's tree or corpus.
	OutcomeGrounded VerificationOutcome = "grounded"

	// OutcomeUngrounded means the claim could not be confirmed: a
	// referenced node does not exist, a cited authority does not resolve,
	// or a numeric/date figure appears nowhere in the claim's supporting
	// facts.
	OutcomeUngrounded VerificationOutcome = "ungrounded"

	// OutcomeUnverifiable means this package had insufficient information
	// to check the claim at all (e.g. a numeric claim on a conclusion with
	// no supporting facts to check against). Distinct from
	// OutcomeUngrounded: this is a coverage gap, not a confirmed
	// hallucination.
	OutcomeUnverifiable VerificationOutcome = "unverifiable"
)

// allVerificationOutcomes is the exhaustive set of recognized
// VerificationOutcome values.
var allVerificationOutcomes = map[VerificationOutcome]struct{}{
	OutcomeGrounded:     {},
	OutcomeUngrounded:   {},
	OutcomeUnverifiable: {},
}

// IsValid reports whether o is one of the recognized VerificationOutcome
// constants.
func (o VerificationOutcome) IsValid() bool {
	_, ok := allVerificationOutcomes[o]
	return ok
}

// Severity classifies how serious a Finding is. This mirrors
// packages/treevalidation and packages/citation's Severity convention
// exactly, redeclared locally to keep this package free of a
// cross-package dependency on treevalidation for a three-value enum (the
// same reasoning packages/citation/finding.go documents for its own
// Severity).
type Severity string

const (
	// SeverityCritical marks a Finding that must block finalization (see
	// gate.go's CanFinalize): a fabricated node reference, a hallucinated
	// or wrong-case citation, or a numeric/date figure directly
	// contradicted by the case's own facts.
	SeverityCritical Severity = "critical"

	// SeverityWarning marks a Finding worth surfacing to a reviewer but
	// that does not, on its own, block finalization: an unverifiable
	// claim, or a citation finding that packages/citation itself only
	// rates as a warning (e.g. a stale citation).
	SeverityWarning Severity = "warning"

	// SeverityInfo marks a purely informational Finding.
	SeverityInfo Severity = "info"
)

// Finding codes: short, stable, machine-readable identifiers for the kind
// of grounding problem a Finding represents.
const (
	// CodeFabricatedReference flags a Claim of ClaimReference whose
	// Value node ID does not exist anywhere in the case's tree.
	CodeFabricatedReference = "grounding_fabricated_reference"

	// CodeCitationHallucinated flags a Claim of ClaimCitation whose
	// underlying node was verified by packages/citation as hallucinated
	// or belonging to a different case.
	CodeCitationHallucinated = "grounding_citation_hallucinated"

	// CodeCitationUnresolved flags a Claim of ClaimCitation whose
	// underlying node carries no resolved citation text at all.
	CodeCitationUnresolved = "grounding_citation_unresolved"

	// CodeNumericMismatch flags a Claim of ClaimNumeric whose figure does
	// not appear in any of the conclusion's supporting fact nodes.
	CodeNumericMismatch = "grounding_numeric_mismatch"

	// CodeDateMismatch flags a Claim of ClaimDate whose date does not
	// appear in any of the conclusion's supporting fact nodes.
	CodeDateMismatch = "grounding_date_mismatch"

	// CodeUnverifiableClaim flags a Claim this package had no supporting
	// facts to check against at all.
	CodeUnverifiableClaim = "grounding_unverifiable_claim"
)

// Finding is a single structured grounding problem (or informational
// note) surfaced while checking one Claim, mirroring
// packages/treevalidation and packages/citation's Finding/Report/Severity
// convention.
type Finding struct {
	// Severity classifies how serious this Finding is.
	Severity Severity

	// Code is a short, stable machine-readable identifier for the kind of
	// problem this Finding represents (see the Code* constants above).
	Code string

	// Message is a human-readable description of this specific
	// occurrence.
	Message string

	// IssueNodeID is the TentativeConclusion.IssueNodeID this Finding
	// concerns.
	IssueNodeID string

	// Claim is the Claim this Finding was raised against.
	Claim Claim
}

// ConclusionResult is the grounding outcome for a single
// synthesisagent.TentativeConclusion: every Claim extracted from it, each
// Claim's verification outcome, any Findings raised, and a per-conclusion
// confidence score.
type ConclusionResult struct {
	// IssueNodeID is the TentativeConclusion.IssueNodeID this result
	// concerns.
	IssueNodeID string

	// Claims is every Claim extracted from this conclusion, in extraction
	// order.
	Claims []Claim

	// Outcomes maps each Claims index to its VerificationOutcome, in the
	// same order as Claims.
	Outcomes []VerificationOutcome

	// Findings is every Finding raised while verifying this conclusion's
	// Claims.
	Findings []Finding

	// CitationFindings is every packages/citation Finding raised while
	// verifying this conclusion's controlling rule citations, kept
	// alongside (not merged into) Findings so a caller can distinguish
	// this package's own grounding findings from citation's, while
	// GroundingScore below still folds both into one number.
	CitationFindings []citation.Finding

	// ConfidenceScore is this conclusion's own [0, 1] grounding
	// confidence — see confidence.go for exactly how it is computed.
	ConfidenceScore float64
}

// Report is the structured, exportable summary of a full grounding check
// run over one synthesisagent.Opinion: a ConclusionResult per conclusion,
// every Finding flattened into a single list for a caller that just wants
// "every problem in this opinion", and an overall opinion-level confidence
// score.
type Report struct {
	// CaseID identifies the case whose opinion this Report was computed
	// for.
	CaseID string

	// Conclusions is one ConclusionResult per
	// synthesisagent.Opinion.Conclusions entry, in the same order.
	Conclusions []ConclusionResult

	// Findings is every Finding across every ConclusionResult, in the
	// order the conclusions were checked. Does not include
	// ConclusionResult.CitationFindings — see AllCitationFindings for
	// those, flattened the same way.
	Findings []Finding

	// OpinionScore is the overall [0, 1] grounding confidence for the
	// whole opinion — see confidence.go for exactly how it is computed
	// from the per-conclusion ConfidenceScore values.
	OpinionScore float64

	// GeneratedAt records when this Report was computed.
	GeneratedAt time.Time
}
