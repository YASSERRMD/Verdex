package tenancy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

func TestMiddleware_AttachesResolvedTenant(t *testing.T) {
	want := &persistence.Tenant{ID: uuid.New(), Name: "Acme Legal", Slug: "acme-legal"}

	var gotTenant *persistence.Tenant
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant, _ = tenancy.TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := tenancy.Middleware(next)

	ctx := tenancy.WithResolvedTenant(context.Background(), want)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotTenant != want {
		t.Errorf("expected handler to observe the resolved tenant, got %+v", gotTenant)
	}
}

func TestMiddleware_RejectsUnresolvedTenant(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := tenancy.Middleware(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if called {
		t.Error("expected next handler not to be called without a resolved tenant")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
