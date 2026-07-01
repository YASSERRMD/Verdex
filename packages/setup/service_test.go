package setup_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/setup"
)

// memRepo is a trivial in-memory [setup.Repository] for tests.
type memRepo struct {
	mu      sync.Mutex
	wizards map[uuid.UUID]*setup.SetupWizard
}

func newMemRepo() *memRepo {
	return &memRepo{wizards: make(map[uuid.UUID]*setup.SetupWizard)}
}

func (r *memRepo) Create(_ context.Context, w *setup.SetupWizard) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.wizards[w.TenantID]; exists {
		return errors.New("setup: duplicate tenant")
	}
	cp := *w
	r.wizards[w.TenantID] = &cp
	return nil
}

func (r *memRepo) GetByTenant(_ context.Context, tenantID uuid.UUID) (*setup.SetupWizard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.wizards[tenantID]
	if !ok {
		return nil, setup.ErrSetupNotFound
	}
	cp := *w
	return &cp, nil
}

func (r *memRepo) Update(_ context.Context, w *setup.SetupWizard) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.wizards[w.TenantID]; !ok {
		return setup.ErrSetupNotFound
	}
	cp := *w
	r.wizards[w.TenantID] = &cp
	return nil
}

// --------------------------------------------------------------------------
// helpers

func newService() (*setup.SetupService, uuid.UUID) {
	repo := newMemRepo()
	svc := setup.NewSetupService(repo)
	tenantID := uuid.New()
	return svc, tenantID
}

func jurisdictionID() uuid.UUID { return uuid.New() }

// runFullWizard drives a wizard through every step and returns the final
// completed wizard.
func runFullWizard(t *testing.T, svc *setup.SetupService, tenantID uuid.UUID) *setup.SetupWizard {
	t.Helper()
	ctx := context.Background()
	jid := jurisdictionID()

	if _, err := svc.StartSetup(ctx, tenantID); err != nil {
		t.Fatalf("StartSetup: %v", err)
	}
	if _, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepSelectJurisdiction(w, jid)
	}); err != nil {
		t.Fatalf("StepSelectJurisdiction: %v", err)
	}
	if _, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepSelectCourt(w, "supreme")
	}); err != nil {
		t.Fatalf("StepSelectCourt: %v", err)
	}
	if _, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepSelectLanguages(w, []string{"en", "ar"})
	}); err != nil {
		t.Fatalf("StepSelectLanguages: %v", err)
	}
	if _, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepConfigureProvider(w, setup.ProviderConfigStub{
			ProviderType: "openai",
			Endpoint:     "https://api.openai.com/v1",
			ModelID:      "gpt-4o",
		})
	}); err != nil {
		t.Fatalf("StepConfigureProvider: %v", err)
	}
	wiz, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepComplete(w)
	})
	if err != nil {
		t.Fatalf("StepComplete: %v", err)
	}
	return wiz
}

// --------------------------------------------------------------------------
// tests

func TestSetupService_StartSetup_CreatesWizard(t *testing.T) {
	t.Parallel()
	svc, tenantID := newService()

	w, err := svc.StartSetup(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("StartSetup: %v", err)
	}
	if w.State != setup.StateInProgress {
		t.Errorf("state = %q, want %q", w.State, setup.StateInProgress)
	}
	if w.TenantID != tenantID {
		t.Errorf("tenant mismatch")
	}
}

func TestSetupService_StartSetup_Idempotent(t *testing.T) {
	t.Parallel()
	svc, tenantID := newService()
	ctx := context.Background()

	w1, err := svc.StartSetup(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	w2, err := svc.StartSetup(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if w1.State != w2.State {
		t.Errorf("idempotent start: state changed from %q to %q", w1.State, w2.State)
	}
}

func TestSetupService_FullWizardFlow(t *testing.T) {
	t.Parallel()
	svc, tenantID := newService()

	wiz := runFullWizard(t, svc, tenantID)

	if wiz.State != setup.StateCompleted {
		t.Errorf("final state = %q, want %q", wiz.State, setup.StateCompleted)
	}
	if wiz.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
	if wiz.JurisdictionID == nil {
		t.Error("JurisdictionID should be set")
	}
	if wiz.CourtLevel == nil || *wiz.CourtLevel != "supreme" {
		t.Errorf("CourtLevel = %v, want supreme", wiz.CourtLevel)
	}
	if len(wiz.Languages) != 2 {
		t.Errorf("Languages len = %d, want 2", len(wiz.Languages))
	}
	if wiz.ProviderConfig == nil {
		t.Error("ProviderConfig should be set")
	}
}

func TestSetupService_IsComplete_TrueAfterCompletion(t *testing.T) {
	t.Parallel()
	svc, tenantID := newService()
	ctx := context.Background()

	runFullWizard(t, svc, tenantID)

	ok, err := svc.IsComplete(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("IsComplete should return true after StepComplete")
	}
}

func TestSetupService_IsComplete_FalseBeforeCompletion(t *testing.T) {
	t.Parallel()
	svc, tenantID := newService()
	ctx := context.Background()

	if _, err := svc.StartSetup(ctx, tenantID); err != nil {
		t.Fatal(err)
	}

	ok, err := svc.IsComplete(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("IsComplete should return false before steps complete")
	}
}

func TestSetupService_LockedWizard_RejectsStep(t *testing.T) {
	t.Parallel()
	svc, tenantID := newService()
	ctx := context.Background()

	runFullWizard(t, svc, tenantID)

	// Lock the wizard.
	if _, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepLock(w)
	}); err != nil {
		t.Fatalf("StepLock: %v", err)
	}

	// Any further step must return ErrSetupLocked.
	_, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepSelectJurisdiction(w, uuid.New())
	})
	if !errors.Is(err, setup.ErrSetupLocked) {
		t.Errorf("expected ErrSetupLocked, got %v", err)
	}
}

func TestSetupService_GetStatus_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newService()

	_, err := svc.GetStatus(context.Background(), uuid.New())
	if !errors.Is(err, setup.ErrSetupNotFound) {
		t.Errorf("expected ErrSetupNotFound, got %v", err)
	}
}

func TestSetupService_StepSelectLanguages_EmptyReturnsError(t *testing.T) {
	t.Parallel()
	svc, tenantID := newService()
	ctx := context.Background()

	if _, err := svc.StartSetup(ctx, tenantID); err != nil {
		t.Fatal(err)
	}
	// Jump to jurisdiction_selected state to be able to test court step.
	if _, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepSelectJurisdiction(w, uuid.New())
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepSelectCourt(w, "trial")
	}); err != nil {
		t.Fatal(err)
	}

	_, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepSelectLanguages(w, []string{})
	})
	if !errors.Is(err, setup.ErrMissingLanguages) {
		t.Errorf("expected ErrMissingLanguages, got %v", err)
	}
}

func TestSetupService_WizardLockedAt_SetOnLock(t *testing.T) {
	t.Parallel()
	svc, tenantID := newService()
	ctx := context.Background()

	runFullWizard(t, svc, tenantID)

	before := time.Now()
	wiz, err := svc.ApplyStep(ctx, tenantID, func(w *setup.SetupWizard) error {
		return setup.StepLock(w)
	})
	after := time.Now()

	if err != nil {
		t.Fatal(err)
	}
	if wiz.LockedAt == nil {
		t.Fatal("LockedAt should be set after locking")
	}
	if wiz.LockedAt.Before(before) || wiz.LockedAt.After(after) {
		t.Errorf("LockedAt %v not in expected range [%v, %v]", wiz.LockedAt, before, after)
	}
}
