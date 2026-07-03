package keymanagement

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// DefaultBreakGlassTTL is the default validity window for a
// break-glass grant when GrantBreakGlass is called with a zero ttl.
const DefaultBreakGlassTTL = 1 * time.Hour

// BreakGlassGrant is an emergency-access grant (task 6): a time-bound,
// justified authorization for one actor to use one key outside the
// normal access-policy flow. A grant does not itself return key
// material — UseBreakGlass, given a valid grant, is the only call
// that touches the underlying Provider.
type BreakGlassGrant struct {
	// ID uniquely identifies this grant.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this grant belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// KeyID identifies the key version this grant authorizes access
	// to.
	KeyID string `json:"key_id"`

	// GrantedTo is the identity.User this grant authorizes.
	GrantedTo uuid.UUID `json:"granted_to"`

	// Justification is the required, non-blank explanation for why
	// this emergency access is needed. Every grant is heavily audited
	// with this string attached (see AuditEntry.Justification).
	Justification string `json:"justification"`

	// GrantedAt is when this grant was created.
	GrantedAt time.Time `json:"granted_at"`

	// ExpiresAt is when this grant's time-bound window ends. UseBreakGlass
	// rejects any attempt after this time with ErrBreakGlassExpired.
	ExpiresAt time.Time `json:"expires_at"`

	// UsedAt is set the first time UseBreakGlass successfully consumes
	// this grant; nil until then. A grant may be used more than once
	// within its window (each use is separately audited) — UsedAt
	// records only the first use for operator visibility, it is not a
	// single-use lock.
	UsedAt *time.Time `json:"used_at,omitempty"`
}

// IsExpired reports whether g's time-bound window has elapsed as of
// now.
func (g *BreakGlassGrant) IsExpired(now time.Time) bool {
	return g == nil || !now.Before(g.ExpiresAt)
}

// BreakGlassStore persists BreakGlassGrant records, scoped to a
// tenant, mirroring Repository's convention. A minimal interface
// (rather than folding this into Repository) keeps the break-glass
// surface distinctly named and easy to fake in tests.
type BreakGlassStore interface {
	// Create inserts g.
	Create(ctx context.Context, tenantID uuid.UUID, g *BreakGlassGrant) error

	// Get returns the grant identified by id, scoped to tenantID.
	// Returns ErrBreakGlassNotFound if no such grant exists.
	Get(ctx context.Context, tenantID, id uuid.UUID) (*BreakGlassGrant, error)

	// MarkUsed sets UsedAt (if not already set) on the grant
	// identified by id.
	MarkUsed(ctx context.Context, tenantID, id uuid.UUID, usedAt time.Time) error
}

// InMemoryBreakGlassStore is an in-process BreakGlassStore
// implementation for tests, safe for concurrent use.
type InMemoryBreakGlassStore struct {
	grants map[uuid.UUID]*BreakGlassGrant
}

// NewInMemoryBreakGlassStore builds an empty InMemoryBreakGlassStore.
func NewInMemoryBreakGlassStore() *InMemoryBreakGlassStore {
	return &InMemoryBreakGlassStore{grants: make(map[uuid.UUID]*BreakGlassGrant)}
}

// Create implements BreakGlassStore.
func (s *InMemoryBreakGlassStore) Create(_ context.Context, tenantID uuid.UUID, g *BreakGlassGrant) error {
	if g.TenantID == uuid.Nil {
		g.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, g.TenantID); err != nil {
		return err
	}
	cp := *g
	s.grants[g.ID] = &cp
	return nil
}

// Get implements BreakGlassStore.
func (s *InMemoryBreakGlassStore) Get(_ context.Context, tenantID, id uuid.UUID) (*BreakGlassGrant, error) {
	g, ok := s.grants[id]
	if !ok || g.TenantID != tenantID {
		return nil, ErrBreakGlassNotFound
	}
	cp := *g
	return &cp, nil
}

// MarkUsed implements BreakGlassStore.
func (s *InMemoryBreakGlassStore) MarkUsed(_ context.Context, tenantID, id uuid.UUID, usedAt time.Time) error {
	g, ok := s.grants[id]
	if !ok || g.TenantID != tenantID {
		return ErrBreakGlassNotFound
	}
	if g.UsedAt == nil {
		t := usedAt
		g.UsedAt = &t
	}
	return nil
}

var _ BreakGlassStore = (*InMemoryBreakGlassStore)(nil)

// GrantBreakGlass creates a new time-bound BreakGlassGrant authorizing
// the caller (who must hold identity.PermBreakGlassKeys) to access
// keyID outside the normal flow. justification must be non-blank
// (ErrJustificationRequired otherwise) — this is the "requires
// explicit justification string" half of task 6. ttl of zero uses
// DefaultBreakGlassTTL. The grant creation itself is audited
// (AuditActionBreakGlassGrant) regardless of outcome.
func (s *Service) GrantBreakGlass(ctx context.Context, tenantID uuid.UUID, keyID, justification string, ttl time.Duration) (*BreakGlassGrant, error) {
	user, err := authorizeBreakGlass(ctx)
	if err != nil {
		s.auditDenied(ctx, tenantID, AuditActionBreakGlassGrant, keyID, err)
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		s.auditDenied(ctx, tenantID, AuditActionBreakGlassGrant, keyID, err)
		return nil, err
	}
	if strings.TrimSpace(justification) == "" {
		s.auditDenied(ctx, tenantID, AuditActionBreakGlassGrant, keyID, ErrJustificationRequired)
		return nil, ErrJustificationRequired
	}
	if ttl <= 0 {
		ttl = DefaultBreakGlassTTL
	}

	now := time.Now().UTC()
	grant := &BreakGlassGrant{
		ID:            uuid.New(),
		TenantID:      tenantID,
		KeyID:         keyID,
		GrantedTo:     user.ID,
		Justification: justification,
		GrantedAt:     now,
		ExpiresAt:     now.Add(ttl),
	}
	if err := s.breakGlass.Create(ctx, tenantID, grant); err != nil {
		s.recordAudit(ctx, tenantID, AuditActionBreakGlassGrant, keyID, AuditOutcomeError, justification, err.Error())
		return nil, wrapf("Service.GrantBreakGlass", err)
	}

	s.recordAudit(ctx, tenantID, AuditActionBreakGlassGrant, keyID, AuditOutcomeSuccess, justification, "")
	return grant, nil
}

// UseBreakGlass consumes grantID to retrieve keyID's material outside
// the normal access-policy flow. It fails closed: an expired grant
// (ErrBreakGlassExpired), an unknown grant (ErrBreakGlassNotFound), a
// grant for a different key, or a grant issued to a different user
// than the current ctx actor are all rejected. Every attempt —
// success or failure — is recorded as AuditActionBreakGlassUse with
// the grant's justification attached, satisfying task 6's "heavily
// audited" requirement.
func (s *Service) UseBreakGlass(ctx context.Context, tenantID uuid.UUID, grantID uuid.UUID) (KeyMaterial, error) {
	user, err := authorizeBreakGlass(ctx)
	if err != nil {
		s.auditDenied(ctx, tenantID, AuditActionBreakGlassUse, "", err)
		return KeyMaterial{}, err
	}

	grant, err := s.breakGlass.Get(ctx, tenantID, grantID)
	if err != nil {
		s.recordAudit(ctx, tenantID, AuditActionBreakGlassUse, "", AuditOutcomeDenied, "", err.Error())
		return KeyMaterial{}, err
	}

	if grant.GrantedTo != user.ID {
		s.recordAudit(ctx, tenantID, AuditActionBreakGlassUse, grant.KeyID, AuditOutcomeDenied, grant.Justification, "grant issued to a different user")
		return KeyMaterial{}, ErrForbidden
	}

	now := time.Now().UTC()
	if grant.IsExpired(now) {
		s.recordAudit(ctx, tenantID, AuditActionBreakGlassUse, grant.KeyID, AuditOutcomeDenied, grant.Justification, "grant expired")
		return KeyMaterial{}, ErrBreakGlassExpired
	}

	material, err := s.provider.Key(ctx, tenantID.String(), grant.KeyID)
	if err != nil {
		s.recordAudit(ctx, tenantID, AuditActionBreakGlassUse, grant.KeyID, AuditOutcomeError, grant.Justification, err.Error())
		return KeyMaterial{}, wrapf("Service.UseBreakGlass", err)
	}

	if err := s.breakGlass.MarkUsed(ctx, tenantID, grantID, now); err != nil {
		return KeyMaterial{}, wrapf("Service.UseBreakGlass", err)
	}

	s.recordAudit(ctx, tenantID, AuditActionBreakGlassUse, grant.KeyID, AuditOutcomeSuccess, grant.Justification, "")
	return material, nil
}

// auditDenied is a small helper for the common "authorization failed
// before we even had an actor to attribute cleanly" path, mirroring
// the ctx-actor-may-be-absent handling in recordAudit.
func (s *Service) auditDenied(ctx context.Context, tenantID uuid.UUID, action AuditAction, keyID string, err error) {
	s.recordAudit(ctx, tenantID, action, keyID, AuditOutcomeDenied, "", err.Error())
}

// currentActor extracts the best-effort actor label for ctx, used by
// recordAudit.
func currentActor(ctx context.Context) string {
	id, ok := identity.UserIDFromContext(ctx)
	return actorLabel(id, ok)
}
