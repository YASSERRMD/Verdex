package analytics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/analytics"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestRequireViewPermission_Unauthenticated(t *testing.T) {
	if err := analytics.RequireViewPermission(unauthedContext()); !errors.Is(err, analytics.ErrUnauthenticated) {
		t.Errorf("RequireViewPermission(unauthed) error = %v, want ErrUnauthenticated", err)
	}
}

func TestRequireViewPermission_AllowsRolesHoldingPermViewCase(t *testing.T) {
	for _, role := range identity.Roles {
		if !identity.HasPermission(role, identity.PermViewCase) {
			continue
		}
		ctx := identity.WithUser(context.Background(), newTestUser(role))
		if err := analytics.RequireViewPermission(ctx); err != nil {
			t.Errorf("RequireViewPermission(%s) error = %v, want nil", role, err)
		}
	}
}

func TestRequireCostPermission_AllowsOnlyAuditRoles(t *testing.T) {
	for _, role := range identity.Roles {
		ctx := identity.WithUser(context.Background(), newTestUser(role))
		err := analytics.RequireCostPermission(ctx)
		wantAllowed := identity.HasPermission(role, identity.PermAuditRead)
		if wantAllowed && err != nil {
			t.Errorf("RequireCostPermission(%s) error = %v, want nil (role holds PermAuditRead)", role, err)
		}
		if !wantAllowed && !errors.Is(err, analytics.ErrForbidden) {
			t.Errorf("RequireCostPermission(%s) error = %v, want ErrForbidden (role lacks PermAuditRead)", role, err)
		}
	}
}
