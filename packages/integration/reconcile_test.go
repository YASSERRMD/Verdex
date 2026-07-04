package integration_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/integration"
)

func TestReconcileCleanMatch(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	connID := uuid.New()
	ranBy := uuid.New()

	expected := []string{"case-1", "case-2", "case-3"}
	observed := []string{"case-1", "case-2", "case-3"}

	result, err := integration.Reconcile(tenantID, connID, integration.ReconciliationKindImport, expected, observed, ranBy, time.Now())
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.HasDrift() {
		t.Errorf("HasDrift() = true, want false for a clean match; missing=%v unexpected=%v", result.MissingExternalIDs, result.UnexpectedExternalIDs)
	}
	if result.ExpectedCount != 3 || result.ObservedCount != 3 {
		t.Errorf("counts = %d/%d, want 3/3", result.ExpectedCount, result.ObservedCount)
	}
}

func TestReconcileDetectsMissingRecords(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	connID := uuid.New()
	ranBy := uuid.New()

	expected := []string{"case-1", "case-2", "case-3"}
	observed := []string{"case-1", "case-2"}

	result, err := integration.Reconcile(tenantID, connID, integration.ReconciliationKindImport, expected, observed, ranBy, time.Now())
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.HasDrift() {
		t.Fatal("HasDrift() = false, want true")
	}
	if len(result.MissingExternalIDs) != 1 || result.MissingExternalIDs[0] != "case-3" {
		t.Errorf("MissingExternalIDs = %v, want [case-3]", result.MissingExternalIDs)
	}
	if len(result.UnexpectedExternalIDs) != 0 {
		t.Errorf("UnexpectedExternalIDs = %v, want empty", result.UnexpectedExternalIDs)
	}
}

func TestReconcileDetectsUnexpectedRecords(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	connID := uuid.New()
	ranBy := uuid.New()

	expected := []string{"case-1", "case-2"}
	observed := []string{"case-1", "case-2", "case-99"}

	result, err := integration.Reconcile(tenantID, connID, integration.ReconciliationKindDelivery, expected, observed, ranBy, time.Now())
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if !result.HasDrift() {
		t.Fatal("HasDrift() = false, want true")
	}
	if len(result.UnexpectedExternalIDs) != 1 || result.UnexpectedExternalIDs[0] != "case-99" {
		t.Errorf("UnexpectedExternalIDs = %v, want [case-99]", result.UnexpectedExternalIDs)
	}
}

func TestReconcileRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	connID := uuid.New()

	t.Run("nil tenant", func(t *testing.T) {
		t.Parallel()
		_, err := integration.Reconcile(uuid.Nil, connID, integration.ReconciliationKindImport, nil, nil, uuid.New(), time.Now())
		if !errors.Is(err, integration.ErrEmptyTenantID) {
			t.Fatalf("Reconcile() error = %v, want ErrEmptyTenantID", err)
		}
	})

	t.Run("nil connector config", func(t *testing.T) {
		t.Parallel()
		_, err := integration.Reconcile(tenantID, uuid.Nil, integration.ReconciliationKindImport, nil, nil, uuid.New(), time.Now())
		if !errors.Is(err, integration.ErrInvalidReconciliation) {
			t.Fatalf("Reconcile() error = %v, want ErrInvalidReconciliation", err)
		}
	})

	t.Run("invalid kind", func(t *testing.T) {
		t.Parallel()
		_, err := integration.Reconcile(tenantID, connID, "bogus", nil, nil, uuid.New(), time.Now())
		if !errors.Is(err, integration.ErrInvalidReconciliation) {
			t.Fatalf("Reconcile() error = %v, want ErrInvalidReconciliation", err)
		}
	})
}

func TestReconciliationResultValidate(t *testing.T) {
	t.Parallel()

	t.Run("nil result", func(t *testing.T) {
		t.Parallel()
		var r *integration.ReconciliationResult
		if err := r.Validate(); !errors.Is(err, integration.ErrInvalidReconciliation) {
			t.Fatalf("Validate() = %v, want ErrInvalidReconciliation", err)
		}
	})

	t.Run("valid result", func(t *testing.T) {
		t.Parallel()
		r := &integration.ReconciliationResult{
			TenantID:          uuid.New(),
			ConnectorConfigID: uuid.New(),
			Kind:              integration.ReconciliationKindImport,
		}
		if err := r.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
}
