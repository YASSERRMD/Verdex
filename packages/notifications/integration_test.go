package notifications_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/notifications"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

// fakeCaseVersionReader is an in-memory signoff.CaseVersionReader
// stub, mirroring packages/signoff/helpers_test.go's fixture of the
// same name — this test drives the *real* packages/signoff.Service
// (not a mock of it), so it needs the same collaborator that
// package's own tests use.
type fakeCaseVersionReader struct {
	mu       sync.Mutex
	versions map[uuid.UUID]int
}

func newFakeCaseVersionReader() *fakeCaseVersionReader {
	return &fakeCaseVersionReader{versions: make(map[uuid.UUID]int)}
}

func (f *fakeCaseVersionReader) set(caseID uuid.UUID, version int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.versions[caseID] = version
}

func (f *fakeCaseVersionReader) CaseVersion(_ context.Context, _, caseID uuid.UUID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.versions[caseID]
	if !ok {
		return 1, nil
	}
	return v, nil
}

var _ signoff.CaseVersionReader = (*fakeCaseVersionReader)(nil)

// TestSignoffIntegration_ReReviewFiresRealNotification drives the real
// packages/signoff.Service end to end — Approve, then simulate a case
// content change, then ReReviewOnCaseUpdate — with a
// notifications.SignoffNotificationSink as its NotificationSink, and
// asserts a genuine Notification lands in the resolved judge's inbox.
// This is the integration-style proof task 3 asks for: no mock of
// packages/signoff, only its own InMemoryRepository and a
// CaseVersionReader fixture identical to packages/signoff's own tests.
func TestSignoffIntegration_ReReviewFiresRealNotification(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	judge := &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "judge@example.test",
		Name:     "Judge",
		Roles:    []identity.Role{identity.RoleJudge},
		Status:   identity.UserStatusActive,
	}
	ctx := identity.WithUser(context.Background(), judge)

	notifSvc := newTestService(t)
	resolver := func(_ context.Context, _, _ uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{judge.ID}, nil
	}
	sink := notifications.NewSignoffNotificationSink(notifSvc, resolver)

	signoffRepo := signoff.NewInMemoryRepository()
	reader := newFakeCaseVersionReader()
	reader.set(caseID, 1)

	signoffSvc, err := signoff.NewService(signoffRepo, reader, sink)
	if err != nil {
		t.Fatalf("signoff.NewService: %v", err)
	}

	if _, err := signoffSvc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Before the case changes, no pending-signoff notification should
	// exist yet — the case was just approved.
	before, err := notifSvc.List(ctxWithUser(judge), tenantID, judge.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List (before): %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("List (before): expected 0 notifications before re-review, got %d", len(before))
	}

	// Simulate the case's content changing after approval.
	reader.set(caseID, 2)

	rec, reverted, err := signoffSvc.ReReviewOnCaseUpdate(ctx, tenantID, caseID)
	if err != nil {
		t.Fatalf("ReReviewOnCaseUpdate: %v", err)
	}
	if !reverted {
		t.Fatal("ReReviewOnCaseUpdate: expected the approval to be reverted")
	}
	if rec.Status.String() != "pending" {
		t.Fatalf("ReReviewOnCaseUpdate: expected status pending, got %v", rec.Status)
	}

	after, err := notifSvc.List(ctxWithUser(judge), tenantID, judge.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List (after): %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("List (after): expected exactly 1 real notification from the signoff re-review, got %d", len(after))
	}
	if after[0].Kind != notifications.KindPendingSignoff {
		t.Fatalf("List (after): expected Kind %q, got %q", notifications.KindPendingSignoff, after[0].Kind)
	}
	if after[0].CaseID == nil || *after[0].CaseID != caseID {
		t.Fatalf("List (after): expected CaseID %s on the notification", caseID)
	}

	count, err := notifSvc.UnreadCount(ctxWithUser(judge), tenantID, judge.ID)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("UnreadCount: expected 1, got %d", count)
	}
}
