package identity

// Permission represents a fine-grained capability that can be granted
// to a role. Permissions are checked at request time via HasPermission.
type Permission string

const (
	// Case-related permissions.

	// PermViewCase allows reading case details, filings, and attached
	// documents.
	PermViewCase Permission = "case:view"

	// PermEditCase allows creating or updating case metadata and
	// submitting filings on behalf of a party.
	PermEditCase Permission = "case:edit"

	// PermSignOff allows a judge to issue a decision, order, or ruling
	// that constitutes the authoritative disposition of a case.
	PermSignOff Permission = "case:sign_off"

	// PermDeleteCase allows hard-deleting a case record. This is an
	// administrative operation.
	PermDeleteCase Permission = "case:delete"

	// Hearing and scheduling permissions.

	// PermScheduleHearing allows creating or modifying hearing slots on
	// the docket.
	PermScheduleHearing Permission = "hearing:schedule"

	// PermViewHearing allows reading hearing details and schedules.
	PermViewHearing Permission = "hearing:view"

	// User management permissions.

	// PermManageUsers allows inviting, disabling, enabling, and changing
	// the roles of users within the tenant.
	PermManageUsers Permission = "users:manage"

	// PermViewUsers allows listing users within the tenant.
	PermViewUsers Permission = "users:view"

	// Audit permissions.

	// PermAuditRead allows reading the immutable audit trail, system
	// event logs, and aggregate compliance reports.
	PermAuditRead Permission = "audit:read"

	// System / configuration permissions.

	// PermManageSettings allows changing tenant-level configuration such
	// as integrations, notification rules, and feature flags.
	PermManageSettings Permission = "settings:manage"
)

// PermissionMatrix maps each Role to the full set of Permissions it
// holds. The matrix is the single authoritative source of truth; all
// runtime enforcement (HasPermission, RequirePermission middleware, and
// the RBAC matrix documentation in doc/rbac-matrix.md) derives from it.
var PermissionMatrix = map[Role][]Permission{
	RoleJudge: {
		PermViewCase,
		PermSignOff,
		PermViewHearing,
		PermScheduleHearing,
		PermViewUsers,
		PermAuditRead,
	},
	RoleAdvocate: {
		PermViewCase,
		PermEditCase,
		PermViewHearing,
	},
	RoleClerk: {
		PermViewCase,
		PermEditCase,
		PermScheduleHearing,
		PermViewHearing,
		PermViewUsers,
	},
	RoleAdmin: {
		PermViewCase,
		PermEditCase,
		PermDeleteCase,
		PermScheduleHearing,
		PermViewHearing,
		PermManageUsers,
		PermViewUsers,
		PermManageSettings,
		PermAuditRead,
	},
	RoleAuditor: {
		PermViewCase,
		PermViewHearing,
		PermViewUsers,
		PermAuditRead,
	},
}

// HasPermission reports whether role holds perm according to the
// PermissionMatrix. Unknown roles always return false.
func HasPermission(role Role, perm Permission) bool {
	perms, ok := PermissionMatrix[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}
