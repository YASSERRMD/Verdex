package persistence

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestPostgresTenantRepository_Create_NilTenant(t *testing.T) {
	t.Parallel()

	r := NewPostgresTenantRepository()
	err := r.Create(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil tenant, got nil")
	}
}

func TestPostgresDeploymentRepository_Create_NilDeployment(t *testing.T) {
	t.Parallel()

	r := NewPostgresDeploymentRepository()
	err := r.Create(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil deployment, got nil")
	}
}

func TestPostgresTenantRepository_Update_NilTenant(t *testing.T) {
	t.Parallel()

	r := NewPostgresTenantRepository()
	err := r.Update(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil tenant, got nil")
	}
}

func TestPostgresDeploymentRepository_Update_NilDeployment(t *testing.T) {
	t.Parallel()

	r := NewPostgresDeploymentRepository()
	err := r.Update(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil deployment, got nil")
	}
}

func TestErrNotFound_IsDistinctSentinel(t *testing.T) {
	t.Parallel()

	if ErrNotFound == nil {
		t.Fatal("ErrNotFound must not be nil")
	}
	if ErrNotFound.Error() == "" {
		t.Fatal("ErrNotFound must have a non-empty message")
	}
}

func TestDeploymentStatusConstants(t *testing.T) {
	t.Parallel()

	statuses := map[string]bool{
		DeploymentStatusProvisioning:   true,
		DeploymentStatusActive:         true,
		DeploymentStatusSuspended:      true,
		DeploymentStatusDecommissioned: true,
	}
	if len(statuses) != 4 {
		t.Fatalf("expected 4 distinct deployment statuses, got %d", len(statuses))
	}
}

func TestTenant_ZeroValueHasNilUUID(t *testing.T) {
	t.Parallel()

	var tenant Tenant
	if tenant.ID != uuid.Nil {
		t.Fatalf("expected zero-value Tenant.ID to be uuid.Nil, got %v", tenant.ID)
	}
}
