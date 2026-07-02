package lawapplication

import (
	"fmt"
	"sort"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

// Apply computes a Result for req: for every issue in req.Issues, it
// maps controlling rules (MapIssueToControllingRules), builds the
// element-to-fact map for each controlling rule (BuildElementFactMap),
// detects conflicting authority (DetectConflictingAuthority), attaches
// citations (AttachCitations), and derives a Confidence score with an
// explicit reasoning trail (ComputeConfidence).
//
// Returns ErrEmptyCaseID if req.CaseID is empty, ErrNoIssues if
// req.Issues is empty, and ErrCaseIDMismatch if either non-empty
// ArgumentSet or a non-empty req.Evidence.CaseID references a case ID
// other than req.CaseID.
func Apply(req Request) (Result, error) {
	if req.CaseID == "" {
		return Result{}, ErrEmptyCaseID
	}
	if len(req.Issues) == 0 {
		return Result{}, ErrNoIssues
	}

	firstArgs := req.FirstParty.Arguments
	secondArgs := req.SecondParty.Arguments

	if len(firstArgs) > 0 && req.FirstParty.CaseID != "" && req.FirstParty.CaseID != req.CaseID {
		return Result{}, ErrCaseIDMismatch
	}
	if len(secondArgs) > 0 && req.SecondParty.CaseID != "" && req.SecondParty.CaseID != req.CaseID {
		return Result{}, ErrCaseIDMismatch
	}
	if req.Evidence.CaseID != "" && req.Evidence.CaseID != req.CaseID {
		return Result{}, ErrCaseIDMismatch
	}

	args := make([]ArgumentRef, 0, len(firstArgs)+len(secondArgs))
	args = append(args, argumentRefsFromFirstParty(req.FirstParty)...)
	args = append(args, argumentRefsFromSecondParty(req.SecondParty)...)

	applications := make([]IssueApplication, 0, len(req.Issues))
	for _, input := range req.Issues {
		applications = append(applications, applyToIssue(input, req.Rules, args, req.Evidence, req.LegalFamily, req.CitationLookup))
	}

	return Result{
		CaseID:            req.CaseID,
		IssueApplications: applications,
		GeneratedAt:       time.Now(),
	}, nil
}

// applyToIssue computes the IssueApplication for a single IssueInput.
func applyToIssue(
	input IssueInput,
	rules []RuleRef,
	args []ArgumentRef,
	evidence evidenceweighing.Result,
	family LegalFamily,
	lookup CitationLookupFunc,
) IssueApplication {
	issueID := input.Issue.SourceIssueNodeID

	controllingRuleIDs := MapIssueToControllingRules(issueID, input.GoverningRuleIDs, args)

	var steps []Step
	steps = append(steps, Step{Description: fmt.Sprintf(
		"mapped issue %q to %d controlling rule(s): %d from governs edges, union with rules cited in either party's SupportingRuleIDs",
		issueID, len(controllingRuleIDs), len(input.GoverningRuleIDs),
	)})

	var elementFacts []ElementFactEntry
	for _, ruleID := range controllingRuleIDs {
		elementFacts = append(elementFacts, BuildElementFactMap(issueID, ruleID, args, evidence)...)
	}
	steps = append(steps, Step{Description: fmt.Sprintf(
		"built element-fact map with %d entrie(s) across %d controlling rule(s)",
		len(elementFacts), len(controllingRuleIDs),
	)})

	conflicts := DetectConflictingAuthority(issueID, controllingRuleIDs, args)
	if len(conflicts) > 0 {
		steps = append(steps, Step{Description: fmt.Sprintf(
			"detected %d conflicting authority finding(s) among controlling rules", len(conflicts),
		)})
	} else {
		steps = append(steps, Step{Description: "no conflicting authority detected among controlling rules"})
	}

	citations := AttachCitations(controllingRuleIDs, rules, lookup)
	unresolved := 0
	for _, c := range citations {
		if !c.Resolved || !c.Verified {
			unresolved++
		}
	}
	if unresolved > 0 {
		steps = append(steps, Step{Description: fmt.Sprintf(
			"%d of %d controlling rule citation(s) are unresolved or unverified", unresolved, len(citations),
		)})
	}

	confidence, confidenceSteps := ComputeConfidence(controllingRuleIDs, rules, citations, elementFacts, conflicts, family)
	steps = append(steps, confidenceSteps...)

	sort.Strings(controllingRuleIDs)

	return IssueApplication{
		IssueNodeID:        issueID,
		ControllingRuleIDs: controllingRuleIDs,
		ElementFactMap:     elementFacts,
		Conflicts:          conflicts,
		Citations:          citations,
		Confidence:         confidence,
		Steps:              steps,
	}
}
