package reportexport_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/reportexport"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// newTestUser builds an identity.User scoped to tenantID, mirroring
// packages/notifications's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, role identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "judge@example.test",
		Name:     "Test User",
		Roles:    []identity.Role{role},
		Status:   identity.UserStatusActive,
	}
}

// ctxWithUser returns a context carrying user, mirroring how an HTTP
// middleware layer would attach the authenticated actor.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// newTestCase builds a minimal, valid *caselifecycle.Case for tests
// that don't exercise caselifecycle's own lifecycle behavior.
func newTestCase(tenantID uuid.UUID) *caselifecycle.Case {
	now := time.Now().UTC()
	return &caselifecycle.Case{
		ID:              uuid.New(),
		TenantID:        tenantID,
		JurisdictionID:  uuid.New(),
		Title:           "Doe v. Acme Corp",
		Reference:       "DKT-2026-001",
		State:           caselifecycle.StateDraft,
		Metadata:        map[string]string{},
		MetadataVersion: 1,
		CreatedBy:       uuid.New(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// newTestOpinion builds a synthesisagent.Opinion with one grounded
// conclusion carrying enough narrative text for redaction/rendering
// assertions, plus a PII-bearing analysis for the redaction tests.
func newTestOpinion(caseID uuid.UUID, analysis string) *synthesisagent.Opinion {
	return &synthesisagent.Opinion{
		CaseID: caseID.String(),
		Conclusions: []synthesisagent.TentativeConclusion{
			{
				IssueNodeID:       "issue-1",
				Text:              analysis,
				FavoredParty:      "first_party",
				Confidence:        0.82,
				WeakestLink:       "Witness testimony is uncorroborated.",
				SupportingFactIDs: []string{"fact-1", "fact-2"},
				SupportingRuleIDs: []string{"rule-1"},
				Grounded:          true,
			},
		},
		SkippedIssueNodeIDs: []string{"issue-2"},
		GeneratedAt:         time.Now().UTC(),
	}
}

// newAssembledReport builds a Report via Assemble with one
// jurisdiction-formatted citation attached to its only issue.
func newAssembledReport(t *testing.T, c *caselifecycle.Case, opinion *synthesisagent.Opinion) *reportexport.Report {
	t.Helper()

	registry := citation.NewDefaultRegistry()
	input := reportexport.AssembleInput{
		JurisdictionKey: "common_law",
		Citations:       registry,
		AuthorityTrailsByIssue: map[string][]reportexport.AuthorityCitationInput{
			"issue-1": {
				{
					RuleID: "rule-1",
					FormatInput: citation.FormatInput{
						Origin:  citation.OriginStatute,
						Act:     "Contracts Act",
						Section: "12",
					},
					Resolved: true,
					Verified: true,
				},
			},
		},
	}

	report, err := reportexport.Assemble(c, opinion, input)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	return report
}

// newTestService builds a reportexport.Service backed by a fresh
// InMemoryAuditRepository.
func newTestService(t *testing.T) (*reportexport.Service, *reportexport.InMemoryAuditRepository) {
	t.Helper()

	repo := reportexport.NewInMemoryAuditRepository()
	svc, err := reportexport.NewService(repo)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, repo
}
