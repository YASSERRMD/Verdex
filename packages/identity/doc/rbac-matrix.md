# Verdex RBAC Permission Matrix

This document is generated from `packages/identity/permission.go` and
`packages/identity/role.go`. The permission matrix is the single source
of truth; this table is for human reference only.

## Roles

| Role       | Description                                                       |
|------------|-------------------------------------------------------------------|
| `judge`    | Presiding officer; issues decisions and orders on cases           |
| `advocate` | Counsel (plaintiff or defence); submits filings for their client  |
| `clerk`    | Court officer; dockets filings, manages hearings, assists judges  |
| `admin`    | Tenant administrator; manages users, settings, and system config  |
| `auditor`  | Read-only access to audit trails and compliance reports           |

## Permissions

| Permission          | Description                                                    |
|---------------------|----------------------------------------------------------------|
| `case:view`         | Read case details, filings, and documents                      |
| `case:edit`         | Create or update case metadata and submit filings              |
| `case:sign_off`     | Issue a final decision, order, or ruling on a case             |
| `case:delete`       | Hard-delete a case record (administrative)                     |
| `hearing:schedule`  | Create or modify hearing slots on the docket                   |
| `hearing:view`      | Read hearing details and schedules                             |
| `users:manage`      | Invite, disable, enable, and re-role users within the tenant   |
| `users:view`        | List users within the tenant                                   |
| `audit:read`        | Read the immutable audit trail and aggregate reports           |
| `settings:manage`   | Change tenant-level configuration (integrations, flags, etc.)  |
| `keys:view`         | Read key metadata (ID, version, state) -- not key material      |
| `keys:manage`       | Rotate and revoke the tenant's encryption keys                 |
| `keys:break_glass`  | Invoke the emergency, justified, time-bound break-glass path   |
| `privacy:view`      | Read-only access to the data inventory and privacy audit trail |
| `privacy:manage`    | Process subject-access, erasure, and consent-change requests   |
| `compliance:view`   | Read-only access to the control catalogue and compliance dashboard |
| `compliance:manage` | Register controls, record evidence, set a compliance profile  |

## Matrix

A tick (✓) means the role holds that permission; a dash (–) means it
does not.

| Permission          | judge | advocate | clerk | admin | auditor |
|---------------------|:-----:|:--------:|:-----:|:-----:|:-------:|
| `case:view`         |   ✓   |    ✓     |   ✓   |   ✓   |    ✓    |
| `case:edit`         |   –   |    ✓     |   ✓   |   ✓   |    –    |
| `case:sign_off`     |   ✓   |    –     |   –   |   –   |    –    |
| `case:delete`       |   –   |    –     |   –   |   ✓   |    –    |
| `hearing:schedule`  |   ✓   |    –     |   ✓   |   ✓   |    –    |
| `hearing:view`      |   ✓   |    ✓     |   ✓   |   ✓   |    ✓    |
| `users:manage`      |   –   |    –     |   –   |   ✓   |    –    |
| `users:view`        |   ✓   |    –     |   ✓   |   ✓   |    ✓    |
| `audit:read`        |   ✓   |    –     |   –   |   ✓   |    ✓    |
| `settings:manage`   |   –   |    –     |   –   |   ✓   |    –    |
| `keys:view`         |   –   |    –     |   –   |   ✓   |    ✓    |
| `keys:manage`       |   –   |    –     |   –   |   ✓   |    –    |
| `keys:break_glass`  |   –   |    –     |   –   |   ✓   |    –    |
| `privacy:view`      |   –   |    –     |   –   |   ✓   |    ✓    |
| `privacy:manage`    |   –   |    –     |   –   |   ✓   |    –    |
| `compliance:view`   |   –   |    –     |   –   |   ✓   |    ✓    |
| `compliance:manage` |   –   |    –     |   –   |   ✓   |    –    |

## Design notes

* Roles are additive. A user holding both `judge` and `clerk` has the
  union of both permission sets.
* `admin` does not imply judicial authority. An admin cannot issue case
  decisions (`case:sign_off`) unless they also hold the `judge` role.
* `auditor` is intentionally read-only across all dimensions. It cannot
  modify any data.
* `advocate` cannot schedule hearings — that is an administrative
  function performed by a clerk or judge.
* The permission matrix is enforced at runtime by
  `identity.HasPermission` and the `RequirePermission` HTTP middleware.
  The database layer (Row-Level Security, Phase 005) provides a
  second defence-in-depth layer against cross-tenant data access.
* `keys:manage` and `keys:break_glass` (Phase 076,
  `packages/keymanagement`) are admin-only by design: key rotation/
  revocation and emergency break-glass access are the highest-blast-
  radius operations in the system and are deliberately not delegated
  to any other role. `auditor` holds `keys:view` (metadata only, never
  key material) consistent with its read-only, compliance-facing
  posture elsewhere in this matrix.
* `privacy:manage` (Phase 081, `packages/privacy`) is admin-only:
  processing a subject-access or right-to-erasure request, or
  recording a consent/legal-basis change, is a data-subject-rights
  action with real legal consequences and is deliberately not
  delegated beyond the tenant administrator. `auditor` holds
  `privacy:view` (read-only access to the data inventory, retention
  report, and privacy audit trail) consistent with its compliance-
  facing posture elsewhere in this matrix.
* `compliance:manage` (Phase 082, `packages/compliance`) is
  admin-only: registering/updating a catalogued control, recording
  control evidence, and setting a tenant's compliance profile are
  administrative actions that shape what a compliance report and gap
  analysis certify. `auditor` holds `compliance:view` (read-only
  access to the control catalogue, a tenant's compliance profile,
  collected evidence, gap-analysis reports, and the compliance
  dashboard) consistent with its compliance-facing posture elsewhere
  in this matrix.
