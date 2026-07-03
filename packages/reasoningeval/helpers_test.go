package reasoningeval_test

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/grounding"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/reasoningeval"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

const testCaseID = "case-reasoningeval"

// newTestUser mirrors packages/grounding/helpers_test.go's fixture
// convention.
func newTestUser(roles ...identity.Role) *identity.User {
	return &identity.User{
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     roles,
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func authedContext() context.Context {
	return identity.WithUser(context.Background(), newTestUser(identity.RoleAdvocate))
}

func unauthedContext() context.Context {
	return context.Background()
}

// wellGroundedOpinion returns a synthesisagent.Opinion with two
// substantive, high-confidence conclusions and no skipped issues.
func wellGroundedOpinion() synthesisagent.Opinion {
	return synthesisagent.Opinion{
		CaseID: testCaseID,
		Conclusions: []synthesisagent.TentativeConclusion{
			{
				IssueNodeID: "issue-1",
				Text:        "The evidence establishes that the contract was validly formed: both parties signed a written memorandum satisfying the statute of frauds, and consideration was exchanged.",
				Confidence:  0.9,
			},
			{
				IssueNodeID: "issue-2",
				Text:        "The breach claim is well supported by the delivery records and the missed deadline documented in the case file, weighing in favor of the plaintiff's position.",
				Confidence:  0.85,
			},
		},
		GeneratedAt: time.Now(),
	}
}

// thinOpinion returns a synthesisagent.Opinion with a skipped issue and a
// trivial, low-confidence conclusion — meant to score poorly on
// coherence.
func thinOpinion() synthesisagent.Opinion {
	return synthesisagent.Opinion{
		CaseID: testCaseID,
		Conclusions: []synthesisagent.TentativeConclusion{
			{
				IssueNodeID: "issue-1",
				Text:        "Unclear.",
				Confidence:  0.1,
			},
		},
		SkippedIssueNodeIDs: []string{"issue-2", "issue-3"},
		GeneratedAt:         time.Now(),
	}
}

// fullyGroundedReport returns a grounding.Report with a perfect score and
// no findings at all.
func fullyGroundedReport() grounding.Report {
	return grounding.Report{
		CaseID:       testCaseID,
		OpinionScore: 1.0,
		GeneratedAt:  time.Now(),
	}
}

// findingsReport returns a grounding.Report carrying one critical and one
// warning citation finding, and a lower opinion score.
func findingsReport() grounding.Report {
	return grounding.Report{
		CaseID:       testCaseID,
		OpinionScore: 0.4,
		Conclusions: []grounding.ConclusionResult{
			{
				IssueNodeID: "issue-1",
				CitationFindings: []citation.Finding{
					{Severity: citation.SeverityCritical, Code: citation.CodeHallucinated, NodeID: "rule-1", CaseID: testCaseID},
					{Severity: citation.SeverityWarning, Code: citation.CodeUnresolved, NodeID: "rule-2", CaseID: testCaseID},
				},
			},
		},
		GeneratedAt: time.Now(),
	}
}

// scoreInput bundles opinion and report into a ScoreInput for
// jurisdictionCode.
// floatNear reports whether a and b are within a small epsilon of each
// other, to guard against floating-point summation drift in aggregation
// tests.
func floatNear(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func scoreInput(opinion synthesisagent.Opinion, report grounding.Report, jurisdictionCode string) reasoningeval.ScoreInput {
	return reasoningeval.ScoreInput{
		CaseID:           opinion.CaseID,
		JurisdictionCode: jurisdictionCode,
		Opinion:          reasoningeval.WrapOpinion(opinion),
		GroundingReport:  reasoningeval.WrapGroundingReport(report),
	}
}
