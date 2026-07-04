package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/integration"
)

func TestInMemoryConfigRepositoryTenantIsolation(t *testing.T) {
	t.Parallel()
	repo := integration.NewInMemoryConfigRepository()
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()

	cfg := &integration.ConnectorConfig{ID: uuid.New(), TenantID: tenantA, ConnectorType: "sandbox", DisplayName: "A"}
	if err := repo.Create(ctx, tenantA, cfg); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := repo.Get(ctx, tenantB, cfg.ID); !errors.Is(err, integration.ErrConnectorNotFound) {
		t.Fatalf("Get() from tenantB error = %v, want ErrConnectorNotFound", err)
	}

	got, err := repo.Get(ctx, tenantA, cfg.ID)
	if err != nil {
		t.Fatalf("Get() from tenantA error = %v", err)
	}
	if got.DisplayName != "A" {
		t.Errorf("DisplayName = %q, want A", got.DisplayName)
	}

	listB, err := repo.List(ctx, tenantB)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listB) != 0 {
		t.Errorf("List() for tenantB returned %d configs, want 0", len(listB))
	}
}

func TestInMemoryConfigRepositoryRejectsCrossTenantCreate(t *testing.T) {
	t.Parallel()
	repo := integration.NewInMemoryConfigRepository()
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()

	cfg := &integration.ConnectorConfig{ID: uuid.New(), TenantID: tenantB, ConnectorType: "sandbox", DisplayName: "Mismatched"}
	if err := repo.Create(ctx, tenantA, cfg); !errors.Is(err, integration.ErrCrossTenantAccess) {
		t.Fatalf("Create() error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestInMemoryConfigRepositoryUpdateNotFound(t *testing.T) {
	t.Parallel()
	repo := integration.NewInMemoryConfigRepository()
	ctx := context.Background()
	tenantID := uuid.New()

	cfg := &integration.ConnectorConfig{ID: uuid.New(), TenantID: tenantID, ConnectorType: "sandbox", DisplayName: "Ghost"}
	if err := repo.Update(ctx, tenantID, cfg); !errors.Is(err, integration.ErrConnectorNotFound) {
		t.Fatalf("Update() error = %v, want ErrConnectorNotFound", err)
	}
}

func TestInMemoryCredentialsRepositoryLifecycle(t *testing.T) {
	t.Parallel()
	repo := integration.NewInMemoryCredentialsRepository()
	ctx := context.Background()
	tenantID := uuid.New()

	creds := &integration.ConnectorCredentials{
		ID: uuid.New(), TenantID: tenantID, Kind: integration.CredentialKindAPIKey, SecretRef: "ref-1",
	}
	if err := repo.Create(ctx, tenantID, creds); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.Get(ctx, tenantID, creds.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.SecretRef != "ref-1" {
		t.Errorf("SecretRef = %q, want ref-1", got.SecretRef)
	}

	got.SecretRef = "ref-2"
	if err := repo.Update(ctx, tenantID, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := repo.Get(ctx, tenantID, creds.ID)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.SecretRef != "ref-2" {
		t.Errorf("SecretRef after update = %q, want ref-2", updated.SecretRef)
	}

	otherTenant := uuid.New()
	if _, err := repo.Get(ctx, otherTenant, creds.ID); !errors.Is(err, integration.ErrCredentialsNotFound) {
		t.Fatalf("Get() from other tenant error = %v, want ErrCredentialsNotFound", err)
	}
}

func TestInMemoryFieldMappingRepositoryLifecycle(t *testing.T) {
	t.Parallel()
	repo := integration.NewInMemoryFieldMappingRepository()
	ctx := context.Background()
	tenantID := uuid.New()

	m := &integration.FieldMapping{ID: uuid.New(), TenantID: tenantID, ConnectorType: "sandbox", Name: "m1"}
	if err := repo.Create(ctx, tenantID, m); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, err := repo.List(ctx, tenantID)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() returned %d mappings, want 1", len(list))
	}

	if _, err := repo.Get(ctx, uuid.New(), m.ID); !errors.Is(err, integration.ErrMappingNotFound) {
		t.Fatalf("Get() from wrong tenant error = %v, want ErrMappingNotFound", err)
	}
}

func TestInMemoryImportRunRepositoryListForConnector(t *testing.T) {
	t.Parallel()
	repo := integration.NewInMemoryImportRunRepository()
	ctx := context.Background()
	tenantID := uuid.New()
	connA := uuid.New()
	connB := uuid.New()

	runA := &integration.ImportRun{ID: uuid.New(), TenantID: tenantID, ConnectorConfigID: connA, Status: integration.ImportRunStatusSucceeded, StartedAt: time.Now(), FinishedAt: time.Now()}
	runB := &integration.ImportRun{ID: uuid.New(), TenantID: tenantID, ConnectorConfigID: connB, Status: integration.ImportRunStatusSucceeded, StartedAt: time.Now(), FinishedAt: time.Now()}

	if err := repo.Create(ctx, tenantID, runA); err != nil {
		t.Fatalf("Create() runA error = %v", err)
	}
	if err := repo.Create(ctx, tenantID, runB); err != nil {
		t.Fatalf("Create() runB error = %v", err)
	}

	listA, err := repo.ListForConnector(ctx, tenantID, connA)
	if err != nil {
		t.Fatalf("ListForConnector() error = %v", err)
	}
	if len(listA) != 1 || listA[0].ID != runA.ID {
		t.Fatalf("ListForConnector(connA) = %+v, want only runA", listA)
	}

	all, err := repo.ListAll(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListAll() returned %d runs, want 2", len(all))
	}
}

func TestInMemoryDeliveryRunRepositoryNotFound(t *testing.T) {
	t.Parallel()
	repo := integration.NewInMemoryDeliveryRunRepository()
	ctx := context.Background()
	if _, err := repo.Get(ctx, uuid.New(), uuid.New()); !errors.Is(err, integration.ErrDeliveryRunNotFound) {
		t.Fatalf("Get() error = %v, want ErrDeliveryRunNotFound", err)
	}
}

func TestInMemoryReconciliationRepositoryLifecycle(t *testing.T) {
	t.Parallel()
	repo := integration.NewInMemoryReconciliationRepository()
	ctx := context.Background()
	tenantID := uuid.New()
	connID := uuid.New()

	result := &integration.ReconciliationResult{
		ID: uuid.New(), TenantID: tenantID, ConnectorConfigID: connID,
		Kind: integration.ReconciliationKindImport, RanAt: time.Now(), RanBy: uuid.New(),
	}
	if err := repo.Create(ctx, tenantID, result); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, err := repo.ListForConnector(ctx, tenantID, connID)
	if err != nil {
		t.Fatalf("ListForConnector() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListForConnector() returned %d results, want 1", len(list))
	}

	if _, err := repo.Get(ctx, uuid.New(), result.ID); !errors.Is(err, integration.ErrReconciliationNotFound) {
		t.Fatalf("Get() from wrong tenant error = %v, want ErrReconciliationNotFound", err)
	}
}

func TestConnectorConfigValidate(t *testing.T) {
	t.Parallel()

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()
		var cfg *integration.ConnectorConfig
		if err := cfg.Validate(); !errors.Is(err, integration.ErrInvalidConnectorConfig) {
			t.Fatalf("Validate() = %v, want ErrInvalidConnectorConfig", err)
		}
	})

	t.Run("missing tenant", func(t *testing.T) {
		t.Parallel()
		cfg := &integration.ConnectorConfig{ConnectorType: "sandbox", DisplayName: "x"}
		if err := cfg.Validate(); !errors.Is(err, integration.ErrEmptyTenantID) {
			t.Fatalf("Validate() = %v, want ErrEmptyTenantID", err)
		}
	})

	t.Run("missing connector type", func(t *testing.T) {
		t.Parallel()
		cfg := &integration.ConnectorConfig{TenantID: uuid.New(), DisplayName: "x"}
		if err := cfg.Validate(); !errors.Is(err, integration.ErrInvalidConnectorConfig) {
			t.Fatalf("Validate() = %v, want ErrInvalidConnectorConfig", err)
		}
	})

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		cfg := &integration.ConnectorConfig{TenantID: uuid.New(), ConnectorType: "sandbox", DisplayName: "x"}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
}

func TestImportRunAndDeliveryRunValidate(t *testing.T) {
	t.Parallel()

	t.Run("import run nil", func(t *testing.T) {
		t.Parallel()
		var r *integration.ImportRun
		if err := r.Validate(); !errors.Is(err, integration.ErrInvalidImportRun) {
			t.Fatalf("Validate() = %v, want ErrInvalidImportRun", err)
		}
	})

	t.Run("import run invalid status", func(t *testing.T) {
		t.Parallel()
		r := &integration.ImportRun{TenantID: uuid.New(), ConnectorConfigID: uuid.New(), Status: "bogus"}
		if err := r.Validate(); !errors.Is(err, integration.ErrInvalidImportRun) {
			t.Fatalf("Validate() = %v, want ErrInvalidImportRun", err)
		}
	})

	t.Run("delivery run nil", func(t *testing.T) {
		t.Parallel()
		var r *integration.DeliveryRun
		if err := r.Validate(); !errors.Is(err, integration.ErrInvalidDeliveryRun) {
			t.Fatalf("Validate() = %v, want ErrInvalidDeliveryRun", err)
		}
	})

	t.Run("delivery run valid", func(t *testing.T) {
		t.Parallel()
		r := &integration.DeliveryRun{TenantID: uuid.New(), ConnectorConfigID: uuid.New(), Status: integration.DeliveryRunStatusAccepted}
		if err := r.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
}

func TestRegistryLifecycle(t *testing.T) {
	t.Parallel()
	registry := integration.NewRegistry()
	sandbox := integration.NewSandboxConnector("sandbox-1")

	if err := registry.Register("sandbox-1", sandbox); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.Register("sandbox-1", sandbox); !errors.Is(err, integration.ErrDuplicateConnector) {
		t.Fatalf("Register() duplicate error = %v, want ErrDuplicateConnector", err)
	}

	got, err := registry.Get("sandbox-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID() != "sandbox-1" {
		t.Errorf("Get().ID() = %q, want sandbox-1", got.ID())
	}

	if _, err := registry.Get("does-not-exist"); !errors.Is(err, integration.ErrConnectorNotFound) {
		t.Fatalf("Get() missing error = %v, want ErrConnectorNotFound", err)
	}

	if list := registry.List(); len(list) != 1 || list[0] != "sandbox-1" {
		t.Errorf("List() = %v, want [sandbox-1]", list)
	}

	registry.Unregister("sandbox-1")
	if _, err := registry.Get("sandbox-1"); !errors.Is(err, integration.ErrConnectorNotFound) {
		t.Fatalf("Get() after Unregister error = %v, want ErrConnectorNotFound", err)
	}
}

func TestRegistryRejectsInvalidRegistration(t *testing.T) {
	t.Parallel()
	registry := integration.NewRegistry()

	if err := registry.Register("", integration.NewSandboxConnector("x")); err == nil {
		t.Fatal("Register() with empty id error = nil, want error")
	}
	if err := registry.Register("x", nil); !errors.Is(err, integration.ErrNilConnector) {
		t.Fatalf("Register() with nil connector error = %v, want ErrNilConnector", err)
	}
}
