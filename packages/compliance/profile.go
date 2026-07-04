package compliance

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Profile is a per-deployment (tenant) selection of which
// Frameworks -- and, optionally, which specific Controls beyond a
// selected Framework's full set -- apply (task 7). Not every
// deployment needs every framework: an air-gapped, UAE-only
// deployment has no reason to carry FrameworkInternationalDataProtection
// obligations, while a tenant serving customers under an international
// data-protection regime opts that overlay in explicitly. Mirrors how
// packages/jurisdiction and packages/reasoningprofile scope behavior by
// deployment rather than assuming one global configuration.
type Profile struct {
	// TenantID is the tenant this profile belongs to. A tenant has at
	// most one Profile at a time -- SetProfile replaces the existing
	// one wholesale rather than merging.
	TenantID uuid.UUID `json:"tenant_id"`

	// Frameworks lists every Framework this deployment is evaluated
	// against. GapAnalysis only considers Controls whose Framework
	// appears here (or every catalogued Control, if Frameworks is
	// empty -- see ApplicableControls).
	Frameworks []Framework `json:"frameworks"`

	// ExcludedControlIDs optionally excludes specific catalogued
	// Controls even though their Framework is selected -- e.g. a
	// deployment that has no cross-border transfer capability at all
	// (a fully air-gapped installation) may exclude
	// CategoryCrossBorderTransfer controls despite otherwise following
	// FrameworkUAEDataProtection.
	ExcludedControlIDs []uuid.UUID `json:"excluded_control_ids,omitempty"`

	// SetBy is the identity.User who last set this profile.
	SetBy uuid.UUID `json:"set_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks p for structural well-formedness.
func (p *Profile) Validate() error {
	if p == nil {
		return ErrInvalidProfile
	}
	if p.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	for _, f := range p.Frameworks {
		if !f.IsValid() {
			return wrapf("Profile.Validate", ErrInvalidFramework)
		}
	}
	return nil
}

// includesFramework reports whether p selects framework. An empty
// Frameworks list means "every framework applies" (the permissive
// default a freshly-provisioned tenant with no profile yet set should
// not silently exempt itself from everything).
func (p *Profile) includesFramework(framework Framework) bool {
	if p == nil || len(p.Frameworks) == 0 {
		return true
	}
	for _, f := range p.Frameworks {
		if f == framework {
			return true
		}
	}
	return false
}

// excludesControl reports whether p explicitly excludes controlID.
func (p *Profile) excludesControl(controlID uuid.UUID) bool {
	if p == nil {
		return false
	}
	for _, id := range p.ExcludedControlIDs {
		if id == controlID {
			return true
		}
	}
	return false
}

// ApplicableControls filters catalogue down to the Controls that apply
// under p: every Control whose Framework is selected by p (or every
// Control, if p is nil or selects no Frameworks) and whose ID is not
// in p's ExcludedControlIDs. This is the single function GapAnalysis
// and the dashboard both call to scope a report to what a specific
// deployment actually needs to satisfy, rather than every control this
// platform has ever catalogued globally.
func ApplicableControls(catalogue []Control, profile *Profile) []Control {
	out := make([]Control, 0, len(catalogue))
	for _, c := range catalogue {
		if !profile.includesFramework(c.Framework) {
			continue
		}
		if profile.excludesControl(c.ID) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// ProfileRepository persists Profile records, one per tenant.
type ProfileRepository interface {
	Set(ctx context.Context, tenantID uuid.UUID, p *Profile) error
	Get(ctx context.Context, tenantID uuid.UUID) (*Profile, error)
}

// SetProfile creates or replaces tenantID's Profile (task 7),
// requiring managePermission and tenant match. Every call is recorded
// via AuditSink regardless of outcome.
func (e *Engine) SetProfile(ctx context.Context, tenantID uuid.UUID, profile Profile) (Profile, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordProfileSet(ctx, tenantID, actorFromCtx(ctx), profile, err)
		}
		return Profile{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordProfileSet(ctx, tenantID, user.ID, profile, err)
		}
		return Profile{}, err
	}

	profile.TenantID = tenantID
	profile.SetBy = user.ID
	now := e.now()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now

	if err := profile.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordProfileSet(ctx, tenantID, user.ID, profile, err)
		}
		return Profile{}, err
	}
	if err := e.profiles.Set(ctx, tenantID, &profile); err != nil {
		wrapped := wrapf("SetProfile", err)
		if e.audit != nil {
			_, _ = e.audit.RecordProfileSet(ctx, tenantID, user.ID, profile, wrapped)
		}
		return Profile{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordProfileSet(ctx, tenantID, user.ID, profile, nil)
	}
	return profile, nil
}

// GetProfile returns tenantID's current Profile, requiring
// viewPermission and tenant match. Returns ErrProfileNotFound if none
// has been set yet.
func (e *Engine) GetProfile(ctx context.Context, tenantID uuid.UUID) (Profile, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return Profile{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Profile{}, err
	}
	profile, err := e.profiles.Get(ctx, tenantID)
	if err != nil {
		return Profile{}, err
	}
	return *profile, nil
}
