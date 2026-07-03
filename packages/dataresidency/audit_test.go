package dataresidency_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestAuditSink_RecordsEveryTransferCheck(t *testing.T) {
	sink, store := newTestAuditSink(t)
	tenantID := uuid.New()
	deploymentID := uuid.New()

	if _, err := sink.RecordTransferCheck(context.Background(), tenantID, deploymentID, "eu", "eu", nil); err != nil {
		t.Fatalf("RecordTransferCheck (allowed): %v", err)
	}
	if _, err := sink.RecordTransferCheck(context.Background(), tenantID, deploymentID, "eu", "cn", dataresidency.ErrRegionNotAllowed); err != nil {
		t.Fatalf("RecordTransferCheck (blocked): %v", err)
	}

	auditor := ctxWithUser(newTestUser(tenantID, identity.RoleAuditor))
	events, err := store.Query(auditor, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 recorded events, got %d", len(events))
	}
	if events[0].Outcome != "allowed" {
		t.Fatalf("expected first event outcome 'allowed', got %q", events[0].Outcome)
	}
	if events[1].Outcome != "blocked" {
		t.Fatalf("expected second event outcome 'blocked', got %q", events[1].Outcome)
	}
	for _, ev := range events {
		if ev.Kind != auditlog.KindSystem {
			t.Fatalf("expected KindSystem, got %q", ev.Kind)
		}
	}
}

func TestAuditSink_RecordsVerificationReport(t *testing.T) {
	sink, store := newTestAuditSink(t)
	tenantID := uuid.New()
	deploymentID := uuid.New()

	report := dataresidency.Report{
		DeploymentID: deploymentID,
		Checks: []dataresidency.CheckResult{
			{Kind: dataresidency.CheckStorageRegion, Passed: false, Detail: "mismatch"},
		},
	}

	if _, err := sink.RecordVerification(context.Background(), tenantID, report); err != nil {
		t.Fatalf("RecordVerification: %v", err)
	}

	auditor := ctxWithUser(newTestUser(tenantID, identity.RoleAuditor))
	events, err := store.Query(auditor, tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 recorded event, got %d", len(events))
	}
	if events[0].Outcome != "fail" {
		t.Fatalf("expected outcome 'fail', got %q", events[0].Outcome)
	}
}
