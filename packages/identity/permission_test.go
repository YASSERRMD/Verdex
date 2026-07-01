package identity_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// expectedPermissions is the authoritative per-role permission set used
// in tests. It must stay in sync with PermissionMatrix in permission.go;
// having it here independently makes divergence visible as a test failure.
var expectedPermissions = map[identity.Role][]identity.Permission{
	identity.RoleJudge: {
		identity.PermViewCase,
		identity.PermSignOff,
		identity.PermViewHearing,
		identity.PermScheduleHearing,
		identity.PermViewUsers,
		identity.PermAuditRead,
	},
	identity.RoleAdvocate: {
		identity.PermViewCase,
		identity.PermEditCase,
		identity.PermViewHearing,
	},
	identity.RoleClerk: {
		identity.PermViewCase,
		identity.PermEditCase,
		identity.PermScheduleHearing,
		identity.PermViewHearing,
		identity.PermViewUsers,
	},
	identity.RoleAdmin: {
		identity.PermViewCase,
		identity.PermEditCase,
		identity.PermDeleteCase,
		identity.PermScheduleHearing,
		identity.PermViewHearing,
		identity.PermManageUsers,
		identity.PermViewUsers,
		identity.PermManageSettings,
		identity.PermAuditRead,
	},
	identity.RoleAuditor: {
		identity.PermViewCase,
		identity.PermViewHearing,
		identity.PermViewUsers,
		identity.PermAuditRead,
	},
}

// allPermissions is every permission constant so we can verify that no
// extra permissions are silently granted.
var allPermissions = []identity.Permission{
	identity.PermViewCase,
	identity.PermEditCase,
	identity.PermSignOff,
	identity.PermDeleteCase,
	identity.PermScheduleHearing,
	identity.PermViewHearing,
	identity.PermManageUsers,
	identity.PermViewUsers,
	identity.PermAuditRead,
	identity.PermManageSettings,
}

func TestEachRoleHasExactlyTheRightPermissions(t *testing.T) {
	t.Parallel()

	for role, want := range expectedPermissions {
		role, want := role, want
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()

			wantSet := make(map[identity.Permission]bool, len(want))
			for _, p := range want {
				wantSet[p] = true
			}

			// Every expected permission must be granted.
			for _, p := range want {
				if !identity.HasPermission(role, p) {
					t.Errorf("role %q should have permission %q but HasPermission returned false", role, p)
				}
			}

			// No unexpected permission should be granted.
			for _, p := range allPermissions {
				if wantSet[p] {
					continue // already checked above
				}
				if identity.HasPermission(role, p) {
					t.Errorf("role %q should NOT have permission %q but HasPermission returned true", role, p)
				}
			}
		})
	}
}

func TestHasPermissionUnknownRole(t *testing.T) {
	t.Parallel()

	for _, p := range allPermissions {
		p := p
		t.Run(string(p), func(t *testing.T) {
			t.Parallel()
			if identity.HasPermission("unknown_role", p) {
				t.Errorf("unknown role should not have permission %q", p)
			}
		})
	}
}

func TestPermissionMatrixLength(t *testing.T) {
	t.Parallel()

	// Verify the matrix in permission.go has the same number of entries
	// as our test fixture.
	if got, want := len(identity.PermissionMatrix), len(expectedPermissions); got != want {
		t.Errorf("PermissionMatrix has %d roles; expected %d", got, want)
	}

	for role, want := range expectedPermissions {
		got, ok := identity.PermissionMatrix[role]
		if !ok {
			t.Errorf("role %q missing from PermissionMatrix", role)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("role %q: PermissionMatrix has %d permissions; expected %d", role, len(got), len(want))
		}
	}
}
