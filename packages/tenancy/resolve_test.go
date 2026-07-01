package tenancy_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// fakeTenantRepository is a minimal persistence.TenantRepository
// implementation for unit-testing resolution error paths without a
// database.
type fakeTenantRepository struct {
	bySlug map[string]*persistence.Tenant
}

func (f *fakeTenantRepository) Create(ctx context.Context, exec persistence.Executor, t *persistence.Tenant) error {
	return errors.New("not implemented")
}

func (f *fakeTenantRepository) Get(ctx context.Context, exec persistence.Executor, id uuid.UUID) (*persistence.Tenant, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTenantRepository) GetBySlug(ctx context.Context, exec persistence.Executor, slug string) (*persistence.Tenant, error) {
	if t, ok := f.bySlug[slug]; ok {
		return t, nil
	}
	return nil, persistence.ErrNotFound
}

func (f *fakeTenantRepository) List(ctx context.Context, exec persistence.Executor) ([]*persistence.Tenant, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTenantRepository) Update(ctx context.Context, exec persistence.Executor, t *persistence.Tenant) error {
	return errors.New("not implemented")
}

func (f *fakeTenantRepository) Delete(ctx context.Context, exec persistence.Executor, id uuid.UUID) error {
	return errors.New("not implemented")
}

func TestHeaderResolver_Resolve_MissingHeader(t *testing.T) {
	resolver := tenancy.NewHeaderResolver(&fakeTenantRepository{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)

	_, err := resolver.Resolve(context.Background(), req)
	if !errors.Is(err, tenancy.ErrTenantSlugMissing) {
		t.Fatalf("expected ErrTenantSlugMissing, got %v", err)
	}
}

func TestHeaderResolver_Resolve_UnknownSlug(t *testing.T) {
	resolver := tenancy.NewHeaderResolver(&fakeTenantRepository{bySlug: map[string]*persistence.Tenant{}}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set(tenancy.TenantSlugHeader, "does-not-exist")

	_, err := resolver.Resolve(context.Background(), req)
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected persistence.ErrNotFound, got %v", err)
	}
}

func TestHeaderResolver_Resolve_Success(t *testing.T) {
	want := &persistence.Tenant{ID: uuid.New(), Name: "Acme Legal", Slug: "acme-legal"}
	resolver := tenancy.NewHeaderResolver(&fakeTenantRepository{bySlug: map[string]*persistence.Tenant{
		"acme-legal": want,
	}}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set(tenancy.TenantSlugHeader, "acme-legal")

	got, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != want {
		t.Errorf("expected the resolved tenant %+v, got %+v", want, got)
	}
}

func TestResolveMiddleware_RejectsMissingHeader(t *testing.T) {
	resolver := tenancy.NewHeaderResolver(&fakeTenantRepository{}, nil)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := tenancy.ResolveMiddleware(resolver)(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if called {
		t.Error("expected next handler not to be called without a resolvable tenant")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestResolveMiddleware_ChainsIntoMiddleware(t *testing.T) {
	want := &persistence.Tenant{ID: uuid.New(), Name: "Acme Legal", Slug: "acme-legal"}
	resolver := tenancy.NewHeaderResolver(&fakeTenantRepository{bySlug: map[string]*persistence.Tenant{
		"acme-legal": want,
	}}, nil)

	var gotTenant *persistence.Tenant
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant, _ = tenancy.TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := tenancy.ResolveMiddleware(resolver)(tenancy.Middleware(next))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set(tenancy.TenantSlugHeader, "acme-legal")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotTenant != want {
		t.Errorf("expected the handler to observe the resolved tenant, got %+v", gotTenant)
	}
}
