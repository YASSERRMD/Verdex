package dataresidency_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/provider"
)

func TestGuard_CheckTransfer_AllowedOperationRecordsAuditNoAlert(t *testing.T) {
	sink, store := newTestAuditSink(t)
	alertSink := &recordingAlertSink{}
	guard, err := dataresidency.NewGuard(sink, alertSink)
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}

	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	tenantID := uuid.New()

	if err := guard.CheckTransfer(context.Background(), tenantID, policy.DeploymentID, "eu", "eu", policy); err != nil {
		t.Fatalf("expected in-region transfer to be allowed: %v", err)
	}

	if alertSink.count() != 0 {
		t.Fatalf("expected no alert for an allowed transfer, got %d", alertSink.count())
	}

	auditor := ctxWithUser(&identity.User{ID: uuid.New(), TenantID: tenantID, Roles: []identity.Role{identity.RoleAuditor}, Status: identity.UserStatusActive})
	events, err := store.Query(auditor, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 audited event, got %d", len(events))
	}
}

func TestGuard_CheckTransfer_BlockedOperationRecordsAuditAndAlerts(t *testing.T) {
	sink, store := newTestAuditSink(t)
	alertSink := &recordingAlertSink{}
	guard, err := dataresidency.NewGuard(sink, alertSink)
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}

	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	tenantID := uuid.New()

	err = guard.CheckTransfer(context.Background(), tenantID, policy.DeploymentID, "eu", "cn", policy)
	if !errors.Is(err, dataresidency.ErrRegionNotAllowed) {
		t.Fatalf("expected ErrRegionNotAllowed from guard, got %v", err)
	}

	if alertSink.count() != 1 {
		t.Fatalf("expected exactly 1 alert fired on violation, got %d", alertSink.count())
	}
	if alertSink.events[0].ViolationType != dataresidency.ViolationTransferBlocked {
		t.Fatalf("expected ViolationTransferBlocked, got %q", alertSink.events[0].ViolationType)
	}

	auditor := ctxWithUser(&identity.User{ID: uuid.New(), TenantID: tenantID, Roles: []identity.Role{identity.RoleAuditor}, Status: identity.UserStatusActive})
	events, err := store.Query(auditor, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 1 || events[0].Outcome != "blocked" {
		t.Fatalf("expected 1 audited 'blocked' event, got %+v", events)
	}
}

func TestNewGuard_RequiresAuditSink(t *testing.T) {
	if _, err := dataresidency.NewGuard(nil, dataresidency.NoOpAlertSink{}); !errors.Is(err, dataresidency.ErrNilStore) {
		t.Fatalf("expected ErrNilStore, got %v", err)
	}
}

func TestNewGuard_DefaultsToNoOpAlertSink(t *testing.T) {
	sink, _ := newTestAuditSink(t)
	guard, err := dataresidency.NewGuard(sink, nil)
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}
	if guard == nil {
		t.Fatal("expected non-nil guard")
	}
}

func TestGuard_CheckProviderLocality_RejectsDisallowedRegionAndAlerts(t *testing.T) {
	sink, store := newTestAuditSink(t)
	alertSink := &recordingAlertSink{}
	guard, err := dataresidency.NewGuard(sink, alertSink)
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}

	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	tenantID := uuid.New()
	cap := provider.Capability{ProviderID: "openai", ModelID: "gpt-us", Region: "us"}

	err = guard.CheckProviderLocality(context.Background(), tenantID, policy.DeploymentID, cap, policy)
	if !errors.Is(err, dataresidency.ErrRegionNotAllowed) {
		t.Fatalf("expected ErrRegionNotAllowed from guard, got %v", err)
	}
	if alertSink.count() != 1 {
		t.Fatalf("expected exactly 1 alert fired on provider-locality violation, got %d", alertSink.count())
	}

	auditor := ctxWithUser(&identity.User{ID: uuid.New(), TenantID: tenantID, Roles: []identity.Role{identity.RoleAuditor}, Status: identity.UserStatusActive})
	events, err := store.Query(auditor, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 1 || events[0].Outcome != "blocked" {
		t.Fatalf("expected 1 audited 'blocked' event for the provider check, got %+v", events)
	}
}

func TestGuard_CheckProviderLocality_AllowsInRegionProvider(t *testing.T) {
	sink, _ := newTestAuditSink(t)
	alertSink := &recordingAlertSink{}
	guard, err := dataresidency.NewGuard(sink, alertSink)
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}

	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	cap := provider.Capability{ProviderID: "anthropic", ModelID: "claude-eu", Region: "eu"}

	if err := guard.CheckProviderLocality(context.Background(), uuid.New(), policy.DeploymentID, cap, policy); err != nil {
		t.Fatalf("expected eu-region provider to be allowed: %v", err)
	}
	if alertSink.count() != 0 {
		t.Fatalf("expected no alert for an allowed provider, got %d", alertSink.count())
	}
}
