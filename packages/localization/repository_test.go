package localization_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/localization"
)

func TestInMemoryPreferenceRepositoryRoundTrip(t *testing.T) {
	repo := localization.NewInMemoryPreferenceRepository()
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()

	if _, err := repo.Get(ctx, tenantID, userID); err != localization.ErrPreferenceNotFound {
		t.Errorf("Get(missing) error = %v, want ErrPreferenceNotFound", err)
	}

	p := &localization.Preference{TenantID: tenantID, UserID: userID, Locale: localization.LocaleUrdu}
	if err := repo.Upsert(ctx, tenantID, p); err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	if p.ID == uuid.Nil {
		t.Errorf("Upsert did not assign an ID")
	}

	got, err := repo.Get(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Locale != localization.LocaleUrdu {
		t.Errorf("Get.Locale = %q, want ur", got.Locale)
	}

	// Upserting again with a different locale for the same
	// (tenant, user) pair updates in place rather than creating a
	// second row, and preserves the original ID.
	firstID := p.ID
	p2 := &localization.Preference{TenantID: tenantID, UserID: userID, Locale: localization.LocaleTamil}
	if err := repo.Upsert(ctx, tenantID, p2); err != nil {
		t.Fatalf("second Upsert error: %v", err)
	}
	if p2.ID != firstID {
		t.Errorf("second Upsert ID = %v, want unchanged %v", p2.ID, firstID)
	}
	got2, err := repo.Get(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("Get after update error: %v", err)
	}
	if got2.Locale != localization.LocaleTamil {
		t.Errorf("Get after update Locale = %q, want ta", got2.Locale)
	}

	if err := repo.Delete(ctx, tenantID, userID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if _, err := repo.Get(ctx, tenantID, userID); err != localization.ErrPreferenceNotFound {
		t.Errorf("Get(after delete) error = %v, want ErrPreferenceNotFound", err)
	}
	if err := repo.Delete(ctx, tenantID, userID); err != localization.ErrPreferenceNotFound {
		t.Errorf("Delete(already deleted) error = %v, want ErrPreferenceNotFound", err)
	}
}

func TestInMemoryPreferenceRepositoryCrossTenantRejected(t *testing.T) {
	repo := localization.NewInMemoryPreferenceRepository()
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()
	userID := uuid.New()

	p := &localization.Preference{TenantID: tenantB, UserID: userID, Locale: localization.LocaleEnglish}
	if err := repo.Upsert(ctx, tenantA, p); err != localization.ErrCrossTenantAccess {
		t.Errorf("Upsert(mismatched tenant) error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestInMemoryPreferenceRepositoryIsolatesTenants(t *testing.T) {
	repo := localization.NewInMemoryPreferenceRepository()
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()
	userID := uuid.New() // same user ID under two different tenants is a distinct row

	if err := repo.Upsert(ctx, tenantA, &localization.Preference{TenantID: tenantA, UserID: userID, Locale: localization.LocaleArabic}); err != nil {
		t.Fatalf("Upsert(tenantA) error: %v", err)
	}
	if _, err := repo.Get(ctx, tenantB, userID); err != localization.ErrPreferenceNotFound {
		t.Errorf("Get(tenantB, same userID) error = %v, want ErrPreferenceNotFound", err)
	}
}
