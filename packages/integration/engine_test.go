package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/integration"
)

func TestNewEngineRejectsNilDeps(t *testing.T) {
	t.Parallel()

	sink, _ := newTestAuditSink(t)
	registry := integration.NewRegistry()
	full := integration.EngineDeps{
		Configs:         integration.NewInMemoryConfigRepository(),
		Credentials:     integration.NewInMemoryCredentialsRepository(),
		Mappings:        integration.NewInMemoryFieldMappingRepository(),
		Imports:         integration.NewInMemoryImportRunRepository(),
		Deliveries:      integration.NewInMemoryDeliveryRunRepository(),
		Reconciliations: integration.NewInMemoryReconciliationRepository(),
		Registry:        registry,
		Audit:           sink,
	}

	t.Run("nil configs", func(t *testing.T) {
		t.Parallel()
		deps := full
		deps.Configs = nil
		if _, err := integration.NewEngine(deps); !errors.Is(err, integration.ErrNilStore) {
			t.Fatalf("NewEngine() error = %v, want ErrNilStore", err)
		}
	})

	t.Run("nil registry", func(t *testing.T) {
		t.Parallel()
		deps := full
		deps.Registry = nil
		if _, err := integration.NewEngine(deps); err == nil {
			t.Fatal("NewEngine() error = nil, want error for nil registry")
		}
	})

	t.Run("nil audit", func(t *testing.T) {
		t.Parallel()
		deps := full
		deps.Audit = nil
		if _, err := integration.NewEngine(deps); !errors.Is(err, integration.ErrNilAuditSink) {
			t.Fatalf("NewEngine() error = %v, want ErrNilAuditSink", err)
		}
	})
}

func TestEngineRegisterConnectorConfigRequiresAuth(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	_, err := engine.RegisterConnectorConfig(context.Background(), integration.ConnectorConfig{
		TenantID:      tenantID,
		ConnectorType: "sandbox",
		DisplayName:   "Sandbox",
	})
	if !errors.Is(err, integration.ErrUnauthenticated) {
		t.Fatalf("RegisterConnectorConfig() error = %v, want ErrUnauthenticated", err)
	}
}

func TestEngineRegisterConnectorConfigRequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(auditorUser(tenantID))

	_, err := engine.RegisterConnectorConfig(ctx, integration.ConnectorConfig{
		TenantID:      tenantID,
		ConnectorType: "sandbox",
		DisplayName:   "Sandbox",
	})
	if !errors.Is(err, integration.ErrForbidden) {
		t.Fatalf("RegisterConnectorConfig() error = %v, want ErrForbidden", err)
	}
}

func TestEngineRegisterConnectorConfigRejectsCrossTenant(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	otherTenant := uuid.New()
	ctx := ctxWithUser(adminUser(tenantID))

	_, err := engine.RegisterConnectorConfig(ctx, integration.ConnectorConfig{
		TenantID:      otherTenant,
		ConnectorType: "sandbox",
		DisplayName:   "Sandbox",
	})
	if !errors.Is(err, integration.ErrCrossTenantAccess) {
		t.Fatalf("RegisterConnectorConfig() error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngineRegisterConnectorConfigSucceeds(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	cfg, err := engine.RegisterConnectorConfig(ctx, integration.ConnectorConfig{
		TenantID:      tenantID,
		ConnectorType: "sandbox",
		DisplayName:   "Sandbox Connection",
	})
	if err != nil {
		t.Fatalf("RegisterConnectorConfig() error = %v", err)
	}
	if cfg.ID == uuid.Nil {
		t.Error("expected a generated ID")
	}

	got, err := engine.GetConnectorConfig(ctx, tenantID, cfg.ID)
	if err != nil {
		t.Fatalf("GetConnectorConfig() error = %v", err)
	}
	if got.DisplayName != "Sandbox Connection" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Sandbox Connection")
	}
}

func TestEngineRegisterConnectorConfigAuditsFailureAndSuccess(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)

	// Failed attempt (forbidden).
	auditorCtx := ctxWithUser(auditorUser(tenantID))
	_, err := engine.RegisterConnectorConfig(auditorCtx, integration.ConnectorConfig{
		TenantID: tenantID, ConnectorType: "sandbox", DisplayName: "X",
	})
	if err == nil {
		t.Fatal("expected forbidden error")
	}

	// Successful attempt.
	adminCtx := ctxWithUser(adminUser(tenantID))
	_, err = engine.RegisterConnectorConfig(adminCtx, integration.ConnectorConfig{
		TenantID: tenantID, ConnectorType: "sandbox", DisplayName: "Y",
	})
	if err != nil {
		t.Fatalf("RegisterConnectorConfig() error = %v", err)
	}

	events, err := auditStore.Query(adminCtx, tenantID, auditlog.Filter{Action: "integration.connector_register"})
	if err != nil {
		t.Fatalf("auditStore.Query() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d connector_register events, want 2", len(events))
	}
	var sawDenied, sawRegistered bool
	for _, ev := range events {
		switch ev.Outcome {
		case "denied":
			sawDenied = true
		case "registered":
			sawRegistered = true
		}
	}
	if !sawDenied || !sawRegistered {
		t.Errorf("expected both denied and registered outcomes, got events=%+v", events)
	}
}

func TestEngineSetCredentialsNeverLogsSecretRef(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	ctx := ctxWithUser(adminUser(tenantID))

	const secret = "super-secret-key-value-should-never-be-logged"
	creds, err := engine.SetCredentials(ctx, integration.ConnectorCredentials{
		TenantID:  tenantID,
		Kind:      integration.CredentialKindAPIKey,
		SecretRef: secret,
		ClientID:  "client-1",
	}, nil)
	if err != nil {
		t.Fatalf("SetCredentials() error = %v", err)
	}
	if creds.LastVerifiedAt.IsZero() {
		t.Error("expected LastVerifiedAt to be set")
	}

	events, err := auditStore.Query(ctx, tenantID, auditlog.Filter{Action: "integration.credentials_set"})
	if err != nil {
		t.Fatalf("auditStore.Query() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d credentials_set events, want 1", len(events))
	}
	if got := events[0].Detail; strings.Contains(got, secret) {
		t.Errorf("audit Detail leaked SecretRef: %q", got)
	}
}

func TestEngineSetCredentialsWithFailingVerify(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	verifyErr := errors.New("upstream rejected key")
	_, err := engine.SetCredentials(ctx, integration.ConnectorCredentials{
		TenantID:  tenantID,
		Kind:      integration.CredentialKindAPIKey,
		SecretRef: "ref-1",
	}, func(_ context.Context, _ integration.ConnectorCredentials) error {
		return verifyErr
	})
	if !errors.Is(err, verifyErr) {
		t.Fatalf("SetCredentials() error = %v, want wrapping %v", err, verifyErr)
	}
}

func TestEngineSetFieldMapping(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	m, err := engine.SetFieldMapping(ctx, integration.FieldMapping{
		TenantID:      tenantID,
		ConnectorType: "sandbox",
		Name:          "sandbox mapping",
		Rules: []integration.FieldRule{
			{SourceField: "case_title", TargetField: "title", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("SetFieldMapping() error = %v", err)
	}
	if m.ID == uuid.Nil {
		t.Error("expected a generated ID")
	}
}

// registerSandboxConfig is a test helper that registers a
// ConnectorConfig bound to the "sandbox" connector type, returning it.
func registerSandboxConfig(t *testing.T, engine *integration.Engine, ctx context.Context, tenantID uuid.UUID) integration.ConnectorConfig {
	t.Helper()
	cfg, err := engine.RegisterConnectorConfig(ctx, integration.ConnectorConfig{
		TenantID:      tenantID,
		ConnectorType: "sandbox",
		DisplayName:   "Sandbox",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("RegisterConnectorConfig() error = %v", err)
	}
	return cfg
}

func TestEngineRunImportSucceeds(t *testing.T) {
	t.Parallel()

	sandbox := integration.NewSandboxConnector("sandbox")
	sandbox.SeedCase(integration.InboundCase{ExternalID: "case-1", ExternalUpdatedAt: time.Now(), Fields: map[string]string{"case_title": "Doe v. Acme"}})
	sandbox.SeedCase(integration.InboundCase{ExternalID: "case-2", ExternalUpdatedAt: time.Now(), Fields: map[string]string{"case_title": "Roe v. Widget"}})

	engine, tenantID := newTestEngineWithConnector(t, "sandbox", sandbox)
	ctx := ctxWithUser(adminUser(tenantID))
	cfg := registerSandboxConfig(t, engine, ctx, tenantID)

	var mappedTitles []string
	run, err := engine.RunImport(ctx, tenantID, cfg.ID, time.Time{}, func(c integration.InboundCase) error {
		mappedTitles = append(mappedTitles, c.Fields["case_title"])
		return nil
	})
	if err != nil {
		t.Fatalf("RunImport() error = %v", err)
	}
	if run.Status != integration.ImportRunStatusSucceeded {
		t.Errorf("Status = %v, want succeeded", run.Status)
	}
	if run.ImportedCount != 2 || run.MappedCount != 2 {
		t.Errorf("ImportedCount/MappedCount = %d/%d, want 2/2", run.ImportedCount, run.MappedCount)
	}
	if len(mappedTitles) != 2 {
		t.Errorf("mapFn invoked %d times, want 2", len(mappedTitles))
	}
}

func TestEngineRunImportPartialOnMappingFailure(t *testing.T) {
	t.Parallel()

	sandbox := integration.NewSandboxConnector("sandbox")
	sandbox.SeedCase(integration.InboundCase{ExternalID: "good-case", ExternalUpdatedAt: time.Now(), Fields: map[string]string{"case_title": "Good"}})
	sandbox.SeedCase(integration.InboundCase{ExternalID: "bad-case", ExternalUpdatedAt: time.Now(), Fields: map[string]string{}})

	engine, tenantID := newTestEngineWithConnector(t, "sandbox", sandbox)
	ctx := ctxWithUser(adminUser(tenantID))
	cfg := registerSandboxConfig(t, engine, ctx, tenantID)

	run, err := engine.RunImport(ctx, tenantID, cfg.ID, time.Time{}, func(c integration.InboundCase) error {
		if c.Fields["case_title"] == "" {
			return errors.New("missing required title")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunImport() error = %v", err)
	}
	if run.Status != integration.ImportRunStatusPartial {
		t.Errorf("Status = %v, want partial", run.Status)
	}
	if len(run.FailedExternalIDs) != 1 || run.FailedExternalIDs[0] != "bad-case" {
		t.Errorf("FailedExternalIDs = %v, want [bad-case]", run.FailedExternalIDs)
	}
}

func TestEngineRunImportFailsWhenConnectorDisabled(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))

	cfg, err := engine.RegisterConnectorConfig(ctx, integration.ConnectorConfig{
		TenantID:      tenantID,
		ConnectorType: "sandbox",
		DisplayName:   "Disabled",
		Enabled:       false,
	})
	if err != nil {
		t.Fatalf("RegisterConnectorConfig() error = %v", err)
	}

	run, err := engine.RunImport(ctx, tenantID, cfg.ID, time.Time{}, nil)
	if err == nil {
		t.Fatal("RunImport() error = nil, want error for disabled connector")
	}
	if run.Status != integration.ImportRunStatusFailed {
		t.Errorf("Status = %v, want failed", run.Status)
	}
}

func TestEngineRunImportFailsWhenConnectorUnavailable(t *testing.T) {
	t.Parallel()

	sandbox := integration.NewSandboxConnector("sandbox")
	sandbox.Unavailable = true

	engine, tenantID := newTestEngineWithConnector(t, "sandbox", sandbox)
	ctx := ctxWithUser(adminUser(tenantID))
	cfg := registerSandboxConfig(t, engine, ctx, tenantID)

	run, err := engine.RunImport(ctx, tenantID, cfg.ID, time.Time{}, nil)
	if err == nil {
		t.Fatal("RunImport() error = nil, want error when connector is unavailable")
	}
	if run.Status != integration.ImportRunStatusFailed {
		t.Errorf("Status = %v, want failed", run.Status)
	}
	if !errors.Is(err, integration.ErrConnectorUnavailable) {
		t.Errorf("error = %v, want wrapping ErrConnectorUnavailable", err)
	}
}

func TestEngineRunDeliverySucceeds(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	cfg := registerSandboxConfig(t, engine, ctx, tenantID)

	run, err := engine.RunDelivery(ctx, tenantID, cfg.ID, integration.OutboundReport{
		CaseExternalID: "case-1",
		ReportKind:     "opinion_summary",
		Format:         "markdown",
		Payload:        []byte("report body"),
	})
	if err != nil {
		t.Fatalf("RunDelivery() error = %v", err)
	}
	if run.Status != integration.DeliveryRunStatusAccepted {
		t.Errorf("Status = %v, want accepted", run.Status)
	}
	if run.ExternalReceiptID == "" {
		t.Error("expected non-empty ExternalReceiptID")
	}
	if run.AttemptCount != 1 {
		t.Errorf("AttemptCount = %d, want 1", run.AttemptCount)
	}
}

func TestEngineRunReconciliationDetectsDrift(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	ctx := ctxWithUser(adminUser(tenantID))
	cfg := registerSandboxConfig(t, engine, ctx, tenantID)

	result, err := engine.RunReconciliation(ctx, tenantID, cfg.ID, integration.ReconciliationKindImport,
		[]string{"case-1", "case-2"}, []string{"case-1"})
	if err != nil {
		t.Fatalf("RunReconciliation() error = %v", err)
	}
	if !result.HasDrift() {
		t.Fatal("expected drift to be detected")
	}

	list, err := engine.ListReconciliations(ctx, tenantID, cfg.ID)
	if err != nil {
		t.Fatalf("ListReconciliations() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d reconciliation results, want 1", len(list))
	}
}

func TestEngineListOperationsRequireViewPermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	adminCtx := ctxWithUser(adminUser(tenantID))
	cfg := registerSandboxConfig(t, engine, adminCtx, tenantID)

	judgeCtx := ctxWithUser(judgeUser(tenantID))
	if _, err := engine.ListConnectorConfigs(judgeCtx, tenantID); !errors.Is(err, integration.ErrForbidden) {
		t.Errorf("ListConnectorConfigs() error = %v, want ErrForbidden", err)
	}
	if _, err := engine.ListImportRuns(judgeCtx, tenantID, cfg.ID); !errors.Is(err, integration.ErrForbidden) {
		t.Errorf("ListImportRuns() error = %v, want ErrForbidden", err)
	}

	auditorCtx := ctxWithUser(auditorUser(tenantID))
	if _, err := engine.ListConnectorConfigs(auditorCtx, tenantID); err != nil {
		t.Errorf("ListConnectorConfigs() with auditor = %v, want nil", err)
	}
}
