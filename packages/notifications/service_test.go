package notifications_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/notifications"
)

func TestNotify_PersistsAndIsListable(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)
	caseID := uuid.New()

	n, err := svc.Notify(ctxWithUser(recipient), notifications.NotifyInput{
		TenantID:    tenantID,
		RecipientID: recipient.ID,
		Kind:        notifications.KindMention,
		Title:       "You were mentioned",
		CaseID:      &caseID,
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if n == nil {
		t.Fatal("Notify: expected a persisted Notification, got nil")
	}
	if n.ID == uuid.Nil {
		t.Fatal("Notify: expected a generated ID")
	}
	if n.IsRead() {
		t.Fatal("Notify: expected a freshly created notification to be unread")
	}

	list, err := svc.List(ctxWithUser(recipient), tenantID, recipient.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: expected 1 notification, got %d", len(list))
	}
	if list[0].ID != n.ID {
		t.Fatalf("List: expected notification %s, got %s", n.ID, list[0].ID)
	}
}

func TestNotify_SuppressedKindIsNotStoredOrDelivered(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)

	// Opt out of KindMention entirely.
	if _, err := svc.SetPreference(ctxWithUser(recipient), tenantID, recipient.ID, notifications.KindMention, false, nil); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	n, err := svc.Notify(ctxWithUser(recipient), notifications.NotifyInput{
		TenantID:    tenantID,
		RecipientID: recipient.ID,
		Kind:        notifications.KindMention,
		Title:       "You were mentioned",
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if n != nil {
		t.Fatalf("Notify: expected suppressed Kind to return nil notification, got %+v", n)
	}

	list, err := svc.List(ctxWithUser(recipient), tenantID, recipient.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List: expected 0 notifications after suppression, got %d", len(list))
	}

	count, err := svc.UnreadCount(ctxWithUser(recipient), tenantID, recipient.ID)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("UnreadCount: expected 0, got %d", count)
	}
}

func TestNotify_OtherKindsStillDeliveredWhenOneIsSuppressed(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)

	if _, err := svc.SetPreference(ctxWithUser(recipient), tenantID, recipient.ID, notifications.KindMention, false, nil); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	n, err := svc.Notify(ctxWithUser(recipient), notifications.NotifyInput{
		TenantID:    tenantID,
		RecipientID: recipient.ID,
		Kind:        notifications.KindPendingSignoff,
		Title:       "Case awaiting sign-off",
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if n == nil {
		t.Fatal("Notify: expected KindPendingSignoff to still be delivered")
	}
}

func TestUnreadCount_AndMarkRead(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)

	var lastID uuid.UUID
	for i := 0; i < 3; i++ {
		n, err := svc.Notify(ctxWithUser(recipient), notifications.NotifyInput{
			TenantID:    tenantID,
			RecipientID: recipient.ID,
			Kind:        notifications.KindTaskAssignment,
			Title:       "Assigned",
		})
		if err != nil {
			t.Fatalf("Notify: %v", err)
		}
		lastID = n.ID
	}

	count, err := svc.UnreadCount(ctxWithUser(recipient), tenantID, recipient.ID)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 3 {
		t.Fatalf("UnreadCount: expected 3, got %d", count)
	}

	if err := svc.MarkRead(ctxWithUser(recipient), tenantID, recipient.ID, lastID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	count, err = svc.UnreadCount(ctxWithUser(recipient), tenantID, recipient.ID)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("UnreadCount after MarkRead: expected 2, got %d", count)
	}

	// Idempotent: marking the same notification read again is not an error.
	if err := svc.MarkRead(ctxWithUser(recipient), tenantID, recipient.ID, lastID); err != nil {
		t.Fatalf("MarkRead (idempotent): %v", err)
	}
}

func TestMarkAllRead(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)

	for i := 0; i < 4; i++ {
		if _, err := svc.Notify(ctxWithUser(recipient), notifications.NotifyInput{
			TenantID:    tenantID,
			RecipientID: recipient.ID,
			Kind:        notifications.KindTaskAssignment,
			Title:       "Assigned",
		}); err != nil {
			t.Fatalf("Notify: %v", err)
		}
	}

	marked, err := svc.MarkAllRead(ctxWithUser(recipient), tenantID, recipient.ID)
	if err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	if marked != 4 {
		t.Fatalf("MarkAllRead: expected 4 marked, got %d", marked)
	}

	count, err := svc.UnreadCount(ctxWithUser(recipient), tenantID, recipient.ID)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("UnreadCount after MarkAllRead: expected 0, got %d", count)
	}
}

func TestList_UnreadOnlyFilter(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)

	n1, err := svc.Notify(ctxWithUser(recipient), notifications.NotifyInput{
		TenantID: tenantID, RecipientID: recipient.ID, Kind: notifications.KindMention, Title: "one",
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if _, err := svc.Notify(ctxWithUser(recipient), notifications.NotifyInput{
		TenantID: tenantID, RecipientID: recipient.ID, Kind: notifications.KindMention, Title: "two",
	}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if err := svc.MarkRead(ctxWithUser(recipient), tenantID, recipient.ID, n1.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	list, err := svc.List(ctxWithUser(recipient), tenantID, recipient.ID, notifications.Filter{UnreadOnly: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List(UnreadOnly): expected 1, got %d", len(list))
	}
	if list[0].Title != "two" {
		t.Fatalf("List(UnreadOnly): expected the still-unread notification, got %q", list[0].Title)
	}
}

func TestService_List_ForbidsReadingAnotherUsersInbox(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	owner := newTestUser(tenantID)
	intruder := newTestUser(tenantID)

	if _, err := svc.Notify(ctxWithUser(owner), notifications.NotifyInput{
		TenantID: tenantID, RecipientID: owner.ID, Kind: notifications.KindMention, Title: "private",
	}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if _, err := svc.List(ctxWithUser(intruder), tenantID, owner.ID, notifications.Filter{}); err != notifications.ErrForbidden {
		t.Fatalf("List: expected ErrForbidden for another user's inbox, got %v", err)
	}
}

func TestService_Notify_RejectsInvalidInput(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)

	_, err := svc.Notify(ctxWithUser(recipient), notifications.NotifyInput{
		TenantID:    tenantID,
		RecipientID: recipient.ID,
		Kind:        "not-a-real-kind",
		Title:       "bad",
	})
	if err != notifications.ErrInvalidKind {
		t.Fatalf("Notify: expected ErrInvalidKind, got %v", err)
	}
}
