package threatmodel_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

func TestNewEngine_NilCatalogueRejected(t *testing.T) {
	t.Parallel()

	_, err := threatmodel.NewEngine(nil, nil)
	if !errors.Is(err, threatmodel.ErrNilRepository) {
		t.Errorf("NewEngine(nil, nil) error = %v, want ErrNilRepository", err)
	}
}

func TestNewEngine_NilAuditAllowed(t *testing.T) {
	t.Parallel()

	catalogue := threatmodel.NewCatalogue(threatmodel.SeedThreatModels())
	engine, err := threatmodel.NewEngine(catalogue, nil)
	if err != nil {
		t.Fatalf("NewEngine(catalogue, nil) error = %v, want nil", err)
	}
	if engine == nil {
		t.Fatal("NewEngine(catalogue, nil) returned nil engine")
	}
}

func TestEngine_ListThreatModels_RequiresAuthentication(t *testing.T) {
	t.Parallel()
	engine, _ := newTestEngine(t)

	_, err := engine.ListThreatModels(t.Context())
	if !errors.Is(err, threatmodel.ErrUnauthenticated) {
		t.Errorf("ListThreatModels() error = %v, want ErrUnauthenticated", err)
	}
}

func TestEngine_ListThreatModels_RequiresPermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	judge := judgeUser(tenantID)
	_, err := engine.ListThreatModels(ctxWithUser(judge))
	if !errors.Is(err, threatmodel.ErrForbidden) {
		t.Errorf("ListThreatModels() with judge role error = %v, want ErrForbidden", err)
	}
}

func TestEngine_ListThreatModels_AuditorCanView(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	auditor := auditorUser(tenantID)
	models, err := engine.ListThreatModels(ctxWithUser(auditor))
	if err != nil {
		t.Fatalf("ListThreatModels() with auditor role error = %v, want nil", err)
	}
	if len(models) < 3 {
		t.Errorf("ListThreatModels() returned %d models, want at least 3", len(models))
	}
}

func TestEngine_GetMitigation_NotFound(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	admin := adminUser(tenantID)
	_, err := engine.GetMitigation(ctxWithUser(admin), uuid.New())
	if !errors.Is(err, threatmodel.ErrMitigationNotFound) {
		t.Errorf("GetMitigation() error = %v, want ErrMitigationNotFound", err)
	}
}

func TestEngine_TransitionMitigation_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	id := findMitigationIDByStatus(t, engine, tenantID, threatmodel.MitigationImplemented)

	auditor := auditorUser(tenantID)
	_, err := engine.TransitionMitigation(ctxWithUser(auditor), tenantID, id, threatmodel.MitigationVerified)
	if !errors.Is(err, threatmodel.ErrForbidden) {
		t.Errorf("TransitionMitigation() with view-only auditor error = %v, want ErrForbidden", err)
	}
}

func TestEngine_TransitionMitigation_ValidTransition(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	id := findMitigationIDByStatus(t, engine, tenantID, threatmodel.MitigationImplemented)

	admin := adminUser(tenantID)
	updated, err := engine.TransitionMitigation(ctxWithUser(admin), tenantID, id, threatmodel.MitigationVerified)
	if err != nil {
		t.Fatalf("TransitionMitigation() error = %v, want nil", err)
	}
	if updated.Status != threatmodel.MitigationVerified {
		t.Errorf("TransitionMitigation() resulted in status %v, want MitigationVerified", updated.Status)
	}

	// Confirm the change is durable within the catalogue -- a
	// subsequent GetMitigation must reflect it.
	got, err := engine.GetMitigation(ctxWithUser(admin), id)
	if err != nil {
		t.Fatalf("GetMitigation() error = %v", err)
	}
	if got.Status != threatmodel.MitigationVerified {
		t.Errorf("GetMitigation() after transition = %v, want MitigationVerified", got.Status)
	}
}

func TestEngine_TransitionMitigation_IllegalTransitionRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	id := findMitigationIDByStatus(t, engine, tenantID, threatmodel.MitigationImplemented)

	admin := adminUser(tenantID)
	// Implemented -> Planned is legal (regression), but there is no
	// seeded MitigationPlanned entry to test the *illegal* skip
	// (Planned -> Verified) against directly; instead prove Verified
	// cannot be skipped past Implemented in a single illegal hop by
	// first transitioning to Verified, then trying to jump straight to
	// Implemented via TransitionMitigation (not ResetMitigation), which
	// CanTransitionMitigation defines as illegal (Verified is terminal
	// except via ResetMitigation).
	if _, err := engine.TransitionMitigation(ctxWithUser(admin), tenantID, id, threatmodel.MitigationVerified); err != nil {
		t.Fatalf("setup TransitionMitigation to Verified: %v", err)
	}

	_, err := engine.TransitionMitigation(ctxWithUser(admin), tenantID, id, threatmodel.MitigationImplemented)
	if !errors.Is(err, threatmodel.ErrIllegalStatusTransition) {
		t.Errorf("TransitionMitigation(Verified -> Implemented) error = %v, want ErrIllegalStatusTransition", err)
	}
}

func TestEngine_TransitionMitigation_NotFound(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	admin := adminUser(tenantID)
	_, err := engine.TransitionMitigation(ctxWithUser(admin), tenantID, uuid.New(), threatmodel.MitigationImplemented)
	if !errors.Is(err, threatmodel.ErrMitigationNotFound) {
		t.Errorf("TransitionMitigation() error = %v, want ErrMitigationNotFound", err)
	}
}

func TestEngine_TransitionMitigation_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	id := findMitigationIDByStatus(t, engine, tenantA, threatmodel.MitigationImplemented)
	tenantB := uuid.New()

	adminA := adminUser(tenantA)
	_, err := engine.TransitionMitigation(ctxWithUser(adminA), tenantB, id, threatmodel.MitigationVerified)
	if !errors.Is(err, threatmodel.ErrCrossTenantAccess) {
		t.Errorf("TransitionMitigation() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngine_ResetMitigation_RequiresReason(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	id := findMitigationIDByStatus(t, engine, tenantID, threatmodel.MitigationImplemented)

	admin := adminUser(tenantID)
	_, err := engine.ResetMitigation(ctxWithUser(admin), tenantID, id, "   ")
	if !errors.Is(err, threatmodel.ErrInputInvalidStructure) {
		t.Errorf("ResetMitigation() with blank reason error = %v, want ErrInputInvalidStructure", err)
	}
}

func TestEngine_ResetMitigation_RegressesVerifiedMitigation(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	id := findMitigationIDByStatus(t, engine, tenantID, threatmodel.MitigationImplemented)
	admin := adminUser(tenantID)

	if _, err := engine.TransitionMitigation(ctxWithUser(admin), tenantID, id, threatmodel.MitigationVerified); err != nil {
		t.Fatalf("setup TransitionMitigation to Verified: %v", err)
	}

	updated, err := engine.ResetMitigation(ctxWithUser(admin), tenantID, id, "control regressed during a dependency upgrade, re-verification required")
	if err != nil {
		t.Fatalf("ResetMitigation() error = %v, want nil", err)
	}
	if updated.Status != threatmodel.MitigationPlanned {
		t.Errorf("ResetMitigation() resulted in status %v, want MitigationPlanned", updated.Status)
	}
	if updated.LastTransitionAt.IsZero() {
		t.Error("ResetMitigation() left LastTransitionAt zero, want it set to the reset time")
	}

	// The catalogue itself, not just the returned struct, must reflect
	// the reset -- confirmed by looking the same mitigation back up by
	// its now-current MitigationPlanned status.
	foundID := findMitigationIDByStatus(t, engine, tenantID, threatmodel.MitigationPlanned)
	if foundID != id {
		t.Errorf("findMitigationIDByStatus(MitigationPlanned) = %s, want the just-reset mitigation %s", foundID, id)
	}
}

func TestEngine_MitigationHistory_RecordsTransitions(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	id := findMitigationIDByStatus(t, engine, tenantID, threatmodel.MitigationImplemented)
	admin := adminUser(tenantID)

	if _, err := engine.TransitionMitigation(ctxWithUser(admin), tenantID, id, threatmodel.MitigationVerified); err != nil {
		t.Fatalf("TransitionMitigation: %v", err)
	}

	events, err := auditStore.Query(ctxWithUser(admin), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("auditStore.Query: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Target == id.String() {
			found = true
		}
	}
	if !found {
		t.Errorf("audit trail does not contain an event for mitigation %s", id)
	}
}

func TestEngine_MitigationHistory_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.MitigationHistory(ctxWithUser(adminA), tenantB, auditlog.Filter{})
	if !errors.Is(err, threatmodel.ErrCrossTenantAccess) {
		t.Errorf("MitigationHistory() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}
