package casesearch_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/casesearch"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func newTestService(t *testing.T) (*casesearch.SavedSearchService, *casesearch.Engine) {
	t.Helper()
	cases := caselifecycle.NewInMemoryRepository()
	engine, err := casesearch.NewEngine(cases, func(context.Context, string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{}, nil
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	svc, err := casesearch.NewSavedSearchService(casesearch.NewInMemorySavedSearchRepository(), engine)
	if err != nil {
		t.Fatalf("NewSavedSearchService: %v", err)
	}
	return svc, engine
}

func TestSavedSearchService_SaveListRun_RoundTrips(t *testing.T) {
	svc, _ := newTestService(t)
	tenantID := uuid.New()
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	q := casesearch.NewQuery("breach of contract").WithMode(casesearch.ModeKeyword)
	saved, err := svc.Save(ctx, tenantID, "My Search", q)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ID == uuid.Nil {
		t.Fatal("expected a generated ID")
	}
	if saved.Query.Text != "breach of contract" {
		t.Fatalf("expected persisted Query.Text preserved, got %q", saved.Query.Text)
	}

	list, err := svc.List(ctx, tenantID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("expected saved search in List, got %+v", list)
	}

	results, err := svc.Run(ctx, tenantID, saved.ID)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if results.Mode != casesearch.ModeKeyword {
		t.Fatalf("expected Run to execute the persisted Query, got mode %q", results.Mode)
	}
}

func TestSavedSearchService_Delete_RemovesSearch(t *testing.T) {
	svc, _ := newTestService(t)
	tenantID := uuid.New()
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	saved, err := svc.Save(ctx, tenantID, "Temp", casesearch.NewQuery("x").WithMode(casesearch.ModeKeyword))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := svc.Delete(ctx, tenantID, saved.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := svc.Run(ctx, tenantID, saved.ID); !errors.Is(err, casesearch.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSavedSearchService_Save_RejectsBlankName(t *testing.T) {
	svc, _ := newTestService(t)
	tenantID := uuid.New()
	ctx := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	_, err := svc.Save(ctx, tenantID, "", casesearch.NewQuery("x").WithMode(casesearch.ModeKeyword))
	if !errors.Is(err, casesearch.ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
}

func TestSavedSearchService_OwnerScoping_CannotSeeAnotherUsersSearch(t *testing.T) {
	svc, _ := newTestService(t)
	tenantID := uuid.New()
	ownerA := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))
	ownerB := ctxWithUser(newTestUser(tenantID, identity.RoleJudge))

	saved, err := svc.Save(ownerA, tenantID, "Owner A's Search", casesearch.NewQuery("x").WithMode(casesearch.ModeKeyword))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	listB, err := svc.List(ownerB, tenantID)
	if err != nil {
		t.Fatalf("List (owner B): %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected owner B to see no saved searches, got %+v", listB)
	}

	if _, err := svc.Run(ownerB, tenantID, saved.ID); !errors.Is(err, casesearch.ErrNotFound) {
		t.Fatalf("expected owner B's Run to be rejected with ErrNotFound, got %v", err)
	}
	if err := svc.Delete(ownerB, tenantID, saved.ID); !errors.Is(err, casesearch.ErrNotFound) {
		t.Fatalf("expected owner B's Delete to be rejected with ErrNotFound, got %v", err)
	}
}

func TestSavedSearchService_TenantIsolation_BlocksCrossTenantAccess(t *testing.T) {
	svc, _ := newTestService(t)
	tenantA := uuid.New()
	tenantB := uuid.New()

	userA := newTestUser(tenantA, identity.RoleJudge)
	ctxA := ctxWithUser(userA)

	saved, err := svc.Save(ctxA, tenantA, "Tenant A's Search", casesearch.NewQuery("x").WithMode(casesearch.ModeKeyword))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Same user identity, but querying under tenant B's scope: the
	// repository must not find tenant A's saved search.
	if _, err := svc.Run(ctxA, tenantB, saved.ID); err == nil {
		t.Fatal("expected an error running a saved search under the wrong tenant scope")
	}
}

func TestInMemorySavedSearchRepository_Get_CrossTenant_ReturnsNotFound(t *testing.T) {
	repo := casesearch.NewInMemorySavedSearchRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	owner := uuid.New()

	s := &casesearch.SavedSearch{
		TenantID: tenantA,
		OwnerID:  owner,
		Name:     "A Search",
		Query:    casesearch.NewQuery("x").WithMode(casesearch.ModeKeyword),
	}
	if err := repo.Create(context.Background(), tenantA, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := repo.Get(context.Background(), tenantB, s.ID); !errors.Is(err, casesearch.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-tenant Get, got %v", err)
	}
}
