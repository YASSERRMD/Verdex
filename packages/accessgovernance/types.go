package accessgovernance

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// Action is the fine-grained operation a Request asks to perform.
// Distinct from identity.Permission: a Permission is a static
// role-level capability ("case:view"); an Action is the concrete verb
// a Policy evaluates against attributes (actor role, resource
// tenant/case/jurisdiction, time). Most Actions correspond 1:1 with an
// identity.Permission string so a Policy can be written directly
// against the existing RBAC vocabulary, but Action remains its own
// type so this package never has to import identity.Permission values
// it doesn't recognize.
type Action string

// DecisionEffect is the outcome of evaluating a Policy against a
// Request: either the request is allowed, or it is denied. There is no
// third "indeterminate" value -- Evaluate always resolves to exactly
// one of these two, defaulting to Deny when nothing in the policy
// grants the request (fail closed).
type DecisionEffect string

const (
	// EffectAllow means the request is permitted.
	EffectAllow DecisionEffect = "allow"

	// EffectDeny means the request is rejected. This is the default
	// when no rule in an evaluated Policy matches.
	EffectDeny DecisionEffect = "deny"
)

// IsValid reports whether e is a recognized DecisionEffect.
func (e DecisionEffect) IsValid() bool {
	return e == EffectAllow || e == EffectDeny
}

// String satisfies fmt.Stringer.
func (e DecisionEffect) String() string { return string(e) }

// Request is the attribute bundle Evaluate judges: who is asking, for
// what action, against which resource, and when. This is the "beyond
// identity's static role->permission matrix" surface task 1 asks
// for -- Role/Permission alone cannot express "only during business
// hours" or "only for this specific case" or "only until this grant
// expires", so Request carries the additional attributes a Policy's
// Rules can key on.
type Request struct {
	// ActorUserID identifies the user making the request. Required.
	ActorUserID uuid.UUID

	// ActorRoles are the roles held by ActorUserID at request time.
	// Populated by the caller (typically read straight off the
	// identity.User on ctx) rather than looked up by this package,
	// which has no dependency on an identity.Repository.
	ActorRoles []identity.Role

	// TenantID is the tenant the request is scoped to. Every Evaluate
	// call requires TenantID to match the ctx actor's own tenant (see
	// access.go) -- Request.TenantID cannot be used to reach into
	// another tenant's resources.
	TenantID uuid.UUID

	// CaseID optionally scopes the request to a single
	// packages/caselifecycle.Case. Zero (uuid.Nil) means the request
	// is not case-scoped (e.g. a tenant-wide administrative action).
	CaseID uuid.UUID

	// JurisdictionID optionally scopes the request to a
	// packages/jurisdiction jurisdiction, stored as a plain string (the
	// same "reference only, no hard dependency" convention
	// packages/caselifecycle.Case.CategoryID uses) so this package does
	// not take on packages/jurisdiction as a module dependency.
	JurisdictionID string

	// Action is the operation being requested.
	Action Action

	// Now is the time the request is evaluated at. Zero means
	// Evaluate substitutes time.Now().UTC() -- tests set this
	// explicitly to make expiry/time-of-day rules deterministic.
	Now time.Time
}

// Validate checks that r has the minimum fields Evaluate needs to
// reason about: a non-nil ActorUserID, TenantID, and non-empty Action.
func (r Request) Validate() error {
	if r.ActorUserID == uuid.Nil {
		return wrapf("Request.Validate", ErrForbidden)
	}
	if r.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if strings.TrimSpace(string(r.Action)) == "" {
		return wrapf("Request.Validate", ErrInvalidPolicy)
	}
	return nil
}

// TimeWindow restricts a PolicyRule (or a Grant) to a specific
// time-of-day range, evaluated in UTC. A zero TimeWindow (both Start
// and End empty) means "no time-of-day restriction".
type TimeWindow struct {
	// StartHour and EndHour are hour-of-day bounds in [0,24), UTC.
	// StartHour <= EndHour is a same-day window (e.g. 9-17); StartHour
	// > EndHour wraps past midnight (e.g. 22-6 for an overnight
	// maintenance window). Both zero with Enabled=false means
	// unrestricted.
	StartHour int
	EndHour   int

	// Enabled gates whether this window is checked at all -- lets a
	// PolicyRule leave StartHour/EndHour at their zero value without
	// that being misread as "midnight to midnight only".
	Enabled bool
}

// Allows reports whether t's hour-of-day (UTC) falls inside w. A
// disabled window always allows.
func (w TimeWindow) Allows(t time.Time) bool {
	if !w.Enabled {
		return true
	}
	h := t.UTC().Hour()
	if w.StartHour <= w.EndHour {
		return h >= w.StartHour && h < w.EndHour
	}
	// Overnight window wrapping past midnight.
	return h >= w.StartHour || h < w.EndHour
}

// PolicyRule is a single attribute-matching clause within a Policy.
// All non-empty match fields are ANDed together; Evaluate stops at the
// first Rule (in order) that matches the Request and returns its
// Effect.
type PolicyRule struct {
	// Actions restricts this rule to the listed Actions. Empty means
	// "any action".
	Actions []Action

	// Roles restricts this rule to actors holding at least one of the
	// listed roles. Empty means "any role".
	Roles []identity.Role

	// Jurisdictions restricts this rule to requests whose
	// JurisdictionID is in this list. Empty means "any jurisdiction".
	Jurisdictions []string

	// RequireCaseScope, when true, only matches requests carrying a
	// non-nil CaseID -- lets a Policy express "this rule only applies
	// to case-scoped requests".
	RequireCaseScope bool

	// TimeWindow restricts this rule to the given time-of-day range.
	TimeWindow TimeWindow

	// Effect is the outcome applied when this Rule matches.
	Effect DecisionEffect
}

// matches reports whether req satisfies every non-empty constraint on
// r.
func (r PolicyRule) matches(req Request) bool {
	if len(r.Actions) > 0 && !containsAction(r.Actions, req.Action) {
		return false
	}
	if len(r.Roles) > 0 && !anyRoleMatches(r.Roles, req.ActorRoles) {
		return false
	}
	if len(r.Jurisdictions) > 0 && !containsString(r.Jurisdictions, req.JurisdictionID) {
		return false
	}
	if r.RequireCaseScope && req.CaseID == uuid.Nil {
		return false
	}
	if !r.TimeWindow.Allows(req.Now) {
		return false
	}
	return true
}

func containsAction(list []Action, a Action) bool {
	for _, x := range list {
		if x == a {
			return true
		}
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

func anyRoleMatches(allowed []identity.Role, actual []identity.Role) bool {
	for _, want := range allowed {
		for _, have := range actual {
			if want == have {
				return true
			}
		}
	}
	return false
}

// Policy is an attribute-based access policy (task 1): an ordered set
// of Rules evaluated against a Request's attributes (actor role,
// resource tenant/case/jurisdiction, action, time-of-day), extending
// identity's static role->permission matrix rather than replacing it.
// A Policy is evaluated in addition to, not instead of, the ordinary
// identity.HasPermission check -- see Evaluate in access.go.
type Policy struct {
	// ID uniquely identifies this policy.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this policy belongs to. Empty
	// (uuid.Nil) means a global/default policy usable as a fallback
	// across tenants -- reserved for future use; every Policy created
	// through NewPolicy in this phase carries a concrete TenantID.
	TenantID uuid.UUID `json:"tenant_id"`

	// Name is a short human-readable label.
	Name string `json:"name"`

	// Rules are evaluated in order; the first matching Rule's Effect
	// wins. If no Rule matches, Evaluate returns EffectDeny (fail
	// closed).
	Rules []PolicyRule `json:"rules"`

	// Active gates whether Evaluate considers this policy at all --
	// lets a policy author register a candidate Policy, dry-run it
	// via TestPolicy (task 8), and only later flip it to Active.
	Active bool `json:"active"`

	// CreatedBy is the identity.User who authored this policy.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks p for structural well-formedness.
func (p *Policy) Validate() error {
	if p == nil {
		return ErrNilPolicy
	}
	if p.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrInvalidPolicy
	}
	for _, r := range p.Rules {
		if !r.Effect.IsValid() {
			return ErrInvalidPolicy
		}
	}
	return nil
}

// Evaluate judges req against p's Rules in order, returning the first
// match's Effect, or EffectDeny if nothing matches or p is inactive.
// This is the pure, in-memory decision function; Engine.Evaluate
// (access.go) wraps it with authorization, per-case grants, JIT
// elevation, and audit recording.
func (p *Policy) evaluate(req Request) (DecisionEffect, *PolicyRule) {
	if p == nil || !p.Active {
		return EffectDeny, nil
	}
	for i := range p.Rules {
		if p.Rules[i].matches(req) {
			return p.Rules[i].Effect, &p.Rules[i]
		}
	}
	return EffectDeny, nil
}

// Decision is the result of Evaluate: whether the request is allowed,
// and enough context to explain why for audit and debugging purposes.
type Decision struct {
	// Effect is the resolved allow/deny outcome.
	Effect DecisionEffect `json:"effect"`

	// Reason is a short human-readable explanation (e.g. "matched
	// policy rule", "case grant permits action", "grant expired").
	Reason string `json:"reason"`

	// MatchedPolicyID is the Policy that produced this Decision, or
	// uuid.Nil if the decision came from a CaseGrant or Grant instead
	// of an attribute policy.
	MatchedPolicyID uuid.UUID `json:"matched_policy_id,omitempty"`

	// MatchedGrantID is the CaseGrant or Grant ID that produced this
	// Decision, or uuid.Nil if it came from a Policy.
	MatchedGrantID uuid.UUID `json:"matched_grant_id,omitempty"`

	// EvaluatedAt is when this decision was computed.
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// Allowed reports whether d represents an allow decision.
func (d Decision) Allowed() bool { return d.Effect == EffectAllow }

// CaseGrant is a per-case access grant (task 2): access to a specific
// case beyond (or restricting) the default tenant/role scope, e.g.
// sharing a case with an external reviewer who holds no ordinary role
// in the tenant, or narrowing what an otherwise-privileged role may do
// on one sensitive case. CaseGrant composes with
// packages/caselifecycle by CaseID -- this package does not duplicate
// Case itself, only the grant record layered on top of it.
type CaseGrant struct {
	// ID uniquely identifies this grant.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this grant belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// CaseID identifies the packages/caselifecycle.Case this grant
	// applies to.
	CaseID uuid.UUID `json:"case_id"`

	// GranteeUserID is the identity.User this grant is issued to.
	GranteeUserID uuid.UUID `json:"grantee_user_id"`

	// Permissions are the identity.Permission values this grant
	// confers on GranteeUserID for CaseID specifically, independent of
	// whatever role(s) GranteeUserID otherwise holds in the tenant.
	Permissions []identity.Permission `json:"permissions"`

	// Deny, when true, makes this an explicit restriction rather than
	// an additive grant: GranteeUserID is denied Permissions on CaseID
	// even if their role would otherwise allow it. Lets a Policy
	// express "restricting it beyond default role access" (task 2)
	// without a second type.
	Deny bool `json:"deny,omitempty"`

	// ExpiresAt is when this grant stops applying. Required
	// (non-zero) -- every CaseGrant is time-bound, mirroring
	// packages/keymanagement.BreakGlassGrant's mandatory expiry.
	ExpiresAt time.Time `json:"expires_at"`

	// GrantedBy is the identity.User who created this grant.
	GrantedBy uuid.UUID `json:"granted_by"`

	// GrantedAt is when this grant was created.
	GrantedAt time.Time `json:"granted_at"`

	// RevokedAt, if non-nil, is when this grant was explicitly revoked
	// before its natural expiry (see Review/Attest in review.go).
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// Validate checks g for structural well-formedness.
func (g *CaseGrant) Validate() error {
	if g == nil {
		return ErrInvalidGrant
	}
	if g.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if g.CaseID == uuid.Nil {
		return wrapf("CaseGrant.Validate", ErrInvalidGrant)
	}
	if g.GranteeUserID == uuid.Nil {
		return wrapf("CaseGrant.Validate", ErrInvalidGrant)
	}
	if g.ExpiresAt.IsZero() {
		return wrapf("CaseGrant.Validate", ErrInvalidGrant)
	}
	if !g.Deny && len(g.Permissions) == 0 {
		return wrapf("CaseGrant.Validate", ErrInvalidGrant)
	}
	return nil
}

// IsExpired reports whether g's window has elapsed as of now, or g
// has been explicitly revoked.
func (g *CaseGrant) IsExpired(now time.Time) bool {
	if g == nil {
		return true
	}
	if g.RevokedAt != nil {
		return true
	}
	return !now.Before(g.ExpiresAt)
}

// GrantsPermission reports whether g (assuming it applies to caseID
// and granteeUserID and is not expired at now) confers perm.
func (g *CaseGrant) grantsPermission(perm identity.Permission) bool {
	if g == nil || g.Deny {
		return false
	}
	for _, p := range g.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// deniesPermission reports whether g explicitly restricts perm.
func (g *CaseGrant) deniesPermission(perm identity.Permission) bool {
	if g == nil || !g.Deny {
		return false
	}
	if len(g.Permissions) == 0 {
		// A blanket deny grant with no explicit permission list
		// restricts every permission on the case.
		return true
	}
	for _, p := range g.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// Grant is a time-bound, just-in-time elevated access grant (task 3):
// conceptually similar to packages/keymanagement's BreakGlassGrant and
// packages/signoff's explicit-acknowledgement pattern, generalized
// beyond keys specifically to any Action this package evaluates. A
// Grant always carries a mandatory expiry and is rejected by Evaluate
// once expired -- checked at evaluation time, with no background
// job required.
type Grant struct {
	// ID uniquely identifies this elevation grant.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this grant belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// GranteeUserID is the identity.User this elevation authorizes.
	GranteeUserID uuid.UUID `json:"grantee_user_id"`

	// Action is the single Action this grant elevates GranteeUserID to
	// perform, independent of their ordinary role/permission set.
	Action Action `json:"action"`

	// CaseID optionally scopes the elevation to one case. Zero means
	// tenant-wide elevation for Action.
	CaseID uuid.UUID `json:"case_id,omitempty"`

	// Justification is the required, non-blank explanation for why
	// this emergency/JIT access is needed, mirroring
	// packages/keymanagement.BreakGlassGrant.Justification exactly.
	Justification string `json:"justification"`

	// GrantedAt is when this grant was created.
	GrantedAt time.Time `json:"granted_at"`

	// ExpiresAt is when this grant's time-bound window ends. Required
	// (non-zero); Evaluate rejects any use after this time with
	// ErrGrantExpired.
	ExpiresAt time.Time `json:"expires_at"`

	// RequestedBy is the identity.User who invoked Elevate --
	// typically equal to GranteeUserID (self-service JIT elevation),
	// but modeled distinctly so a future flow where one actor requests
	// elevation on behalf of another remains representable, and so
	// segregation-of-duties (task 5) has a clear "requester" to compare
	// an approver against.
	RequestedBy uuid.UUID `json:"requested_by"`

	// RevokedAt, if non-nil, is when this grant was explicitly revoked
	// (e.g. via Attest) before its natural expiry.
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// Validate checks g for structural well-formedness.
func (g *Grant) Validate() error {
	if g == nil {
		return ErrInvalidGrant
	}
	if g.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if g.GranteeUserID == uuid.Nil {
		return wrapf("Grant.Validate", ErrInvalidGrant)
	}
	if strings.TrimSpace(string(g.Action)) == "" {
		return wrapf("Grant.Validate", ErrInvalidGrant)
	}
	if g.ExpiresAt.IsZero() {
		return wrapf("Grant.Validate", ErrInvalidGrant)
	}
	if strings.TrimSpace(g.Justification) == "" {
		return ErrJustificationRequired
	}
	return nil
}

// IsExpired reports whether g's window has elapsed as of now, or g has
// been explicitly revoked.
func (g *Grant) IsExpired(now time.Time) bool {
	if g == nil {
		return true
	}
	if g.RevokedAt != nil {
		return true
	}
	return !now.Before(g.ExpiresAt)
}

// DefaultElevationTTL is the default validity window for a Grant
// produced by Elevate when called with a zero ttl, mirroring
// packages/keymanagement.DefaultBreakGlassTTL.
const DefaultElevationTTL = 1 * time.Hour
