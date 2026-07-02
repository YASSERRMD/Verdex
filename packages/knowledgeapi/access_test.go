package knowledgeapi_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// TestGetTree_Unauthenticated_Rejected proves that a call with no
// identity.User on the context is rejected before any store is touched,
// per the access-control model documented on access.go.
func TestGetTree_Unauthenticated_Rejected(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t)

	_, err := f.api.GetTree(context.Background(), knowledgeapi.GetTreeRequest{CaseID: "case-a"})
	if !errors.Is(err, knowledgeapi.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

// TestGetTree_MissingPermission_Rejected proves that an authenticated
// actor lacking identity.PermViewCase is rejected with ErrForbidden. No
// role in identity.PermissionMatrix omits PermViewCase entirely except an
// unrecognized/empty role, so this test uses a user with no roles at all.
func TestGetTree_MissingPermission_Rejected(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t)

	ctx := authedContext() // no roles => no permissions
	_, err := f.api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: "case-a"})
	if !errors.Is(err, knowledgeapi.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

// TestGetTree_AuthorizedRole_Allowed proves every role granted
// PermViewCase in identity.PermissionMatrix can successfully call a
// KnowledgeAPI read method.
func TestGetTree_AuthorizedRole_Allowed(t *testing.T) {
	t.Parallel()

	for _, role := range identity.Roles {
		role := role
		if !identity.HasPermission(role, identity.PermViewCase) {
			continue
		}
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()

			f := newTestFixture(t)
			ctx := authedContext(role)

			_, err := f.api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: "case-a"})
			if err != nil {
				t.Fatalf("expected role %q to be authorized, got %v", role, err)
			}
		})
	}
}

// TestEveryMethod_RequiresAuthentication proves the access-control gate
// is applied uniformly across every exported KnowledgeAPI method, not
// just GetTree, so a future endpoint added to this facade cannot
// accidentally skip the RBAC check.
func TestEveryMethod_RequiresAuthentication(t *testing.T) {
	t.Parallel()

	f := newTestFixture(t)
	ctx := context.Background()

	checks := []struct {
		name string
		call func() error
	}{
		{"GetTree", func() error {
			_, err := f.api.GetTree(ctx, knowledgeapi.GetTreeRequest{CaseID: "case-a"})
			return err
		}},
		{"GetNode", func() error {
			_, err := f.api.GetNode(ctx, knowledgeapi.GetNodeRequest{CaseID: "case-a", NodeID: "n1"})
			return err
		}},
		{"LookupPaths", func() error {
			_, err := f.api.LookupPaths(ctx, knowledgeapi.LookupPathsRequest{CaseID: "case-a", FromNodeID: "n1", EdgeType: "governs"})
			return err
		}},
		{"Retrieve", func() error {
			_, err := f.api.Retrieve(ctx, knowledgeapi.RetrieveRequest{CaseID: "case-a", AnchorNodeID: "n1"})
			return err
		}},
		{"ResolveCitation", func() error {
			_, err := f.api.ResolveCitation(ctx, knowledgeapi.ResolveCitationRequest{CaseID: "case-a", NodeID: "n1"})
			return err
		}},
		{"ValidationStatus", func() error {
			_, err := f.api.ValidationStatus(ctx, knowledgeapi.ValidationStatusRequest{CaseID: "case-a"})
			return err
		}},
	}

	for _, c := range checks {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if err := c.call(); !errors.Is(err, knowledgeapi.ErrUnauthenticated) {
				t.Fatalf("%s: expected ErrUnauthenticated, got %v", c.name, err)
			}
		})
	}
}
