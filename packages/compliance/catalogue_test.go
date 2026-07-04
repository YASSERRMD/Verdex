package compliance_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/compliance"
)

func TestInMemoryControlRepository_DuplicateCodeRejected(t *testing.T) {
	t.Parallel()
	repo := compliance.NewInMemoryControlRepository()

	c1 := &compliance.Control{ID: uuid.New(), Code: "DUP-01", Title: "First", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	if err := repo.Create(t.Context(), c1); err != nil {
		t.Fatalf("Create (first): %v", err)
	}

	c2 := &compliance.Control{ID: uuid.New(), Code: "DUP-01", Title: "Second", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	if err := repo.Create(t.Context(), c2); !errors.Is(err, compliance.ErrDuplicateControl) {
		t.Fatalf("Create (duplicate code) error = %v, want ErrDuplicateControl", err)
	}
}

func TestInMemoryControlRepository_GetByCode(t *testing.T) {
	t.Parallel()
	repo := compliance.NewInMemoryControlRepository()
	c := &compliance.Control{ID: uuid.New(), Code: "GET-01", Title: "Gettable", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	if err := repo.Create(t.Context(), c); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByCode(t.Context(), "GET-01")
	if err != nil {
		t.Fatalf("GetByCode: %v", err)
	}
	if got.ID != c.ID {
		t.Fatalf("GetByCode().ID = %v, want %v", got.ID, c.ID)
	}

	if _, err := repo.GetByCode(t.Context(), "MISSING"); !errors.Is(err, compliance.ErrControlNotFound) {
		t.Fatalf("GetByCode(missing) error = %v, want ErrControlNotFound", err)
	}
}

func TestInMemoryControlRepository_ListByFramework(t *testing.T) {
	t.Parallel()
	repo := compliance.NewInMemoryControlRepository()
	uae := &compliance.Control{ID: uuid.New(), Code: "F-01", Title: "UAE", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	jrh := &compliance.Control{ID: uuid.New(), Code: "F-02", Title: "JRH", Framework: compliance.FrameworkJudicialRecordsHandling, Category: compliance.CategoryRecordRetention}
	if err := repo.Create(t.Context(), uae); err != nil {
		t.Fatalf("Create (uae): %v", err)
	}
	if err := repo.Create(t.Context(), jrh); err != nil {
		t.Fatalf("Create (jrh): %v", err)
	}

	list, err := repo.ListByFramework(t.Context(), compliance.FrameworkUAEDataProtection)
	if err != nil {
		t.Fatalf("ListByFramework: %v", err)
	}
	if len(list) != 1 || list[0].ID != uae.ID {
		t.Fatalf("ListByFramework(UAE) = %v, want exactly the UAE control", list)
	}
}

func TestInMemoryControlRepository_Update(t *testing.T) {
	t.Parallel()
	repo := compliance.NewInMemoryControlRepository()
	c := &compliance.Control{ID: uuid.New(), Code: "UPD-01", Title: "Original", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	if err := repo.Create(t.Context(), c); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated := *c
	updated.Title = "Updated title"
	updated.Description = "now has a description"
	if err := repo.Update(t.Context(), &updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.Get(t.Context(), c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Updated title" || got.Description != "now has a description" {
		t.Fatalf("Get() after Update = %+v, want updated title/description", got)
	}
}

func TestInMemoryControlRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	repo := compliance.NewInMemoryControlRepository()
	missing := &compliance.Control{ID: uuid.New(), Code: "MISSING-01", Title: "Missing", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	if err := repo.Update(t.Context(), missing); !errors.Is(err, compliance.ErrControlNotFound) {
		t.Fatalf("Update(missing) error = %v, want ErrControlNotFound", err)
	}
}

// TestEngine_RegisterControl_RequiresManagePermission proves an
// auditor (view-only) cannot register a new Control.
func TestEngine_RegisterControl_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID)

	_, err := engine.RegisterControl(ctxWithUser(auditor), validControl())
	if !errors.Is(err, compliance.ErrForbidden) {
		t.Fatalf("RegisterControl() error = %v, want ErrForbidden", err)
	}
}

// TestEngine_RegisterControl_Unauthenticated proves an anonymous
// context is rejected.
func TestEngine_RegisterControl_Unauthenticated(t *testing.T) {
	t.Parallel()
	engine, _ := newTestEngine(t)

	_, err := engine.RegisterControl(t.Context(), validControl())
	if !errors.Is(err, compliance.ErrUnauthenticated) {
		t.Fatalf("RegisterControl() error = %v, want ErrUnauthenticated", err)
	}
}

// TestEngine_RegisterControl_AdminSucceedsAndIsListable proves an
// admin can register a control and immediately see it via
// ListControls, exercising task 1's write+read round trip.
func TestEngine_RegisterControl_AdminSucceedsAndIsListable(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	registered, err := engine.RegisterControl(ctxWithUser(admin), validControl())
	if err != nil {
		t.Fatalf("RegisterControl: %v", err)
	}
	if registered.ID == (compliance.Control{}).ID {
		t.Fatal("RegisterControl() left ID zero")
	}
	if registered.CreatedBy != admin.ID {
		t.Fatalf("registered.CreatedBy = %v, want %v", registered.CreatedBy, admin.ID)
	}

	list, err := engine.ListControls(ctxWithUser(auditorUser(tenantID)))
	if err != nil {
		t.Fatalf("ListControls: %v", err)
	}
	if len(list) != 1 || list[0].ID != registered.ID {
		t.Fatalf("ListControls() = %v, want exactly the registered control", list)
	}
}

// TestEngine_RegisterControl_DuplicateCodeRejected proves the Engine
// surfaces ErrDuplicateControl rather than silently overwriting.
func TestEngine_RegisterControl_DuplicateCodeRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	if _, err := engine.RegisterControl(ctxWithUser(admin), validControl()); err != nil {
		t.Fatalf("RegisterControl (first): %v", err)
	}
	if _, err := engine.RegisterControl(ctxWithUser(admin), validControl()); !errors.Is(err, compliance.ErrDuplicateControl) {
		t.Fatalf("RegisterControl (duplicate) error = %v, want ErrDuplicateControl", err)
	}
}

// TestEngine_ListControlsByFramework proves task 1's read-side
// filtering by framework works through the Engine.
func TestEngine_ListControlsByFramework(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	for _, c := range compliance.SeedControls() {
		if _, err := engine.RegisterControl(ctxWithUser(admin), c); err != nil {
			t.Fatalf("RegisterControl(%s): %v", c.Code, err)
		}
	}

	uaeControls, err := engine.ListControlsByFramework(ctxWithUser(admin), compliance.FrameworkUAEDataProtection)
	if err != nil {
		t.Fatalf("ListControlsByFramework: %v", err)
	}
	if len(uaeControls) != 7 {
		t.Fatalf("len(uaeControls) = %d, want 7", len(uaeControls))
	}
	for _, c := range uaeControls {
		if c.Framework != compliance.FrameworkUAEDataProtection {
			t.Fatalf("ListControlsByFramework leaked a control from framework %q", c.Framework)
		}
	}
}

// TestSeedControls_AllValidAndUnique proves every seeded starter
// Control passes structural validation and has a unique Code -- a
// regression guard against a copy-paste duplicate slipping into the
// catalogue.
func TestSeedControls_AllValidAndUnique(t *testing.T) {
	t.Parallel()
	seeds := compliance.SeedControls()
	if len(seeds) == 0 {
		t.Fatal("SeedControls() returned no controls")
	}

	seenCodes := make(map[string]bool)
	for _, c := range seeds {
		c := c
		if err := c.Validate(); err != nil {
			t.Errorf("seed control %q failed Validate(): %v", c.Code, err)
		}
		if seenCodes[c.Code] {
			t.Errorf("duplicate seed control code %q", c.Code)
		}
		seenCodes[c.Code] = true
	}
}

// TestSeedControls_CoversAllThreeFrameworks proves the starter
// catalogue actually covers UAE data protection, judicial records
// handling, and the international overlay (tasks 2-4), not just one
// of the three.
func TestSeedControls_CoversAllThreeFrameworks(t *testing.T) {
	t.Parallel()
	seeds := compliance.SeedControls()

	seen := map[compliance.Framework]int{}
	for _, c := range seeds {
		seen[c.Framework]++
	}

	for _, fw := range []compliance.Framework{
		compliance.FrameworkUAEDataProtection,
		compliance.FrameworkJudicialRecordsHandling,
		compliance.FrameworkInternationalDataProtection,
	} {
		if seen[fw] == 0 {
			t.Errorf("SeedControls() has no controls for framework %q", fw)
		}
	}
}
