package identity_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestRoleIsValid(t *testing.T) {
	t.Parallel()

	valid := []identity.Role{
		identity.RoleJudge,
		identity.RoleAdvocate,
		identity.RoleClerk,
		identity.RoleAdmin,
		identity.RoleAuditor,
	}
	for _, r := range valid {
		r := r
		t.Run(string(r)+"_valid", func(t *testing.T) {
			t.Parallel()
			if !r.IsValid() {
				t.Errorf("expected role %q to be valid", r)
			}
		})
	}

	invalid := []identity.Role{
		"",
		"superuser",
		"JUDGE",
		"Judge",
		" judge",
	}
	for _, r := range invalid {
		r := r
		t.Run(string(r)+"_invalid", func(t *testing.T) {
			t.Parallel()
			if r.IsValid() {
				t.Errorf("expected role %q to be invalid", r)
			}
		})
	}
}

func TestRolesSliceContainsAllRoles(t *testing.T) {
	t.Parallel()

	expected := map[identity.Role]bool{
		identity.RoleJudge:    false,
		identity.RoleAdvocate: false,
		identity.RoleClerk:    false,
		identity.RoleAdmin:    false,
		identity.RoleAuditor:  false,
	}
	for _, r := range identity.Roles {
		if _, ok := expected[r]; !ok {
			t.Errorf("unexpected role %q in identity.Roles slice", r)
			continue
		}
		expected[r] = true
	}
	for r, seen := range expected {
		if !seen {
			t.Errorf("role %q missing from identity.Roles slice", r)
		}
	}
}

func TestRoleString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		role identity.Role
		want string
	}{
		{identity.RoleJudge, "judge"},
		{identity.RoleAdvocate, "advocate"},
		{identity.RoleClerk, "clerk"},
		{identity.RoleAdmin, "admin"},
		{identity.RoleAuditor, "auditor"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.role.String(); got != tc.want {
				t.Errorf("Role.String() = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestPermissionMatrixAllRolesPresent(t *testing.T) {
	t.Parallel()

	for _, r := range identity.Roles {
		r := r
		t.Run(string(r), func(t *testing.T) {
			t.Parallel()
			perms, ok := identity.PermissionMatrix[r]
			if !ok {
				t.Errorf("role %q has no entry in PermissionMatrix", r)
				return
			}
			if len(perms) == 0 {
				t.Errorf("role %q has an empty permission list in PermissionMatrix", r)
			}
		})
	}
}
