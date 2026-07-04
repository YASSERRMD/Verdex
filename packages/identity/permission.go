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

	// Key management permissions (packages/keymanagement, Phase 076).

	// PermViewKeys allows reading key metadata (ID, version, state,
	// timestamps) for the tenant's encryption keys. It does not grant
	// access to key material itself -- packages/keymanagement never
	// returns raw key bytes through any operation gated solely by this
	// permission.
	PermViewKeys Permission = "keys:view"

	// PermManageKeys allows rotating and revoking the tenant's
	// encryption keys. This was genuinely missing before Phase 076:
	// no existing permission named key lifecycle operations, so this
	// phase adds it rather than overloading PermManageSettings, which
	// covers unrelated tenant configuration.
	PermManageKeys Permission = "keys:manage"

	// PermBreakGlassKeys allows invoking the emergency, time-bound,
	// justification-required break-glass procedure to retrieve or use
	// a key outside the normal access flow. Deliberately not granted
	// to any role except RoleAdmin -- break-glass is an
	// emergency-only, heavily audited escalation, not a routine
	// capability.
	PermBreakGlassKeys Permission = "keys:break_glass"

	// Privacy / data-subject-rights permissions (packages/privacy,
	// Phase 081).

	// PermViewPrivacy allows read-only access to the tenant's data
	// inventory, retention report, and privacy audit trail. It does not
	// permit processing a subject-access, erasure, or consent-change
	// request -- see PermManagePrivacy.
	PermViewPrivacy Permission = "privacy:view"

	// PermManagePrivacy allows processing subject-access requests,
	// executing right-to-erasure requests, and recording
	// consent/legal-basis changes. This was genuinely missing before
	// Phase 081: no existing permission named data-subject-rights
	// processing, so this phase adds it rather than overloading
	// PermManageSettings, which covers unrelated tenant configuration.
	PermManagePrivacy Permission = "privacy:manage"

	// Compliance mapping permissions (packages/compliance, Phase 082).

	// PermViewCompliance allows read-only access to the control
	// catalogue, a tenant's compliance profile, collected control
	// evidence, gap-analysis reports, and the compliance dashboard. It
	// does not permit registering controls, recording evidence, or
	// changing a tenant's compliance profile -- see
	// PermManageCompliance.
	PermViewCompliance Permission = "compliance:view"

	// PermManageCompliance allows registering/updating catalogued
	// controls, recording control evidence, and setting a tenant's
	// ComplianceProfile. This was genuinely missing before Phase 082:
	// no existing permission named compliance-mapping administration,
	// so this phase adds it rather than overloading
	// PermManageSettings, which covers unrelated tenant configuration.
	PermManageCompliance Permission = "compliance:manage"

	// Threat-modeling permissions (packages/threatmodel, Phase 083).

	// PermViewThreatmodel allows read-only access to the STRIDE threat
	// catalogue and a mitigation's recorded status-transition history.
	// It does not permit transitioning a mitigation's status -- see
	// PermManageThreatmodel.
	PermViewThreatmodel Permission = "threatmodel:view"

	// PermManageThreatmodel allows transitioning a catalogued
	// Mitigation's status (e.g. Planned -> Implemented -> Verified).
	// This was genuinely missing before Phase 083: no existing
	// permission named threat-model/mitigation-status administration,
	// so this phase adds it rather than overloading
	// PermManageSettings, which covers unrelated tenant configuration.
	PermManageThreatmodel Permission = "threatmodel:manage"

	// Vulnerability and dependency management permissions
	// (packages/vulnmanagement, Phase 084).

	// PermViewVulnmanagement allows read-only access to scanner
	// findings, triage decision history, SLA-breach reports, and the
	// vulnerability-management dashboard. It does not permit recording
	// findings, triaging them, or transitioning a finding's status --
	// see PermManageVulnmanagement.
	PermViewVulnmanagement Permission = "vulnmanagement:view"

	// PermManageVulnmanagement allows recording scanner findings,
	// triaging them (Engine.Triage), and transitioning a finding's
	// remediation Status. This was genuinely missing before Phase 084:
	// no existing permission named vulnerability-finding/triage
	// administration, so this phase adds it rather than overloading
	// PermManageSettings, which covers unrelated tenant configuration.
	PermManageVulnmanagement Permission = "vulnmanagement:manage"
	// Backup / disaster-recovery permissions (packages/backupdr, Phase
	// 085).

	// PermViewBackupDR allows read-only access to a tenant's backup
	// policies, backup record history, restore-drill history, and
	// RPO/RTO evaluation results. It does not permit setting a backup
	// policy, recording a backup, or executing a restore drill -- see
	// PermManageBackupDR.
	PermViewBackupDR Permission = "backupdr:view"

	// PermManageBackupDR allows setting a tenant's BackupPolicy per
	// DataClass, recording BackupRecord entries, and executing
	// RestoreDrill runs. This was genuinely missing before Phase 085:
	// no existing permission named backup/DR administration, so this
	// phase adds it rather than overloading PermManageSettings, which
	// covers unrelated tenant configuration.
	PermManageBackupDR Permission = "backupdr:manage"

	// External system integration permissions (packages/integration,
	// Phase 087).

	// PermViewIntegration allows read-only access to registered
	// connector configurations (minus credential material), import and
	// delivery run history, and reconciliation results. It does not
	// permit registering a connector, triggering an import/delivery, or
	// changing credentials -- see PermManageIntegration.
	PermViewIntegration Permission = "integration:view"

	// PermManageIntegration allows registering/updating connector
	// configurations, setting connector credentials, triggering inbound
	// case imports and outbound report deliveries, and running
	// reconciliation. This was genuinely missing before Phase 087: no
	// existing permission named external-system-integration
	// administration, so this phase adds it rather than overloading
	// PermManageSettings, which covers unrelated tenant configuration.
	PermManageIntegration Permission = "integration:manage"
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
		PermViewKeys,
		PermManageKeys,
		PermBreakGlassKeys,
		PermViewPrivacy,
		PermManagePrivacy,
		PermViewCompliance,
		PermManageCompliance,
		PermViewThreatmodel,
		PermManageThreatmodel,
		PermViewVulnmanagement,
		PermManageVulnmanagement,
		PermViewBackupDR,
		PermManageBackupDR,
		PermViewIntegration,
		PermManageIntegration,
	},
	RoleAuditor: {
		PermViewCase,
		PermViewHearing,
		PermViewUsers,
		PermAuditRead,
		PermViewKeys,
		PermViewPrivacy,
		PermViewCompliance,
		PermViewThreatmodel,
		PermViewVulnmanagement,
		PermViewBackupDR,
		PermViewIntegration,
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
