package localization

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Engine ties this package's Catalog, the durable PreferenceRepository,
// and AuditSink together behind the identity permission/tenant-scoping
// discipline every other packages/* Engine in this codebase follows
// (authenticate, check tenant match, check permission, mutate, audit
// regardless of outcome -- mirroring packages/compliance.Engine's shape
// closely).
//
// Engine is the natural place for a future HTTP handler layer to sit:
// SetPreference/GetPreference/ClearPreference are the three operations
// apps/web's locale switcher (task 6) needs from a backend, and
// Translate/MissingKeys expose the Catalog itself for a server-rendered
// page or an API response that needs translated strings.
type Engine struct {
	catalog     *Catalog
	preferences PreferenceRepository
	audit       *AuditSink
	clock       func() time.Time
}

// NewEngine builds an Engine from its dependencies. catalog and
// preferences must be non-nil (ErrNilCatalog / ErrNilStore); audit may
// be nil (a nil audit sink means preference changes simply skip audit
// recording -- useful for lightweight unit tests of the decision logic
// itself, though production callers should always supply one).
func NewEngine(catalog *Catalog, preferences PreferenceRepository, audit *AuditSink) (*Engine, error) {
	if catalog == nil {
		return nil, ErrNilCatalog
	}
	if preferences == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		catalog:     catalog,
		preferences: preferences,
		audit:       audit,
		clock:       time.Now,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// Catalog returns the Engine's underlying Catalog, so a caller can run
// Translate/MissingKeys/UntranslatedKeys/CoveragePercent directly
// without this package needing thin wrapper methods for every Catalog
// query.
func (e *Engine) Catalog() *Catalog {
	return e.catalog
}

// SetPreference sets targetUserID's locale preference within tenantID
// to locale, on behalf of the actor authenticated on ctx. The actor
// must either be targetUserID themselves or hold
// identity.PermManageUsers (see access.go's requireSelfOrManage). Every
// call is recorded via AuditSink regardless of outcome.
func (e *Engine) SetPreference(ctx context.Context, tenantID, targetUserID uuid.UUID, locale Locale) (Preference, error) {
	actor, err := authorizeActor(ctx)
	if err != nil {
		return Preference{}, err
	}
	if err := requireMatchingUserTenant(actor, tenantID); err != nil {
		return Preference{}, e.audited(ctx, tenantID, actor.ID, targetUserID, locale, err)
	}
	if err := requireSelfOrManage(actor, targetUserID); err != nil {
		return Preference{}, e.audited(ctx, tenantID, actor.ID, targetUserID, locale, err)
	}
	if !locale.IsValid() {
		return Preference{}, e.audited(ctx, tenantID, actor.ID, targetUserID, locale, ErrInvalidLocale)
	}

	// Preserve CreatedAt across an update by reading any existing
	// Preference first -- mirroring
	// packages/compliance.Engine.SetProfile's "CreatedAt.IsZero() ->
	// stamp, else keep" convention -- so Upsert's caller-supplied
	// timestamps are always authoritative and a repository never needs
	// its own now() logic.
	now := e.now()
	createdAt := now
	if existing, err := e.preferences.Get(ctx, tenantID, targetUserID); err == nil {
		createdAt = existing.CreatedAt
	}

	p := &Preference{
		TenantID:  tenantID,
		UserID:    targetUserID,
		Locale:    locale,
		CreatedAt: createdAt,
		UpdatedAt: now,
	}
	if err := p.Validate(); err != nil {
		return Preference{}, e.audited(ctx, tenantID, actor.ID, targetUserID, locale, err)
	}

	if err := e.preferences.Upsert(ctx, tenantID, p); err != nil {
		return Preference{}, e.audited(ctx, tenantID, actor.ID, targetUserID, locale, err)
	}

	if e.audit != nil {
		if _, err := e.audit.RecordPreferenceSet(ctx, tenantID, actor.ID, targetUserID, locale, nil); err != nil {
			return Preference{}, err
		}
	}
	return *p, nil
}

// audited records a failed SetPreference attempt (setErr is always
// non-nil here) via AuditSink if one is configured, then returns setErr
// unchanged, so every call site above can write
// `return zero, e.audited(...)` as a single expression, mirroring
// packages/compliance's "nil audit sink -- skip recording" convention.
func (e *Engine) audited(ctx context.Context, tenantID, actorID, targetUserID uuid.UUID, locale Locale, setErr error) error {
	if e.audit != nil {
		_, _ = e.audit.RecordPreferenceSet(ctx, tenantID, actorID, targetUserID, locale, setErr)
	}
	return setErr
}

// GetPreference returns targetUserID's locale preference within
// tenantID, on behalf of the actor authenticated on ctx. Any
// authenticated actor within the same tenant may read any user's
// locale preference (it is not sensitive data), mirroring how
// packages/notifications lets any tenant member read another's
// non-sensitive display preferences.
func (e *Engine) GetPreference(ctx context.Context, tenantID, targetUserID uuid.UUID) (Preference, error) {
	actor, err := authorizeActor(ctx)
	if err != nil {
		return Preference{}, err
	}
	if err := requireMatchingUserTenant(actor, tenantID); err != nil {
		return Preference{}, err
	}
	p, err := e.preferences.Get(ctx, tenantID, targetUserID)
	if err != nil {
		return Preference{}, err
	}
	return *p, nil
}

// ResolveLocale returns targetUserID's preferred Locale within
// tenantID, or defaultLocale if no Preference is on file (a brand-new
// user who has never chosen a locale) -- the convenience entry point a
// page-render path calls once per request rather than handling
// ErrPreferenceNotFound itself every time.
func (e *Engine) ResolveLocale(ctx context.Context, tenantID, targetUserID uuid.UUID, defaultLocale Locale) Locale {
	p, err := e.GetPreference(ctx, tenantID, targetUserID)
	if err != nil {
		return defaultLocale
	}
	return p.Locale
}

// ClearPreference removes targetUserID's locale preference within
// tenantID, reverting them to the platform default locale, on behalf
// of the actor authenticated on ctx (same authorization rule as
// SetPreference). Every call is recorded via AuditSink regardless of
// outcome.
func (e *Engine) ClearPreference(ctx context.Context, tenantID, targetUserID uuid.UUID) error {
	actor, err := authorizeActor(ctx)
	if err != nil {
		return err
	}
	if err := requireMatchingUserTenant(actor, tenantID); err != nil {
		return e.auditedDelete(ctx, tenantID, actor.ID, targetUserID, err)
	}
	if err := requireSelfOrManage(actor, targetUserID); err != nil {
		return e.auditedDelete(ctx, tenantID, actor.ID, targetUserID, err)
	}

	if err := e.preferences.Delete(ctx, tenantID, targetUserID); err != nil {
		return e.auditedDelete(ctx, tenantID, actor.ID, targetUserID, err)
	}

	if e.audit != nil {
		if _, err := e.audit.RecordPreferenceDelete(ctx, tenantID, actor.ID, targetUserID, nil); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) auditedDelete(ctx context.Context, tenantID, actorID, targetUserID uuid.UUID, deleteErr error) error {
	if e.audit != nil {
		_, _ = e.audit.RecordPreferenceDelete(ctx, tenantID, actorID, targetUserID, deleteErr)
	}
	return deleteErr
}
