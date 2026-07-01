package tenancy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// fakeDeploymentRepository lets the fail-fast cross-tenant guard be
// tested without a live database: since the guard runs before
// WithTenantScope opens a transaction, these tests only need to prove
// the fake is never invoked.
type fakeDeploymentRepository struct {
	called bool
}

func (f *fakeDeploymentRepository) Create(ctx context.Context, exec persistence.Executor, d *persistence.Deployment) error {
	f.called = true
	return nil
}

func (f *fakeDeploymentRepository) Get(ctx context.Context, exec persistence.Executor, id uuid.UUID) (*persistence.Deployment, error) {
	f.called = true
	return nil, nil
}

func (f *fakeDeploymentRepository) List(ctx context.Context, exec persistence.Executor) ([]*persistence.Deployment, error) {
	f.called = true
	return nil, nil
}

func (f *fakeDeploymentRepository) Update(ctx context.Context, exec persistence.Executor, d *persistence.Deployment) error {
	f.called = true
	return nil
}

func (f *fakeDeploymentRepository) Delete(ctx context.Context, exec persistence.Executor, id uuid.UUID) error {
	f.called = true
	return nil
}

func TestTenantScopedDeploymentRepository_Create_RejectsMismatchedTenant(t *testing.T) {
	fake := &fakeDeploymentRepository{}
	repo := tenancy.NewTenantScopedDeploymentRepository(nil, fake)

	scopeTenant := uuid.New()
	otherTenant := uuid.New()
	deployment := &persistence.Deployment{TenantID: otherTenant, Profile: "standard"}

	err := repo.Create(context.Background(), scopeTenant, deployment)
	if !errors.Is(err, tenancy.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess, got %v", err)
	}
	if fake.called {
		t.Error("expected the underlying repository not to be called before the DB, but it was")
	}
}

func TestTenantScopedDeploymentRepository_Update_RejectsMismatchedTenant(t *testing.T) {
	fake := &fakeDeploymentRepository{}
	repo := tenancy.NewTenantScopedDeploymentRepository(nil, fake)

	scopeTenant := uuid.New()
	otherTenant := uuid.New()
	deployment := &persistence.Deployment{ID: uuid.New(), TenantID: otherTenant, Profile: "standard"}

	err := repo.Update(context.Background(), scopeTenant, deployment)
	if !errors.Is(err, tenancy.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess, got %v", err)
	}
	if fake.called {
		t.Error("expected the underlying repository not to be called before the DB, but it was")
	}
}

func TestTenantScopedDeploymentRepository_Create_StampsUnsetTenantID(t *testing.T) {
	fake := &fakeDeploymentRepository{}
	repo := tenancy.NewTenantScopedDeploymentRepository(nil, fake)

	scopeTenant := uuid.New()
	deployment := &persistence.Deployment{Profile: "standard"}

	// pool is nil, so WithTenantScope itself will fail once past the
	// guard; this test only asserts the guard does not reject a
	// deployment with an unset TenantID and does attempt to proceed
	// (i.e. TenantID gets stamped before the nil-pool error surfaces).
	err := repo.Create(context.Background(), scopeTenant, deployment)
	if errors.Is(err, tenancy.ErrCrossTenantAccess) {
		t.Fatalf("expected an unset TenantID not to trigger ErrCrossTenantAccess, got %v", err)
	}
	if deployment.TenantID != scopeTenant {
		t.Errorf("expected TenantID to be stamped to %v, got %v", scopeTenant, deployment.TenantID)
	}
}
