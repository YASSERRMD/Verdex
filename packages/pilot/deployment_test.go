package pilot_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

func TestEngine_ProvisionDeployment_StartsAtProvisioning(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	admin := adminUser(te.tenantID)

	d, err := te.engine.ProvisionDeployment(ctxWithUser(admin), te.tenantID, pilot.PilotDeployment{
		Name:             "Dubai Commercial Court Q3 pilot",
		JurisdictionCode: "AE-DXB-COMM",
		StartDate:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ProvisionDeployment: %v", err)
	}
	if d.Status != pilot.DeploymentStatusProvisioning {
		t.Fatalf("Status = %q, want %q", d.Status, pilot.DeploymentStatusProvisioning)
	}
	if d.ID == uuid.Nil {
		t.Fatal("ID should be assigned")
	}
	if d.TenantID != te.tenantID {
		t.Fatalf("TenantID = %v, want %v", d.TenantID, te.tenantID)
	}
}

func TestEngine_ProvisionDeployment_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	auditor := auditorUser(te.tenantID)

	_, err := te.engine.ProvisionDeployment(ctxWithUser(auditor), te.tenantID, pilot.PilotDeployment{
		Name:             "Should be rejected",
		JurisdictionCode: "AE-DXB-COMM",
		StartDate:        time.Now().UTC(),
	})
	if !errors.Is(err, pilot.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestEngine_TransitionDeployment_FollowsLifecycle(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	if d.Status != pilot.DeploymentStatusActive {
		t.Fatalf("Status = %q, want %q", d.Status, pilot.DeploymentStatusActive)
	}

	admin := adminUser(te.tenantID)
	concluded, err := te.engine.TransitionDeployment(ctxWithUser(admin), te.tenantID, d.ID, pilot.DeploymentStatusConcluded)
	if err != nil {
		t.Fatalf("TransitionDeployment (concluded): %v", err)
	}
	if concluded.Status != pilot.DeploymentStatusConcluded {
		t.Fatalf("Status = %q, want %q", concluded.Status, pilot.DeploymentStatusConcluded)
	}
	if concluded.EndDate.IsZero() {
		t.Fatal("EndDate should be set once concluded")
	}
}

func TestEngine_TransitionDeployment_RejectsIllegalSkip(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	admin := adminUser(te.tenantID)

	d, err := te.engine.ProvisionDeployment(ctxWithUser(admin), te.tenantID, pilot.PilotDeployment{
		Name:             "Test pilot",
		JurisdictionCode: "AE-DXB-COMM",
		StartDate:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ProvisionDeployment: %v", err)
	}

	// Provisioning -> Active directly (skipping CorpusOnboarding) is
	// not a legal move.
	_, err = te.engine.TransitionDeployment(ctxWithUser(admin), te.tenantID, d.ID, pilot.DeploymentStatusActive)
	if !errors.Is(err, pilot.ErrIllegalStatusTransition) {
		t.Fatalf("error = %v, want ErrIllegalStatusTransition", err)
	}
}

func TestEngine_TransitionDeployment_RejectsMoveOutOfTerminalState(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	admin := adminUser(te.tenantID)

	concluded, err := te.engine.TransitionDeployment(ctxWithUser(admin), te.tenantID, d.ID, pilot.DeploymentStatusConcluded)
	if err != nil {
		t.Fatalf("TransitionDeployment (concluded): %v", err)
	}
	if !concluded.Status.IsTerminal() {
		t.Fatal("Concluded should be terminal")
	}

	_, err = te.engine.TransitionDeployment(ctxWithUser(admin), te.tenantID, concluded.ID, pilot.DeploymentStatusActive)
	if !errors.Is(err, pilot.ErrIllegalStatusTransition) {
		t.Fatalf("error = %v, want ErrIllegalStatusTransition", err)
	}
}

func TestCanTransitionDeployment_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to pilot.DeploymentStatus
		want     bool
	}{
		{pilot.DeploymentStatusProvisioning, pilot.DeploymentStatusCorpusOnboarding, true},
		{pilot.DeploymentStatusCorpusOnboarding, pilot.DeploymentStatusActive, true},
		{pilot.DeploymentStatusActive, pilot.DeploymentStatusConcluded, true},
		{pilot.DeploymentStatusProvisioning, pilot.DeploymentStatusActive, false},
		{pilot.DeploymentStatusConcluded, pilot.DeploymentStatusProvisioning, false},
		{pilot.DeploymentStatusActive, pilot.DeploymentStatusProvisioning, false},
	}
	for _, c := range cases {
		got := pilot.CanTransitionDeployment(c.from, c.to)
		if got != c.want {
			t.Errorf("CanTransitionDeployment(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
