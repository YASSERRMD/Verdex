package knowledgeapi_test

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// TestValidationStatus_EmptyTree_CannotFinalize proves ValidationStatus
// surfaces treevalidation's ErrEmptyTree case as CanFinalize=false rather
// than propagating it as a hard error, since this endpoint's contract is
// to report status, not to fail on an unfinalizable tree.
func TestValidationStatus_EmptyTree_CannotFinalize(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	ctx := authedContext(identity.RoleJudge)

	resp, err := f.api.ValidationStatus(ctx, knowledgeapi.ValidationStatusRequest{CaseID: "case-a"})
	if err != nil {
		t.Fatalf("ValidationStatus: %v", err)
	}
	if resp.CanFinalize {
		t.Errorf("expected CanFinalize false for an empty tree")
	}
}

// TestValidationStatus_OrphanNode_SurfacesFinding proves ValidationStatus
// composes treevalidation's own orphan-detection check (DetectOrphans)
// rather than re-deriving it: a node with zero edges surfaces as a
// Finding in the response.
func TestValidationStatus_OrphanNode_SurfacesFinding(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	f.seedNode(t, irac.Node{
		ID: "issue-1", Type: irac.NodeIssue, CaseID: "case-a",
		Text: "An orphan issue with no edges.", CreatedAt: time.Now(), Confidence: 0.9,
	})

	ctx := authedContext(identity.RoleJudge)
	resp, err := f.api.ValidationStatus(ctx, knowledgeapi.ValidationStatusRequest{CaseID: "case-a"})
	if err != nil {
		t.Fatalf("ValidationStatus: %v", err)
	}
	if len(resp.Findings) == 0 {
		t.Fatalf("expected at least one finding for an orphan node")
	}

	foundOrphan := false
	for _, f := range resp.Findings {
		if f.NodeID == "issue-1" {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Fatalf("expected a finding referencing issue-1, got %+v", resp.Findings)
	}
}

// TestValidationStatus_WrongCaseID_Rejected proves a request naming a
// different case than the KnowledgeAPI instance is scoped to is rejected
// structurally.
func TestValidationStatus_WrongCaseID_Rejected(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t, "case-a")
	ctx := authedContext(identity.RoleJudge)

	_, err := f.api.ValidationStatus(ctx, knowledgeapi.ValidationStatusRequest{CaseID: "case-b"})
	if err != knowledgeapi.ErrEmptyCaseID {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}
