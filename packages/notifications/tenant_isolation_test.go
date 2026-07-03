package notifications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/notifications"
)

// TestRepository_TenantIsolation_CrossTenantAccessBlocked proves that
// InMemoryRepository refuses to let one tenant's scope see or write
// another tenant's notifications, mirroring
// packages/caseversioning's tenant_isolation_test.go exactly for this
// package's Repository.
func TestRepository_TenantIsolation_CrossTenantAccessBlocked(t *testing.T) {
	repo := notifications.NewInMemoryRepository()

	tenantA := uuid.New()
	tenantB := uuid.New()
	recipientA := uuid.New()

	nA := &notifications.Notification{
		TenantID:    tenantA,
		RecipientID: recipientA,
		Kind:        notifications.KindMention,
		Title:       "tenant A's notification",
	}
	if err := repo.Create(context.Background(), tenantA, nA); err != nil {
		t.Fatalf("Create in tenant A: %v", err)
	}

	// Tenant B cannot read tenant A's notification even by guessing its ID.
	if _, err := repo.Get(context.Background(), tenantB, nA.ID); !errors.Is(err, notifications.ErrNotFound) {
		t.Fatalf("Get across tenants error = %v, want ErrNotFound", err)
	}

	// Listing tenant B's recipient must not surface tenant A's notification,
	// even for the same RecipientID value (recipients are also
	// tenant-scoped identities in this system).
	list, err := repo.ListForRecipient(context.Background(), tenantB, recipientA, notifications.Filter{})
	if err != nil {
		t.Fatalf("ListForRecipient: %v", err)
	}
	for _, n := range list {
		if n.ID == nA.ID {
			t.Fatalf("tenant A's notification leaked into tenant B's ListForRecipient result")
		}
	}

	// MarkRead across tenants must fail.
	if err := repo.MarkRead(context.Background(), tenantB, recipientA, nA.ID); !errors.Is(err, notifications.ErrNotFound) {
		t.Fatalf("cross-tenant MarkRead error = %v, want ErrNotFound", err)
	}

	// UnreadCount for tenant B must not count tenant A's notification.
	count, err := repo.UnreadCount(context.Background(), tenantB, recipientA)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("UnreadCount across tenants = %d, want 0", count)
	}

	// Directly at the repository layer, an explicit tenant mismatch on
	// the Notification's own TenantID must be refused.
	mismatched := &notifications.Notification{
		TenantID:    tenantA,
		RecipientID: recipientA,
		Kind:        notifications.KindMention,
	}
	if err := repo.Create(context.Background(), tenantB, mismatched); !errors.Is(err, notifications.ErrCrossTenantAccess) {
		t.Fatalf("repo.Create tenant mismatch error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestService_TenantIsolation_ListScopedToTenant proves that
// Service.List, called with the right actor but the wrong tenant
// scope, cannot see another tenant's notifications for what is
// otherwise the "same" recipient ID.
func TestService_TenantIsolation_ListScopedToTenant(t *testing.T) {
	svc := newTestService(t)

	tenantA := uuid.New()
	tenantB := uuid.New()
	user := newTestUser(tenantA)
	// Same user ID, but this test asks the Service to look inside
	// tenant B's data for it — must come back empty.
	user.TenantID = tenantA

	if _, err := svc.Notify(context.Background(), notifications.NotifyInput{
		TenantID:    tenantA,
		RecipientID: user.ID,
		Kind:        notifications.KindMention,
		Title:       "tenant A only",
	}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	list, err := svc.List(ctxWithUser(user), tenantB, user.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List scoped to tenant B: expected 0 results, got %d", len(list))
	}
}

// TestPreferenceRepository_TenantIsolation proves that
// InMemoryPreferenceRepository scopes preferences by tenant, mirroring
// the Repository tenant-isolation test above.
func TestPreferenceRepository_TenantIsolation(t *testing.T) {
	repo := notifications.NewInMemoryPreferenceRepository()

	tenantA := uuid.New()
	tenantB := uuid.New()
	userID := uuid.New()

	pref := &notifications.Preference{
		TenantID: tenantA,
		UserID:   userID,
		Kind:     notifications.KindMention,
		Enabled:  false,
	}
	if err := repo.Upsert(context.Background(), tenantA, pref); err != nil {
		t.Fatalf("Upsert in tenant A: %v", err)
	}

	// The same (userID, kind) pair under tenant B must not see tenant
	// A's opt-out.
	if _, err := repo.Get(context.Background(), tenantB, userID, notifications.KindMention); !errors.Is(err, notifications.ErrNotFound) {
		t.Fatalf("Get across tenants error = %v, want ErrNotFound", err)
	}

	mismatched := &notifications.Preference{
		TenantID: tenantA,
		UserID:   userID,
		Kind:     notifications.KindMention,
	}
	if err := repo.Upsert(context.Background(), tenantB, mismatched); !errors.Is(err, notifications.ErrCrossTenantAccess) {
		t.Fatalf("repo.Upsert tenant mismatch error = %v, want ErrCrossTenantAccess", err)
	}
}
