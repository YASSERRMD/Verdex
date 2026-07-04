package localization_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/localization"
)

func TestNewEngineRejectsNilDependencies(t *testing.T) {
	cat := localization.NewSeededCatalog()
	prefs := localization.NewInMemoryPreferenceRepository()

	if _, err := localization.NewEngine(nil, prefs, nil); err != localization.ErrNilCatalog {
		t.Errorf("NewEngine(nil catalog) error = %v, want ErrNilCatalog", err)
	}
	if _, err := localization.NewEngine(cat, nil, nil); err != localization.ErrNilStore {
		t.Errorf("NewEngine(nil preferences) error = %v, want ErrNilStore", err)
	}
	if _, err := localization.NewEngine(cat, prefs, nil); err != nil {
		t.Errorf("NewEngine(nil audit) unexpected error: %v", err)
	}
}

func TestEngineSetAndGetPreferenceSelfService(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	user := judgeUser(tenantID)
	ctx := ctxWithUser(user)

	got, err := engine.SetPreference(ctx, tenantID, user.ID, localization.LocaleUrdu)
	if err != nil {
		t.Fatalf("SetPreference error: %v", err)
	}
	if got.Locale != localization.LocaleUrdu {
		t.Errorf("SetPreference result Locale = %q, want ur", got.Locale)
	}

	fetched, err := engine.GetPreference(ctx, tenantID, user.ID)
	if err != nil {
		t.Fatalf("GetPreference error: %v", err)
	}
	if fetched.Locale != localization.LocaleUrdu {
		t.Errorf("GetPreference Locale = %q, want ur", fetched.Locale)
	}
}

func TestEngineSetPreferenceRequiresAuthentication(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	_, err := engine.SetPreference(context.Background(), tenantID, uuid.New(), localization.LocaleEnglish)
	if err != localization.ErrUnauthenticated {
		t.Errorf("SetPreference(no actor) error = %v, want ErrUnauthenticated", err)
	}
}

func TestEngineSetPreferenceRejectsOtherUsersWithoutManagePermission(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	actor := judgeUser(tenantID) // holds no PermManageUsers
	ctx := ctxWithUser(actor)
	otherUserID := uuid.New()

	_, err := engine.SetPreference(ctx, tenantID, otherUserID, localization.LocaleTamil)
	if err != localization.ErrForbidden {
		t.Errorf("SetPreference(other user, no manage perm) error = %v, want ErrForbidden", err)
	}
}

func TestEngineAdminCanSetAnotherUsersPreference(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)
	ctx := ctxWithUser(admin)
	otherUserID := uuid.New()

	got, err := engine.SetPreference(ctx, tenantID, otherUserID, localization.LocaleTamil)
	if err != nil {
		t.Fatalf("SetPreference(admin, other user) error: %v", err)
	}
	if got.UserID != otherUserID {
		t.Errorf("SetPreference result UserID = %v, want %v", got.UserID, otherUserID)
	}
}

func TestEngineSetPreferenceRejectsCrossTenantActor(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	otherTenant := uuid.New()
	actor := judgeUser(otherTenant)
	ctx := ctxWithUser(actor)

	_, err := engine.SetPreference(ctx, tenantID, actor.ID, localization.LocaleEnglish)
	if err != localization.ErrCrossTenantAccess {
		t.Errorf("SetPreference(cross-tenant actor) error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngineSetPreferenceRejectsInvalidLocale(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	user := judgeUser(tenantID)
	ctx := ctxWithUser(user)

	_, err := engine.SetPreference(ctx, tenantID, user.ID, localization.Locale(""))
	if err != localization.ErrInvalidLocale {
		t.Errorf("SetPreference(blank locale) error = %v, want ErrInvalidLocale", err)
	}
}

func TestEngineResolveLocaleDefaultsWhenNoPreference(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	userID := uuid.New()
	ctx := ctxWithUser(judgeUser(tenantID))

	got := engine.ResolveLocale(ctx, tenantID, userID, localization.LocaleEnglish)
	if got != localization.LocaleEnglish {
		t.Errorf("ResolveLocale(no preference) = %q, want default en", got)
	}
}

func TestEngineClearPreferenceRevertsToDefault(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	user := judgeUser(tenantID)
	ctx := ctxWithUser(user)

	if _, err := engine.SetPreference(ctx, tenantID, user.ID, localization.LocaleArabic); err != nil {
		t.Fatalf("SetPreference error: %v", err)
	}
	if err := engine.ClearPreference(ctx, tenantID, user.ID); err != nil {
		t.Fatalf("ClearPreference error: %v", err)
	}

	got := engine.ResolveLocale(ctx, tenantID, user.ID, localization.LocaleEnglish)
	if got != localization.LocaleEnglish {
		t.Errorf("ResolveLocale(after clear) = %q, want default en", got)
	}
}

func TestEngineSetPreferencePreservesCreatedAtAcrossUpdate(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	user := judgeUser(tenantID)
	ctx := ctxWithUser(user)

	first, err := engine.SetPreference(ctx, tenantID, user.ID, localization.LocaleArabic)
	if err != nil {
		t.Fatalf("first SetPreference error: %v", err)
	}
	second, err := engine.SetPreference(ctx, tenantID, user.ID, localization.LocaleTamil)
	if err != nil {
		t.Fatalf("second SetPreference error: %v", err)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("second.CreatedAt = %v, want unchanged from first %v", second.CreatedAt, first.CreatedAt)
	}
	if second.ID != first.ID {
		t.Errorf("second.ID = %v, want unchanged from first %v", second.ID, first.ID)
	}
}

// TestEngineSetPreferenceRecordsAudit asserts every SetPreference call
// -- success or failure -- is recorded via AuditSink, mirroring every
// other packages/* Engine's "audited regardless of outcome" discipline.
func TestEngineSetPreferenceRecordsAudit(t *testing.T) {
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	user := judgeUser(tenantID)
	ctx := ctxWithUser(user)

	if _, err := engine.SetPreference(ctx, tenantID, user.ID, localization.LocaleArabic); err != nil {
		t.Fatalf("SetPreference error: %v", err)
	}

	// Failure path: another judge (no manage permission) tries to set
	// a different user's preference.
	otherActor := judgeUser(tenantID)
	otherCtx := ctxWithUser(otherActor)
	if _, err := engine.SetPreference(otherCtx, tenantID, user.ID, localization.LocaleTamil); err != localization.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	// auditlog.Store.Query itself requires identity.PermAuditRead on
	// the querying actor (see packages/auditlog/query.go) -- a plain
	// judgeUser does not hold it, so use an admin actor here purely to
	// read back what was recorded, independent of which actor
	// performed the SetPreference calls above.
	auditorCtx := ctxWithUser(newTestUser(tenantID, identity.RoleAdmin))
	events, err := auditStore.Query(auditorCtx, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("auditStore.Query error: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("len(events) = %d, want at least 2 (success + denied)", len(events))
	}

	var sawSuccess, sawDenied bool
	for _, ev := range events {
		if ev.Outcome == "set" {
			sawSuccess = true
		}
		if ev.Outcome == "denied" {
			sawDenied = true
		}
	}
	if !sawSuccess {
		t.Errorf("no successful preference_set event recorded")
	}
	if !sawDenied {
		t.Errorf("no denied preference_set event recorded")
	}
}
