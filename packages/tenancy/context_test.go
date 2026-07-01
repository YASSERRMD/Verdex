package tenancy_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

func TestTenantFromContext_ReturnsBoundTenant(t *testing.T) {
	want := &persistence.Tenant{ID: uuid.New(), Name: "Acme Legal", Slug: "acme-legal"}

	ctx := tenancy.WithTenant(context.Background(), want)

	got, ok := tenancy.TenantFromContext(ctx)
	if !ok {
		t.Fatal("expected TenantFromContext to report ok=true")
	}
	if got != want {
		t.Errorf("expected TenantFromContext to return the bound tenant, got %+v", got)
	}
}

func TestTenantFromContext_AbsentWhenUnset(t *testing.T) {
	got, ok := tenancy.TenantFromContext(context.Background())
	if ok {
		t.Errorf("expected ok=false for a context with no bound tenant, got tenant %+v", got)
	}
	if got != nil {
		t.Errorf("expected nil tenant, got %+v", got)
	}
}

func TestTenantFromContext_AbsentWhenNilTenantBound(t *testing.T) {
	ctx := tenancy.WithTenant(context.Background(), nil)

	got, ok := tenancy.TenantFromContext(ctx)
	if ok {
		t.Errorf("expected ok=false when a nil tenant was bound, got tenant %+v", got)
	}
	if got != nil {
		t.Errorf("expected nil tenant, got %+v", got)
	}
}
