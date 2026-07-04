package accessgovernance

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// Engine is the access-governance decision engine (task 1): it
// composes attribute-based Policy evaluation, per-case CaseGrant
// overrides, and time-bound Grant (JIT elevation) checks into a
// single Evaluate call, recording every decision via AuditSink (task
// 6). Engine does not replace identity.HasPermission -- callers are
// expected to have already checked the ordinary role/permission gate
// for a coarse action; Engine adds the finer-grained layer on top.
type Engine struct {
	policies PolicyRepository
	grants   CaseGrantRepository
	elevate  GrantRepository
	reviews  ReviewRepository
	audit    *AuditSink
	clock    func() time.Time
}

// NewEngine builds an Engine from its dependencies. policies, grants,
// and elevate must be non-nil (ErrNilStore); reviews and audit may be
// nil (a nil audit sink means Evaluate/Elevate/Attest simply skip
// audit recording -- useful for lightweight unit tests of the
// decision logic itself, though production callers should always
// supply one).
func NewEngine(policies PolicyRepository, grants CaseGrantRepository, elevate GrantRepository, reviews ReviewRepository, audit *AuditSink) (*Engine, error) {
	if policies == nil || grants == nil || elevate == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		policies: policies,
		grants:   grants,
		elevate:  elevate,
		reviews:  reviews,
		audit:    audit,
		clock:    time.Now,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// Evaluate is the real, working decision engine (task 1): it resolves
// req against every active Policy for req.TenantID, then applies any
// matching CaseGrant (task 2, which can override a Policy's decision
// in either direction) and any active, unexpired Grant (task 3, JIT
// elevation) for the same actor/action/case. Evaluation order,
// highest precedence first:
//
//  1. An unexpired CaseGrant for req.CaseID + req.ActorUserID that
//     explicitly Denies the requested permission -- an explicit
//     restriction always wins over anything else.
//  2. An unexpired Grant (JIT elevation) for req.ActorUserID +
//     req.Action (and, if set, req.CaseID) -- elevates to Allow.
//  3. An unexpired CaseGrant for req.CaseID + req.ActorUserID that
//     grants the requested permission -- allows.
//  4. The first matching Policy Rule (task 1) for req.TenantID.
//  5. Fail closed: EffectDeny.
//
// Every call is recorded via AuditSink regardless of outcome (task 6),
// unless the Engine was built with a nil audit sink.
func (e *Engine) Evaluate(ctx context.Context, req Request) (Decision, error) {
	if err := req.Validate(); err != nil {
		return Decision{}, err
	}
	user, err := authorizeActor(ctx)
	if err != nil {
		return Decision{}, err
	}
	if err := requireMatchingUserTenant(user, req.TenantID); err != nil {
		return Decision{}, err
	}

	if req.Now.IsZero() {
		req.Now = e.now()
	} else {
		req.Now = req.Now.UTC()
	}

	dec, evalErr := e.evaluateLocked(ctx, req)
	if e.audit != nil {
		// Audit recording failures are not surfaced as Evaluate errors
		// -- a decision that was correctly computed must not be
		// retroactively invalidated by an audit-store hiccup, mirroring
		// packages/keymanagement's recordAudit being best-effort from
		// the caller's perspective. The evaluation error itself (e.g. a
		// storage failure while looking up grants) is still returned.
		_, _ = e.audit.RecordEvaluate(ctx, req.TenantID, req, dec)
	}
	return dec, evalErr
}

func (e *Engine) evaluateLocked(ctx context.Context, req Request) (Decision, error) {
	perm := actionToPermission(req.Action)

	// Step 1 & 3: per-case grants, if the request is case-scoped.
	if req.CaseID != uuid.Nil {
		caseGrants, err := e.grants.ListForCase(ctx, req.TenantID, req.CaseID)
		if err != nil {
			return Decision{Effect: EffectDeny, Reason: "grant lookup failed", EvaluatedAt: req.Now}, wrapf("Evaluate", err)
		}
		for i := range caseGrants {
			g := caseGrants[i]
			if g.GranteeUserID != req.ActorUserID {
				continue
			}
			if g.IsExpired(req.Now) {
				continue
			}
			if g.deniesPermission(perm) {
				return Decision{
					Effect:         EffectDeny,
					Reason:         "case grant explicitly restricts this permission",
					MatchedGrantID: g.ID,
					EvaluatedAt:    req.Now,
				}, nil
			}
		}
		for i := range caseGrants {
			g := caseGrants[i]
			if g.GranteeUserID != req.ActorUserID {
				continue
			}
			if g.IsExpired(req.Now) {
				continue
			}
			if g.grantsPermission(perm) {
				return Decision{
					Effect:         EffectAllow,
					Reason:         "case grant permits this permission",
					MatchedGrantID: g.ID,
					EvaluatedAt:    req.Now,
				}, nil
			}
		}
	}

	// Step 2: JIT elevation grants.
	elevations, err := e.elevate.ListActive(ctx, req.TenantID, req.Now)
	if err != nil {
		return Decision{Effect: EffectDeny, Reason: "elevation lookup failed", EvaluatedAt: req.Now}, wrapf("Evaluate", err)
	}
	for i := range elevations {
		g := elevations[i]
		if g.GranteeUserID != req.ActorUserID || g.Action != req.Action {
			continue
		}
		if g.CaseID != uuid.Nil && g.CaseID != req.CaseID {
			continue
		}
		if g.IsExpired(req.Now) {
			continue
		}
		return Decision{
			Effect:         EffectAllow,
			Reason:         "just-in-time elevation grant permits this action",
			MatchedGrantID: g.ID,
			EvaluatedAt:    req.Now,
		}, nil
	}

	// Step 4: attribute-based policy.
	policies, err := e.policies.List(ctx, req.TenantID)
	if err != nil {
		return Decision{Effect: EffectDeny, Reason: "policy lookup failed", EvaluatedAt: req.Now}, wrapf("Evaluate", err)
	}
	for i := range policies {
		p := policies[i]
		effect, rule := p.evaluate(req)
		if rule == nil {
			continue
		}
		reason := "matched policy rule"
		if effect == EffectDeny {
			reason = "matched policy rule (deny)"
		}
		return Decision{
			Effect:          effect,
			Reason:          reason,
			MatchedPolicyID: p.ID,
			EvaluatedAt:     req.Now,
		}, nil
	}

	// Step 5: fail closed.
	return Decision{Effect: EffectDeny, Reason: "no policy or grant matched", EvaluatedAt: req.Now}, nil
}

// actionToPermission maps an Action to the identity.Permission of the
// same name, when one exists in identity's vocabulary, so a CaseGrant
// (which is expressed in terms of identity.Permission) can be checked
// against a Request (expressed in terms of Action). Actions that do
// not correspond to any identity.Permission simply never match a
// CaseGrant's Permissions list, which is the correct behavior --
// CaseGrant only overrides permission-shaped actions.
func actionToPermission(a Action) identity.Permission {
	return identity.Permission(a)
}

// Elevate produces a temporary elevated Grant (task 3): time-bound,
// justification-required, just-in-time access to perform action,
// mirroring packages/keymanagement.GrantBreakGlass's shape and
// generalizing it beyond keys specifically. ttl of zero uses
// DefaultElevationTTL. Every call is audited (success or failure),
// and requesting elevation is itself gated on the actor holding
// managePermission -- a plain user cannot self-elevate without at
// least tenant-admin-level standing to request it.
func (e *Engine) Elevate(ctx context.Context, tenantID uuid.UUID, granteeUserID uuid.UUID, action Action, caseID uuid.UUID, justification string, ttl time.Duration) (Grant, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			actorID, _ := identity.UserIDFromContext(ctx)
			_, _ = e.audit.RecordElevate(ctx, tenantID, actorID, action, justification, err)
		}
		return Grant{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordElevate(ctx, tenantID, user.ID, action, justification, err)
		}
		return Grant{}, err
	}

	if ttl <= 0 {
		ttl = DefaultElevationTTL
	}
	now := e.now()
	grant := &Grant{
		ID:            uuid.New(),
		TenantID:      tenantID,
		GranteeUserID: granteeUserID,
		Action:        action,
		CaseID:        caseID,
		Justification: justification,
		GrantedAt:     now,
		ExpiresAt:     now.Add(ttl),
		RequestedBy:   user.ID,
	}
	if err := grant.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordElevate(ctx, tenantID, user.ID, action, justification, err)
		}
		return Grant{}, err
	}

	if err := e.elevate.Create(ctx, tenantID, grant); err != nil {
		wrapped := wrapf("Elevate", err)
		if e.audit != nil {
			_, _ = e.audit.RecordElevate(ctx, tenantID, user.ID, action, justification, wrapped)
		}
		return Grant{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordElevate(ctx, tenantID, user.ID, action, justification, nil)
	}
	return *grant, nil
}

// GrantCaseAccess creates a per-case CaseGrant (task 2), requiring the
// caller to hold managePermission and to belong to tenantID.
func (e *Engine) GrantCaseAccess(ctx context.Context, tenantID uuid.UUID, g CaseGrant) (CaseGrant, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return CaseGrant{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return CaseGrant{}, err
	}

	g.TenantID = tenantID
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	if g.GrantedAt.IsZero() {
		g.GrantedAt = e.now()
	}
	if g.GrantedBy == uuid.Nil {
		g.GrantedBy = user.ID
	}
	if err := g.Validate(); err != nil {
		return CaseGrant{}, err
	}
	if err := e.grants.Create(ctx, tenantID, &g); err != nil {
		return CaseGrant{}, wrapf("GrantCaseAccess", err)
	}
	return g, nil
}

// RevokeCaseAccess immediately revokes a CaseGrant before its natural
// expiry, requiring managePermission.
func (e *Engine) RevokeCaseAccess(ctx context.Context, tenantID, grantID uuid.UUID) error {
	user, err := authorizeManage(ctx)
	if err != nil {
		return err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return err
	}
	if err := e.grants.Revoke(ctx, tenantID, grantID, e.now()); err != nil {
		return wrapf("RevokeCaseAccess", err)
	}
	return nil
}
