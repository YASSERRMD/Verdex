// Package accessgovernance is Phase 080, the capstone that draws the
// RBAC/tenancy/audit threads running through this codebase since
// Phase 006 (identity's role/permission model, Phase 063's per-case
// lifecycle, Phase 076's key-management break-glass, and Phase 077's
// hash-chained audit trail) into one fine-grained, auditable access
// control layer.
//
// # What is new here
//
//   - Policy / PolicyRule / Engine.Evaluate (types.go, engine.go):
//     attribute-based access policies evaluated over a Request's actor
//     role, resource tenant/case/jurisdiction, action, and
//     time-of-day -- a real Evaluate(ctx, Request) (Decision, error)
//     engine, not a stub. This extends identity's static
//     role->permission matrix; callers are expected to have already
//     checked identity.HasPermission for the coarse action, then call
//     Evaluate for the finer-grained layer.
//   - CaseGrant (types.go): per-case access grants -- sharing a case
//     with a specific non-tenant-wide actor (e.g. an external
//     reviewer), or restricting a role's default access on one
//     sensitive case. Composes with packages/caselifecycle by CaseID
//     without duplicating Case itself.
//   - Grant / Engine.Elevate (types.go, engine.go): time-bound,
//     just-in-time elevated access with a mandatory expiry, rejected
//     at evaluation time once expired -- no background job required.
//   - Review / Engine.Attest (review_types.go, review.go): a
//     periodic/manual access-review workflow -- list active grants due
//     for review, and attest (approve or immediately revoke) a
//     specific grant or elevation.
//   - ConflictRule / CheckConflict (review_types.go): segregation of
//     duties -- the actor who requested a grant cannot also attest it
//     (RuleRequesterCannotApprove), and a case's sole author cannot be
//     its approving reviewer (RuleApproverNotSoleAuthor). Enforced
//     automatically inside Attest.
//   - AuditSink.PrivilegedActivity (audit.go): surfaces
//     elevation/break-glass-style access specifically, by querying
//     packages/auditlog.Store for this package's own
//     "access_governance.elevate" action -- not a parallel log.
//   - Engine.Certify / ExportReport (certify.go): access certification
//     reports aggregating every CaseGrant, Grant, and Review recorded
//     in a Period, exportable as CSV or JSON, mirroring
//     packages/auditlog.Export and packages/dataresidency's export
//     conventions.
//   - TestPolicy (policytest.go): a policy-testing harness letting a
//     policy author dry-run a candidate Policy against a set of
//     representative Scenarios before activating it -- a real
//     evaluation over the same Policy.evaluate logic Engine.Evaluate
//     uses, not a fake.
//
// # What is explicitly reused, not duplicated
//
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate; Policy/Request add
//     attributes on top, they do not replace the permission matrix.
//   - packages/caselifecycle.Case is referenced by CaseID only;
//     CaseGrant does not fork a competing per-case access model.
//   - Time-bound, justification-required elevation (Grant, Elevate)
//     deliberately mirrors packages/keymanagement.BreakGlassGrant /
//     GrantBreakGlass's shape -- same mandatory expiry, same
//     non-blank-justification requirement, same "audited regardless of
//     outcome" discipline -- generalized from "emergency key access"
//     to "any governed Action". packages/keymanagement's break-glass
//     mechanism itself is untouched and remains the key-specific
//     entry point; this package does not fork a second, competing
//     break-glass implementation for keys.
//   - packages/signoff's explicit-acknowledgement pattern (a reviewer
//     records a decision with required notes for Rejected) is the
//     reference this package's Review/Attest workflow follows, not a
//     dependency -- accessgovernance's review subjects are grants, not
//     case sign-off status.
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to (via AuditSink); there is no second audit
//     table.
//
// See doc/access-governance.md for the full write-up.
package accessgovernance
