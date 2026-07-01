package identity

// Role is a named set of capabilities granted to a user within a tenant.
// Roles are additive: a user may hold more than one role simultaneously.
type Role string

const (
	// RoleJudge is the presiding officer on a case. A judge can view all
	// case materials and issue final decisions.
	RoleJudge Role = "judge"

	// RoleAdvocate represents counsel (plaintiff or defence). An advocate
	// can submit filings and view their own client's case materials.
	RoleAdvocate Role = "advocate"

	// RoleClerk is a court officer who assists judges with administrative
	// tasks: docketing filings, managing the schedule, and running
	// case-status reports.
	RoleClerk Role = "clerk"

	// RoleAdmin has full control over tenant configuration, user
	// management, and system settings. Admin does not automatically imply
	// judicial powers.
	RoleAdmin Role = "admin"

	// RoleAuditor has read-only access to audit trails, system logs, and
	// aggregate reporting. An auditor cannot modify any data.
	RoleAuditor Role = "auditor"
)

// Roles is the ordered slice of every valid Role constant. It is useful
// for validation loops, seeding test fixtures, and generating the RBAC
// matrix documentation.
var Roles = []Role{
	RoleJudge,
	RoleAdvocate,
	RoleClerk,
	RoleAdmin,
	RoleAuditor,
}

// IsValid reports whether r is one of the named Role constants defined
// by this package. Unknown or empty strings return false.
func (r Role) IsValid() bool {
	for _, known := range Roles {
		if r == known {
			return true
		}
	}
	return false
}

// String returns the string value of the role, satisfying fmt.Stringer.
func (r Role) String() string { return string(r) }
