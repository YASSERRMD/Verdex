package caselifecycle_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestSetMetadata_ReplacesEntireMapAndBumpsVersion(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)
	startVersion := c.MetadataVersion

	updated, err := caselifecycle.SetMetadata(ctx, repo, caselifecycle.MetadataUpdateInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		Values:   map[string]string{"docket_number": "2026-CV-001"},
	})
	if err != nil {
		t.Fatalf("SetMetadata: %v", err)
	}
	if len(updated.Metadata) != 1 || updated.Metadata["docket_number"] != "2026-CV-001" {
		t.Fatalf("Metadata = %v, want exactly {docket_number: 2026-CV-001}", updated.Metadata)
	}
	if updated.MetadataVersion != startVersion+1 {
		t.Errorf("MetadataVersion = %d, want %d", updated.MetadataVersion, startVersion+1)
	}

	// A second SetMetadata call replaces (not merges) the map.
	updated2, err := caselifecycle.SetMetadata(ctx, repo, caselifecycle.MetadataUpdateInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		Values:   map[string]string{"court_room": "3B"},
	})
	if err != nil {
		t.Fatalf("second SetMetadata: %v", err)
	}
	if _, ok := updated2.Metadata["docket_number"]; ok {
		t.Errorf("expected docket_number to be gone after SetMetadata replace, got %v", updated2.Metadata)
	}
	if updated2.Metadata["court_room"] != "3B" {
		t.Errorf("Metadata[court_room] = %q, want %q", updated2.Metadata["court_room"], "3B")
	}
}

func TestMergeMetadata_OverlaysWithoutDroppingExistingKeys(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)

	_, err := caselifecycle.SetMetadata(ctx, repo, caselifecycle.MetadataUpdateInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		Values:   map[string]string{"docket_number": "2026-CV-001", "court_room": "1A"},
	})
	if err != nil {
		t.Fatalf("SetMetadata: %v", err)
	}

	merged, err := caselifecycle.MergeMetadata(ctx, repo, caselifecycle.MetadataUpdateInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		Values:   map[string]string{"court_room": "3B", "judge": "Hon. Doe"},
	})
	if err != nil {
		t.Fatalf("MergeMetadata: %v", err)
	}
	want := map[string]string{
		"docket_number": "2026-CV-001",
		"court_room":    "3B",
		"judge":         "Hon. Doe",
	}
	if len(merged.Metadata) != len(want) {
		t.Fatalf("Metadata = %v, want %v", merged.Metadata, want)
	}
	for k, v := range want {
		if merged.Metadata[k] != v {
			t.Errorf("Metadata[%q] = %q, want %q", k, merged.Metadata[k], v)
		}
	}
}

func TestSetMetadata_BlankKeyRejected(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)

	_, err := caselifecycle.SetMetadata(ctx, repo, caselifecycle.MetadataUpdateInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		Values:   map[string]string{"  ": "value"},
	})
	if !errors.Is(err, caselifecycle.ErrInvalidMetadataKey) {
		t.Fatalf("expected ErrInvalidMetadataKey, got %v", err)
	}
}

func TestSetMetadata_VersionConflictRejected(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)

	_, err := caselifecycle.SetMetadata(ctx, repo, caselifecycle.MetadataUpdateInput{
		TenantID:        tenantID,
		CaseID:          c.ID,
		Values:          map[string]string{"a": "1"},
		ExpectedVersion: c.MetadataVersion + 99, // deliberately wrong
	})
	if !errors.Is(err, caselifecycle.ErrMetadataVersionConflict) {
		t.Fatalf("expected ErrMetadataVersionConflict, got %v", err)
	}
}

func TestSetMetadata_CorrectExpectedVersionSucceeds(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)

	updated, err := caselifecycle.SetMetadata(ctx, repo, caselifecycle.MetadataUpdateInput{
		TenantID:        tenantID,
		CaseID:          c.ID,
		Values:          map[string]string{"a": "1"},
		ExpectedVersion: c.MetadataVersion,
	})
	if err != nil {
		t.Fatalf("SetMetadata with correct ExpectedVersion: %v", err)
	}
	if updated.Metadata["a"] != "1" {
		t.Errorf("Metadata[a] = %q, want %q", updated.Metadata["a"], "1")
	}
}

func TestGetMetadataValue_NilSafeAndPresence(t *testing.T) {
	if v, ok := caselifecycle.GetMetadataValue(nil, "x"); ok || v != "" {
		t.Errorf("GetMetadataValue(nil, x) = (%q, %v), want (\"\", false)", v, ok)
	}

	c := &caselifecycle.Case{Metadata: map[string]string{"k": "v"}}
	if v, ok := caselifecycle.GetMetadataValue(c, "k"); !ok || v != "v" {
		t.Errorf("GetMetadataValue(c, k) = (%q, %v), want (\"v\", true)", v, ok)
	}
	if _, ok := caselifecycle.GetMetadataValue(c, "missing"); ok {
		t.Error("GetMetadataValue(c, missing) ok = true, want false")
	}
}

func TestNewCase_RequiresCreatedBy(t *testing.T) {
	_, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       uuid.New(),
		JurisdictionID: uuid.New(),
		Title:          "Doe v. Acme",
	})
	if !errors.Is(err, caselifecycle.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated for missing CreatedBy, got %v", err)
	}
}

func TestNewCase_RequiresTitleAndJurisdiction(t *testing.T) {
	tenantID := uuid.New()
	actor := uuid.New()

	_, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:  tenantID,
		Title:     "Missing jurisdiction",
		CreatedBy: actor,
	})
	if !errors.Is(err, caselifecycle.ErrInvalidCase) {
		t.Fatalf("expected ErrInvalidCase for missing JurisdictionID, got %v", err)
	}

	_, err = caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantID,
		JurisdictionID: uuid.New(),
		Title:          "   ",
		CreatedBy:      actor,
	})
	if !errors.Is(err, caselifecycle.ErrInvalidCase) {
		t.Fatalf("expected ErrInvalidCase for blank title, got %v", err)
	}
}

func TestNewCase_StartsInDraftState(t *testing.T) {
	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       uuid.New(),
		JurisdictionID: uuid.New(),
		Title:          "Doe v. Acme",
		CreatedBy:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}
	if c.State != caselifecycle.StateDraft {
		t.Errorf("State = %s, want %s", c.State, caselifecycle.StateDraft)
	}
	if c.ID == uuid.Nil {
		t.Error("expected NewCase to assign a non-nil ID")
	}
	if c.MetadataVersion != 1 {
		t.Errorf("MetadataVersion = %d, want 1", c.MetadataVersion)
	}
}
