package securitytesting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// This file is task 5's authorization-bypass suite: real, lightweight
// black-box calls against packages/identity.HasPermission and
// packages/accessgovernance.Engine.Evaluate (Phase 080), constructing
// wrong-tenant, expired-grant, and forged-role bypass attempts and
// asserting each is correctly rejected. Both packages are already
// direct dependencies of this module (accessgovernance's own footprint
// is as lean as compliance/threatmodel's -- see go.mod), so this suite
// exercises the real public API rather than a documented-only
// scenario, per this phase's brief.

// ScenarioWrongTenantHasPermissionNeverEscalates proves
// identity.HasPermission is a pure role->permission lookup with no
// notion of tenant at all -- so a caller cannot smuggle a
// cross-tenant escalation through HasPermission itself; the actual
// tenant check lives one layer up, in every package's own
// requireMatchingUserTenant (this package's own access.go included).
// This scenario documents and re-asserts that boundary: no role, real
// or forged, ever grants a permission HasPermission wasn't already
// wired to grant for that role in identity.PermissionMatrix.
func ScenarioWrongTenantHasPermissionNeverEscalates() Scenario {
	return NewScenarioFunc(
		"authz-bypass/hasPermission-ignores-tenant-forged-role",
		CategoryAuthzBypass,
		func(_ context.Context) (Result, error) {
			// RoleAdvocate never holds PermManageSecuritytesting in the
			// authoritative identity.PermissionMatrix (only RoleAdmin
			// does) -- if this ever returned true, some phase had wired
			// an unintended over-grant into the matrix.
			if identity.HasPermission(identity.RoleAdvocate, identity.PermManageSecuritytesting) {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  "identity.HasPermission(RoleAdvocate, PermManageSecuritytesting) = true, want false -- unintended privilege escalation",
				}, nil
			}
			// A forged/unknown role string must never resolve to any
			// permission -- HasPermission's unknown-role branch must fail
			// closed.
			if identity.HasPermission(identity.Role("super-admin-forged"), identity.PermManageSecuritytesting) {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  "identity.HasPermission(forged role, PermManageSecuritytesting) = true, want false -- unknown role did not fail closed",
				}, nil
			}
			return Result{Outcome: OutcomePassed, Detail: "HasPermission correctly denies both an unprivileged real role and a forged role"}, nil
		},
	)
}

// newAuthzBypassEngine builds a minimal accessgovernance.Engine wired
// to fresh in-memory repositories, for this file's Scenarios to drive
// constructed bypass attempts against.
func newAuthzBypassEngine() (*accessgovernance.Engine, error) {
	policies := accessgovernance.NewInMemoryPolicyRepository()
	caseGrants := accessgovernance.NewInMemoryCaseGrantRepository()
	grants := accessgovernance.NewInMemoryGrantRepository()
	return accessgovernance.NewEngine(policies, caseGrants, grants, nil, nil)
}

// ScenarioExpiredGrantStillDenied proves an expired
// accessgovernance.Grant (JIT elevation) is never honored by Evaluate
// -- inserting an already-expired Grant directly into the repository
// (bypassing Engine.Elevate's own validity checks, exactly what an
// attacker who compromised the grant store would attempt) and then
// confirming Evaluate still resolves to EffectDeny (fail closed) for
// the action the expired grant would have covered.
func ScenarioExpiredGrantStillDenied() Scenario {
	return NewScenarioFunc(
		"authz-bypass/expired-grant-still-denied",
		CategoryAuthzBypass,
		func(ctx context.Context) (Result, error) {
			tenantID := uuid.New()
			actorID := uuid.New()
			action := accessgovernance.Action("securitytesting:manage")

			policies := accessgovernance.NewInMemoryPolicyRepository()
			caseGrants := accessgovernance.NewInMemoryCaseGrantRepository()
			grants := accessgovernance.NewInMemoryGrantRepository()
			engine, err := accessgovernance.NewEngine(policies, caseGrants, grants, nil, nil)
			if err != nil {
				return Result{}, fmt.Errorf("newAuthzBypassEngine: %w", err)
			}

			now := time.Now().UTC()
			expired := &accessgovernance.Grant{
				ID:            uuid.New(),
				TenantID:      tenantID,
				GranteeUserID: actorID,
				Action:        action,
				Justification: "adversarial fixture: deliberately expired grant",
				GrantedAt:     now.Add(-2 * time.Hour),
				ExpiresAt:     now.Add(-1 * time.Hour), // expired one hour ago
				RequestedBy:   actorID,
			}
			if err := grants.Create(ctx, tenantID, expired); err != nil {
				return Result{}, fmt.Errorf("seed expired grant: %w", err)
			}

			actorCtx := identity.WithUser(ctx, &identity.User{
				ID:       actorID,
				TenantID: tenantID,
				Roles:    []identity.Role{identity.RoleAdvocate}, // holds no relevant Policy-level permission on its own
				Status:   identity.UserStatusActive,
			})

			decision, evalErr := engine.Evaluate(actorCtx, accessgovernance.Request{
				ActorUserID: actorID,
				ActorRoles:  []identity.Role{identity.RoleAdvocate},
				TenantID:    tenantID,
				Action:      action,
				Now:         now,
			})
			if evalErr != nil {
				return Result{}, fmt.Errorf("evaluate: %w", evalErr)
			}
			if decision.Effect == accessgovernance.EffectAllow {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  "Evaluate allowed an action covered only by an expired Grant -- expired-grant bypass succeeded",
					Evidence: map[string]string{
						"matched_grant_id": decision.MatchedGrantID.String(),
					},
				}, nil
			}
			return Result{Outcome: OutcomePassed, Detail: "Evaluate correctly denied: expired Grant was not honored, fail-closed to EffectDeny"}, nil
		},
	)
}

// ScenarioWrongTenantEvaluateRejected proves Evaluate refuses to
// resolve a Request scoped to a tenant other than the authenticated
// ctx actor's own tenant, returning ErrCrossTenantAccess rather than a
// Decision (of either effect) -- an actor authenticated against tenant
// A can never use Request.TenantID to probe or influence tenant B's
// policy evaluation, even indirectly via a deny/allow signal.
func ScenarioWrongTenantEvaluateRejected() Scenario {
	return NewScenarioFunc(
		"authz-bypass/wrong-tenant-evaluate-rejected",
		CategoryAuthzBypass,
		func(ctx context.Context) (Result, error) {
			engine, err := newAuthzBypassEngine()
			if err != nil {
				return Result{}, fmt.Errorf("newAuthzBypassEngine: %w", err)
			}

			tenantA := uuid.New()
			tenantB := uuid.New()
			actorID := uuid.New()

			// actorID is authenticated against tenant A...
			actorCtx := identity.WithUser(ctx, &identity.User{
				ID:       actorID,
				TenantID: tenantA,
				Roles:    []identity.Role{identity.RoleAdmin},
				Status:   identity.UserStatusActive,
			})

			// ...but the Request names tenant B as the scope, attempting
			// to reach across the tenant boundary.
			_, evalErr := engine.Evaluate(actorCtx, accessgovernance.Request{
				ActorUserID: actorID,
				ActorRoles:  []identity.Role{identity.RoleAdmin},
				TenantID:    tenantB,
				Action:      "securitytesting:manage",
				Now:         time.Now().UTC(),
			})

			if evalErr == nil {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  "Evaluate returned a Decision for a cross-tenant Request instead of rejecting it -- cross-tenant bypass succeeded",
				}, nil
			}
			if !errors.Is(evalErr, accessgovernance.ErrCrossTenantAccess) {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("Evaluate rejected the cross-tenant Request but with an unexpected error (%v), want ErrCrossTenantAccess", evalErr),
				}, nil
			}
			return Result{Outcome: OutcomePassed, Detail: "Evaluate correctly rejected a cross-tenant Request with ErrCrossTenantAccess"}, nil
		},
	)
}

// ScenarioNoPolicyOrGrantFailsClosed proves that with zero Policies
// and zero Grants on file for a tenant, Evaluate resolves to
// EffectDeny -- the "no rule matched" default must be deny, not allow,
// so a misconfigured or freshly-provisioned tenant with an empty
// policy set is maximally restricted rather than maximally permissive.
func ScenarioNoPolicyOrGrantFailsClosed() Scenario {
	return NewScenarioFunc(
		"authz-bypass/no-policy-or-grant-fails-closed",
		CategoryAuthzBypass,
		func(ctx context.Context) (Result, error) {
			engine, err := newAuthzBypassEngine()
			if err != nil {
				return Result{}, fmt.Errorf("newAuthzBypassEngine: %w", err)
			}

			tenantID := uuid.New()
			actorID := uuid.New()
			actorCtx := identity.WithUser(ctx, &identity.User{
				ID:       actorID,
				TenantID: tenantID,
				Roles:    []identity.Role{identity.RoleAdmin},
				Status:   identity.UserStatusActive,
			})

			decision, evalErr := engine.Evaluate(actorCtx, accessgovernance.Request{
				ActorUserID: actorID,
				ActorRoles:  []identity.Role{identity.RoleAdmin},
				TenantID:    tenantID,
				Action:      "some:never-configured-action",
				Now:         time.Now().UTC(),
			})
			if evalErr != nil {
				return Result{}, fmt.Errorf("evaluate: %w", evalErr)
			}
			if decision.Effect != accessgovernance.EffectDeny {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("Evaluate with no matching Policy/Grant resolved to %s, want EffectDeny (fail closed)", decision.Effect),
				}, nil
			}
			return Result{Outcome: OutcomePassed, Detail: "Evaluate correctly fails closed (EffectDeny) when no Policy or Grant matches"}, nil
		},
	)
}

// NewAuthzBypassSuite returns every Scenario in this file's fixed
// authorization-bypass suite.
func NewAuthzBypassSuite() []Scenario {
	return []Scenario{
		ScenarioWrongTenantHasPermissionNeverEscalates(),
		ScenarioExpiredGrantStillDenied(),
		ScenarioWrongTenantEvaluateRejected(),
		ScenarioNoPolicyOrGrantFailsClosed(),
	}
}
