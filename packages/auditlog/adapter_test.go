package auditlog_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

// TestSignoffAdapter_EndToEnd proves task 3/9: a real
// packages/signoff.AuditEntry (produced by the real signoff.Service,
// not a hand-built fixture standing in for it) flows into this
// package's durable, hash-chained Store and comes back out through
// Store.Query — i.e. this is not a parallel, unused system.
func TestSignoffAdapter_EndToEnd(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	reviewer := newTestUser(tenantID, identity.RoleJudge)

	signoffRepo := signoff.NewInMemoryRepository()
	caseReader := fixedVersionReader{version: 1}
	svc, err := signoff.NewService(signoffRepo, caseReader, nil)
	if err != nil {
		t.Fatalf("signoff.NewService: %v", err)
	}

	ctx := ctxWithUser(reviewer)
	_, err = svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Notes:           "reviewed and approved",
		Acknowledgement: signoff.AcknowledgementConfirmation,
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Pull the real audit trail signoff.Service just wrote.
	history, err := signoffRepo.ListAudit(ctx, tenantID, caseID)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("ListAudit: got %d entries, want 1", len(history))
	}

	store := newTestStore(t)
	sink, err := auditlog.NewSignoffAuditSink(store)
	if err != nil {
		t.Fatalf("NewSignoffAuditSink: %v", err)
	}

	projected, err := sink.RecordSignoffEntry(context.Background(), history[0])
	if err != nil {
		t.Fatalf("RecordSignoffEntry: %v", err)
	}
	if projected.Kind != auditlog.KindSignoff {
		t.Fatalf("projected.Kind = %v, want KindSignoff", projected.Kind)
	}
	if projected.CaseID != caseID {
		t.Fatalf("projected.CaseID = %v, want %v", projected.CaseID, caseID)
	}
	if projected.Actor != reviewer.ID.String() {
		t.Fatalf("projected.Actor = %q, want %q", projected.Actor, reviewer.ID.String())
	}
	if projected.Target != guardrail.SignoffApproved.String() {
		t.Fatalf("projected.Target = %q, want %q", projected.Target, guardrail.SignoffApproved.String())
	}

	// Now query it back through the real, access-controlled Store API,
	// proving the whole path is queryable end-to-end.
	auditorCtx := ctxWithUser(newTestUser(tenantID, identity.RoleAuditor))
	got, err := store.Query(auditorCtx, tenantID, auditlog.Filter{CaseID: caseID, Kinds: []auditlog.Kind{auditlog.KindSignoff}})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].ID != projected.ID {
		t.Fatalf("Query: got %+v, want the projected signoff event", got)
	}
}

func TestFromSignoffEntry_SystemReReviewHasNonBlankActor(t *testing.T) {
	entry := &signoff.AuditEntry{
		ID:          uuid.New(),
		CaseID:      uuid.New(),
		TenantID:    uuid.New(),
		FromStatus:  guardrail.SignoffApproved,
		ToStatus:    guardrail.SignoffPending,
		Actor:       uuid.Nil,
		Source:      signoff.DecisionSourceReReview,
		Notes:       "case metadata changed",
		CaseVersion: 2,
		OccurredAt:  time.Now().UTC(),
	}

	ev := auditlog.FromSignoffEntry(entry)
	if ev.Actor == "" {
		t.Fatalf("FromSignoffEntry produced a blank Actor for a system re-review entry")
	}
	if err := ev.Validate(); err != nil {
		t.Fatalf("projected re-review Event failed Validate: %v", err)
	}
}

// fixedVersionReader is a minimal signoff.CaseVersionReader stub.
type fixedVersionReader struct {
	version int
}

func (r fixedVersionReader) CaseVersion(_ context.Context, _, _ uuid.UUID) (int, error) {
	return r.version, nil
}
