package compliance_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/compliance"
)

func validEvidence(tenantID, controlID uuid.UUID) compliance.ControlEvidence {
	return compliance.ControlEvidence{
		TenantID:  tenantID,
		ControlID: controlID,
		Kind:      compliance.EvidenceKindTestName,
		Reference: "packages/compliance.TestSomething",
	}
}

func TestControlEvidence_Validate(t *testing.T) {
	t.Parallel()
	tenantID, controlID := uuid.New(), uuid.New()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		e := validEvidence(tenantID, controlID)
		e.CollectedAt = time.Now()
		if err := e.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		var e *compliance.ControlEvidence
		if err := e.Validate(); !errors.Is(err, compliance.ErrInvalidEvidence) {
			t.Errorf("Validate() = %v, want ErrInvalidEvidence", err)
		}
	})

	t.Run("empty tenant", func(t *testing.T) {
		t.Parallel()
		e := validEvidence(uuid.Nil, controlID)
		e.CollectedAt = time.Now()
		if err := e.Validate(); !errors.Is(err, compliance.ErrEmptyTenantID) {
			t.Errorf("Validate() = %v, want ErrEmptyTenantID", err)
		}
	})

	t.Run("empty control id", func(t *testing.T) {
		t.Parallel()
		e := validEvidence(tenantID, uuid.Nil)
		e.CollectedAt = time.Now()
		if err := e.Validate(); !errors.Is(err, compliance.ErrInvalidEvidence) {
			t.Errorf("Validate() = %v, want ErrInvalidEvidence", err)
		}
	})

	t.Run("invalid kind", func(t *testing.T) {
		t.Parallel()
		e := validEvidence(tenantID, controlID)
		e.Kind = "not_a_kind"
		e.CollectedAt = time.Now()
		if err := e.Validate(); !errors.Is(err, compliance.ErrInvalidEvidence) {
			t.Errorf("Validate() = %v, want ErrInvalidEvidence", err)
		}
	})

	t.Run("blank reference", func(t *testing.T) {
		t.Parallel()
		e := validEvidence(tenantID, controlID)
		e.Reference = "   "
		e.CollectedAt = time.Now()
		if err := e.Validate(); !errors.Is(err, compliance.ErrInvalidEvidence) {
			t.Errorf("Validate() = %v, want ErrInvalidEvidence", err)
		}
	})

	t.Run("zero collected at", func(t *testing.T) {
		t.Parallel()
		e := validEvidence(tenantID, controlID)
		if err := e.Validate(); !errors.Is(err, compliance.ErrInvalidEvidence) {
			t.Errorf("Validate() = %v, want ErrInvalidEvidence", err)
		}
	})
}

// TestEngine_RecordEvidence_RequiresKnownControl proves RecordEvidence
// rejects a ControlID that does not resolve to a catalogued Control.
func TestEngine_RecordEvidence_RequiresKnownControl(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	_, err := engine.RecordEvidence(ctxWithUser(admin), tenantID, validEvidence(tenantID, uuid.New()))
	if !errors.Is(err, compliance.ErrControlNotFound) {
		t.Fatalf("RecordEvidence() error = %v, want ErrControlNotFound", err)
	}
}

// TestEngine_RecordEvidence_RequiresManagePermission proves an auditor
// cannot record evidence.
func TestEngine_RecordEvidence_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	control := registerTestControl(t, engine, tenantID)
	auditor := auditorUser(tenantID)

	_, err := engine.RecordEvidence(ctxWithUser(auditor), tenantID, validEvidence(tenantID, control.ID))
	if !errors.Is(err, compliance.ErrForbidden) {
		t.Fatalf("RecordEvidence() error = %v, want ErrForbidden", err)
	}
}

// TestEngine_RecordEvidence_SucceedsAndIsListable exercises task 5's
// full write+read round trip.
func TestEngine_RecordEvidence_SucceedsAndIsListable(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	control := registerTestControl(t, engine, tenantID)
	admin := adminUser(tenantID)

	recorded, err := engine.RecordEvidence(ctxWithUser(admin), tenantID, validEvidence(tenantID, control.ID))
	if err != nil {
		t.Fatalf("RecordEvidence: %v", err)
	}
	if recorded.CollectedBy != admin.ID {
		t.Fatalf("recorded.CollectedBy = %v, want %v", recorded.CollectedBy, admin.ID)
	}
	if recorded.CollectedAt.IsZero() {
		t.Fatal("recorded.CollectedAt left zero, want defaulted to now")
	}

	forControl, err := engine.ListEvidenceForControl(ctxWithUser(auditorUser(tenantID)), tenantID, control.ID)
	if err != nil {
		t.Fatalf("ListEvidenceForControl: %v", err)
	}
	if len(forControl) != 1 || forControl[0].ID != recorded.ID {
		t.Fatalf("ListEvidenceForControl() = %v, want exactly the recorded evidence", forControl)
	}

	all, err := engine.ListAllEvidence(ctxWithUser(auditorUser(tenantID)), tenantID)
	if err != nil {
		t.Fatalf("ListAllEvidence: %v", err)
	}
	if len(all) != 1 || all[0].ID != recorded.ID {
		t.Fatalf("ListAllEvidence() = %v, want exactly the recorded evidence", all)
	}
}

// TestEngine_RecordEvidence_CrossTenantRejected proves an admin
// authenticated against tenant A can never record evidence scoped to
// tenant B.
func TestEngine_RecordEvidence_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	control := registerTestControl(t, engine, tenantA)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.RecordEvidence(ctxWithUser(adminA), tenantB, validEvidence(tenantB, control.ID))
	if !errors.Is(err, compliance.ErrCrossTenantAccess) {
		t.Fatalf("RecordEvidence() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}
