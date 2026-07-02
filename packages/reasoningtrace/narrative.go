package reasoningtrace

import (
	"fmt"
	"strings"

	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
)

// buildNarrativeSegments produces one NarrativeSegment per completed
// stage in state.CompletedStages that this package knows how to narrate,
// in pipeline order, each carrying the tree node IDs its prose
// discusses.
func buildNarrativeSegments(state reasoningorchestration.RunState, checkpoints map[reasoningorchestration.Stage]reasoningorchestration.Checkpoint) []NarrativeSegment {
	var segments []NarrativeSegment
	for _, stage := range state.CompletedStages {
		cp, ok := checkpoints[stage]
		if !ok {
			continue
		}
		if seg, ok := narrateStage(stage, cp); ok {
			segments = append(segments, seg)
		}
	}
	return segments
}

// narrateStage builds the NarrativeSegment for one stage's Checkpoint.
// The second return value is false for a stage this package has no
// narration text for (currently none — every runnable stage narrates —
// but kept for forward compatibility with a future stage this package
// has not been taught to describe yet).
func narrateStage(stage reasoningorchestration.Stage, cp reasoningorchestration.Checkpoint) (NarrativeSegment, bool) {
	switch stage {
	case reasoningorchestration.StageIssueFraming:
		var ids []string
		for _, issue := range cp.IssueAnalysis.Issues {
			ids = append(ids, issue.SourceIssueNodeID)
		}
		text := fmt.Sprintf("The issue-framing stage identified %d issue(s) for this case.", len(cp.IssueAnalysis.Issues))
		return NarrativeSegment{Stage: stage, Text: text, RelatedNodeIDs: ids}, true

	case reasoningorchestration.StageFirstPartyArguments:
		text := fmt.Sprintf("The first-party agent constructed %d argument(s) in support of its position.", len(cp.FirstPartyArguments.Arguments))
		return NarrativeSegment{Stage: stage, Text: text, RelatedNodeIDs: firstPartyArgumentIssueIDs(cp)}, true

	case reasoningorchestration.StageSecondPartyArguments:
		text := fmt.Sprintf("The second-party agent responded with %d argument(s), including rebuttal of the first party's position.", len(cp.SecondPartyArguments.Arguments))
		return NarrativeSegment{Stage: stage, Text: text, RelatedNodeIDs: secondPartyArgumentIssueIDs(cp)}, true

	case reasoningorchestration.StageEvidenceWeighing:
		text := fmt.Sprintf("The evidence-weighing stage scored %d fact(s) and found %d contradiction(s).", len(cp.Evidence.FactWeights), len(cp.Evidence.Contradictions))
		return NarrativeSegment{Stage: stage, Text: text}, true

	case reasoningorchestration.StageLawApplication:
		text := fmt.Sprintf("The law-application stage mapped controlling authority to %d issue(s).", len(cp.Law.IssueApplications))
		return NarrativeSegment{Stage: stage, Text: text, RelatedNodeIDs: lawApplicationIssueIDs(cp)}, true

	case reasoningorchestration.StageSynthesis:
		var ids []string
		for _, c := range cp.Opinion.Conclusions {
			ids = append(ids, c.IssueNodeID)
			ids = append(ids, c.SupportingFactIDs...)
			ids = append(ids, c.SupportingRuleIDs...)
		}
		text := fmt.Sprintf("The synthesis stage concluded with %d tentative, non-binding conclusion(s).", len(cp.Opinion.Conclusions))
		return NarrativeSegment{Stage: stage, Text: text, RelatedNodeIDs: ids}, true

	case reasoningorchestration.StageUncertaintySurfacing:
		text := fmt.Sprintf("The uncertainty-surfacing stage flagged %d source(s) of doubt in the draft analysis.", len(cp.Uncertainty.Uncertainties))
		return NarrativeSegment{Stage: stage, Text: text, RelatedNodeIDs: uncertaintyIssueIDs(cp)}, true

	case reasoningorchestration.StageGuardrailCheck:
		verdict := "did not pass"
		if cp.GuardrailApproved {
			verdict = "passed"
		}
		text := fmt.Sprintf("The guardrail-check stage %s: the draft analysis and sign-off gate were reviewed before any finalization.", verdict)
		return NarrativeSegment{Stage: stage, Text: text}, true

	default:
		return NarrativeSegment{}, false
	}
}

// firstPartyArgumentIssueIDs extracts every IssueNodeID the first
// party's arguments addressed.
func firstPartyArgumentIssueIDs(cp reasoningorchestration.Checkpoint) []string {
	var ids []string
	for _, arg := range cp.FirstPartyArguments.Arguments {
		ids = append(ids, arg.IssueNodeID)
	}
	return ids
}

// secondPartyArgumentIssueIDs extracts every IssueNodeID the second
// party's arguments addressed.
func secondPartyArgumentIssueIDs(cp reasoningorchestration.Checkpoint) []string {
	var ids []string
	for _, arg := range cp.SecondPartyArguments.Arguments {
		ids = append(ids, arg.IssueNodeID)
	}
	return ids
}

// lawApplicationIssueIDs extracts every IssueNodeID the law-application
// stage produced an IssueApplication for.
func lawApplicationIssueIDs(cp reasoningorchestration.Checkpoint) []string {
	var ids []string
	for _, app := range cp.Law.IssueApplications {
		ids = append(ids, app.IssueNodeID)
	}
	return ids
}

// uncertaintyIssueIDs extracts every IssueNodeID an Uncertainty finding
// concerns.
func uncertaintyIssueIDs(cp reasoningorchestration.Checkpoint) []string {
	var ids []string
	for _, u := range cp.Uncertainty.Uncertainties {
		ids = append(ids, u.IssueNodeID)
	}
	return ids
}

// renderNarrative joins every segment's Text into one flat, ordered
// narrative string, one segment per paragraph.
func renderNarrative(segments []NarrativeSegment) string {
	texts := make([]string, 0, len(segments))
	for _, seg := range segments {
		texts = append(texts, seg.Text)
	}
	return strings.Join(texts, "\n\n")
}
