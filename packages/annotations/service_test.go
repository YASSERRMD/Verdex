package annotations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/annotations"
	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestService_CreateThreadResolveRoundTrip(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	author := newTestUser(tenantID, identity.RoleClerk)
	replier := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(author)

	root, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "This finding looks unsupported.",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create root: %v", err)
	}
	if root.ID == uuid.Nil {
		t.Fatal("expected root ID to be generated")
	}
	if root.AuthorID != author.ID {
		t.Fatalf("AuthorID = %s, want %s", root.AuthorID, author.ID)
	}
	if root.Resolved {
		t.Fatal("expected new annotation to be unresolved")
	}

	replyCtx := ctxWithUser(replier)
	reply, err := svc.Create(replyCtx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "Agreed, flagging for review.",
		AnchorType: annotations.AnchorCase,
		ParentID:   &root.ID,
	})
	if err != nil {
		t.Fatalf("Create reply: %v", err)
	}

	thread, err := svc.Thread(ctx, tenantID, root.ID)
	if err != nil {
		t.Fatalf("Thread: %v", err)
	}
	if len(thread) != 2 {
		t.Fatalf("len(thread) = %d, want 2", len(thread))
	}
	if thread[0].ID != root.ID || thread[1].ID != reply.ID {
		t.Fatalf("thread order = %v, %v; want root then reply", thread[0].ID, thread[1].ID)
	}

	resolved, err := svc.Resolve(ctx, tenantID, root.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !resolved.Resolved {
		t.Fatal("expected Resolved to be true after Resolve")
	}
	if resolved.ResolvedBy == nil || *resolved.ResolvedBy != author.ID {
		t.Fatalf("ResolvedBy = %v, want %s", resolved.ResolvedBy, author.ID)
	}
	if resolved.ResolvedAt == nil {
		t.Fatal("expected ResolvedAt to be set")
	}

	if _, err := svc.Resolve(ctx, tenantID, root.ID); !errors.Is(err, annotations.ErrAlreadyResolved) {
		t.Fatalf("second Resolve error = %v, want ErrAlreadyResolved", err)
	}

	reopened, err := svc.Reopen(ctx, tenantID, root.ID)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if reopened.Resolved {
		t.Fatal("expected Resolved to be false after Reopen")
	}
	if reopened.ResolvedBy != nil || reopened.ResolvedAt != nil {
		t.Fatal("expected ResolvedBy/ResolvedAt to be cleared after Reopen")
	}

	if _, err := svc.Reopen(ctx, tenantID, root.ID); !errors.Is(err, annotations.ErrNotResolved) {
		t.Fatalf("second Reopen error = %v, want ErrNotResolved", err)
	}
}

func TestService_Create_RejectsReplyToReply(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	author := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(author)

	root, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "root",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create root: %v", err)
	}
	reply, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "reply",
		AnchorType: annotations.AnchorCase,
		ParentID:   &root.ID,
	})
	if err != nil {
		t.Fatalf("Create reply: %v", err)
	}

	_, err = svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "reply to reply",
		AnchorType: annotations.AnchorCase,
		ParentID:   &reply.ID,
	})
	if !errors.Is(err, annotations.ErrParentIsReply) {
		t.Fatalf("error = %v, want ErrParentIsReply", err)
	}
}

func TestService_Create_RejectsMissingParent(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	author := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(author)

	missing := uuid.New()
	_, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "orphan reply",
		AnchorType: annotations.AnchorCase,
		ParentID:   &missing,
	})
	if !errors.Is(err, annotations.ErrParentNotFound) {
		t.Fatalf("error = %v, want ErrParentNotFound", err)
	}
}

func TestService_UpdateBody_OnlyAuthorMayEdit(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	author := newTestUser(tenantID, identity.RoleClerk)
	other := newTestUser(tenantID, identity.RoleClerk)

	a, err := svc.Create(ctxWithUser(author), tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "original",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.UpdateBody(ctxWithUser(other), tenantID, a.ID, "hijacked"); !errors.Is(err, annotations.ErrNotAuthor) {
		t.Fatalf("error = %v, want ErrNotAuthor", err)
	}

	updated, err := svc.UpdateBody(ctxWithUser(author), tenantID, a.ID, "edited body")
	if err != nil {
		t.Fatalf("UpdateBody by author: %v", err)
	}
	if updated.Body != "edited body" {
		t.Fatalf("Body = %q, want %q", updated.Body, "edited body")
	}
}

func TestService_Delete_OnlyAuthorMayDelete(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	author := newTestUser(tenantID, identity.RoleClerk)
	other := newTestUser(tenantID, identity.RoleClerk)

	a, err := svc.Create(ctxWithUser(author), tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "to be deleted",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(ctxWithUser(other), tenantID, a.ID); !errors.Is(err, annotations.ErrNotAuthor) {
		t.Fatalf("error = %v, want ErrNotAuthor", err)
	}

	if err := svc.Delete(ctxWithUser(author), tenantID, a.ID); err != nil {
		t.Fatalf("Delete by author: %v", err)
	}

	if _, err := svc.Get(ctxWithUser(author), tenantID, a.ID); !errors.Is(err, annotations.ErrNotFound) {
		t.Fatalf("Get after delete error = %v, want ErrNotFound", err)
	}
}

func TestService_Delete_CascadesToReplies(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	author := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(author)

	root, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "root",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create root: %v", err)
	}
	reply, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "reply",
		AnchorType: annotations.AnchorCase,
		ParentID:   &root.ID,
	})
	if err != nil {
		t.Fatalf("Create reply: %v", err)
	}

	if err := svc.Delete(ctx, tenantID, root.ID); err != nil {
		t.Fatalf("Delete root: %v", err)
	}
	if _, err := svc.Get(ctx, tenantID, reply.ID); !errors.Is(err, annotations.ErrNotFound) {
		t.Fatalf("Get reply after root delete error = %v, want ErrNotFound", err)
	}
}

func TestService_Create_RequiresEditPermission(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	viewer := newTestUser(tenantID, identity.RoleAuditor) // view + audit, no edit

	_, err := svc.Create(ctxWithUser(viewer), tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "should be forbidden",
		AnchorType: annotations.AnchorCase,
	})
	if !errors.Is(err, annotations.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestService_Create_RequiresAuthentication(t *testing.T) {
	svc, c, tenantID := newTestService(t)

	_, err := svc.Create(context.TODO(), tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "no user on context",
		AnchorType: annotations.AnchorCase,
	})
	if !errors.Is(err, annotations.ErrUnauthenticated) {
		t.Fatalf("error = %v, want ErrUnauthenticated", err)
	}
}

func TestService_MentionSink_NotifiedOnCreateAndEdit(t *testing.T) {
	tenantID := uuid.New()
	caseRepo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, caseRepo, tenantID)

	sink := &recordingSink{}
	repo := annotations.NewInMemoryRepository()
	svc, err := annotations.NewService(repo, caseRepo, sink)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	author := newTestUser(tenantID, identity.RoleClerk)
	mentioned := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(author)

	a, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "cc @" + mentioned.ID.String(),
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(sink.received) != 1 {
		t.Fatalf("len(sink.received) = %d, want 1", len(sink.received))
	}
	if sink.received[0].MentionedUserID != mentioned.ID {
		t.Fatalf("MentionedUserID = %s, want %s", sink.received[0].MentionedUserID, mentioned.ID)
	}
	if sink.received[0].AnnotationID != a.ID {
		t.Fatalf("AnnotationID = %s, want %s", sink.received[0].AnnotationID, a.ID)
	}

	otherMentioned := newTestUser(tenantID, identity.RoleClerk)
	if _, err := svc.UpdateBody(ctx, tenantID, a.ID, "now cc @"+otherMentioned.ID.String()); err != nil {
		t.Fatalf("UpdateBody: %v", err)
	}
	if len(sink.received) != 2 {
		t.Fatalf("len(sink.received) after edit = %d, want 2", len(sink.received))
	}
	if sink.received[1].MentionedUserID != otherMentioned.ID {
		t.Fatalf("second MentionedUserID = %s, want %s", sink.received[1].MentionedUserID, otherMentioned.ID)
	}
}
