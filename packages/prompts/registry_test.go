package prompts_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

func newTestTemplate(id string, version int, locale, legalFamily string) prompts.PromptTemplate {
	return prompts.PromptTemplate{
		ID:          id,
		Name:        "Test " + id,
		Version:     version,
		Locale:      locale,
		LegalFamily: legalFamily,
		Body:        `Hello from {{index . "name"}}.`,
		Variables: []prompts.VariableSpec{
			{Name: "name", Required: false, Sanitize: true, MaxLen: 64},
		},
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := prompts.NewRegistry()
	tmpl := newTestTemplate("test.hello", 1, "en", "common_law")

	if err := reg.Register(tmpl); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := reg.Get("test.hello", 1, "en", "common_law")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "test.hello" || got.Version != 1 {
		t.Errorf("unexpected template: %+v", got)
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := prompts.NewRegistry()

	_, err := reg.Get("nonexistent", 1, "en", "common_law")
	if !errors.Is(err, prompts.ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got: %v", err)
	}
}

func TestRegistry_VersionConflict(t *testing.T) {
	reg := prompts.NewRegistry()
	tmpl := newTestTemplate("test.conflict", 1, "en", "common_law")

	if err := reg.Register(tmpl); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := reg.Register(tmpl)
	if !errors.Is(err, prompts.ErrVersionConflict) {
		t.Errorf("expected ErrVersionConflict, got: %v", err)
	}
}

func TestRegistry_Latest_ReturnsHighestVersion(t *testing.T) {
	reg := prompts.NewRegistry()

	v1 := newTestTemplate("test.versioned", 1, "en", "common_law")
	v2 := newTestTemplate("test.versioned", 2, "en", "common_law")
	v3 := newTestTemplate("test.versioned", 3, "en", "common_law")

	for _, tmpl := range []prompts.PromptTemplate{v1, v3, v2} {
		if err := reg.Register(tmpl); err != nil {
			t.Fatalf("Register v%d failed: %v", tmpl.Version, err)
		}
	}

	got, err := reg.Latest("test.versioned", "en", "common_law")
	if err != nil {
		t.Fatalf("Latest failed: %v", err)
	}
	if got.Version != 3 {
		t.Errorf("expected latest version 3, got %d", got.Version)
	}
}

func TestRegistry_List(t *testing.T) {
	reg := prompts.NewRegistry()

	templates := []prompts.PromptTemplate{
		newTestTemplate("test.list", 1, "en", "common_law"),
		newTestTemplate("test.list", 1, "ar", "civil_law"),
		newTestTemplate("test.list2", 1, "en", "common_law"),
	}

	for _, tmpl := range templates {
		if err := reg.Register(tmpl); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	list := reg.List()
	if len(list) != 3 {
		t.Errorf("expected 3 templates, got %d", len(list))
	}
}

func TestRegistry_InvalidTemplate_MissingID(t *testing.T) {
	reg := prompts.NewRegistry()
	tmpl := prompts.PromptTemplate{
		// ID intentionally blank
		Name:    "No ID",
		Version: 1,
		Body:    "hello",
	}

	err := reg.Register(tmpl)
	if !errors.Is(err, prompts.ErrInvalidTemplate) {
		t.Errorf("expected ErrInvalidTemplate, got: %v", err)
	}
}

func TestVariantSelector_ExactMatch(t *testing.T) {
	reg := prompts.NewRegistry()
	exact := newTestTemplate("test.variant", 1, "en", "common_law")
	universal := newTestTemplate("test.variant", 1, "", "")

	_ = reg.Register(exact)
	_ = reg.Register(universal)

	vs := prompts.VariantSelector{}
	got, err := vs.SelectBest(reg, "test.variant", "en", "common_law")
	if err != nil {
		t.Fatalf("SelectBest failed: %v", err)
	}
	if got.Locale != "en" || got.LegalFamily != "common_law" {
		t.Errorf("expected exact match, got locale=%q family=%q", got.Locale, got.LegalFamily)
	}
}

func TestVariantSelector_FallbackToLocaleOnly(t *testing.T) {
	reg := prompts.NewRegistry()
	// Register locale-only (no legalFamily) and universal.
	localeOnly := newTestTemplate("test.fallback", 1, "en", "")
	universal := newTestTemplate("test.fallback", 1, "", "")

	_ = reg.Register(localeOnly)
	_ = reg.Register(universal)

	vs := prompts.VariantSelector{}
	got, err := vs.SelectBest(reg, "test.fallback", "en", "civil_law")
	if err != nil {
		t.Fatalf("SelectBest failed: %v", err)
	}
	// Should pick locale-only over universal.
	if got.Locale != "en" || got.LegalFamily != "" {
		t.Errorf("expected locale-only fallback, got locale=%q family=%q", got.Locale, got.LegalFamily)
	}
}

func TestVariantSelector_FallbackToFamilyOnly(t *testing.T) {
	reg := prompts.NewRegistry()
	familyOnly := newTestTemplate("test.familyfallback", 1, "", "common_law")
	universal := newTestTemplate("test.familyfallback", 1, "", "")

	_ = reg.Register(familyOnly)
	_ = reg.Register(universal)

	vs := prompts.VariantSelector{}
	got, err := vs.SelectBest(reg, "test.familyfallback", "fr", "common_law")
	if err != nil {
		t.Fatalf("SelectBest failed: %v", err)
	}
	if got.Locale != "" || got.LegalFamily != "common_law" {
		t.Errorf("expected family-only fallback, got locale=%q family=%q", got.Locale, got.LegalFamily)
	}
}

func TestVariantSelector_FallbackToUniversal(t *testing.T) {
	reg := prompts.NewRegistry()
	universal := newTestTemplate("test.universal", 1, "", "")
	_ = reg.Register(universal)

	vs := prompts.VariantSelector{}
	got, err := vs.SelectBest(reg, "test.universal", "zh", "mixed")
	if err != nil {
		t.Fatalf("SelectBest failed: %v", err)
	}
	if got.Locale != "" || got.LegalFamily != "" {
		t.Errorf("expected universal fallback, got locale=%q family=%q", got.Locale, got.LegalFamily)
	}
}

func TestVariantSelector_NoTemplate_Error(t *testing.T) {
	reg := prompts.NewRegistry()

	vs := prompts.VariantSelector{}
	_, err := vs.SelectBest(reg, "test.nonexistent", "en", "common_law")
	if !errors.Is(err, prompts.ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got: %v", err)
	}
}

func TestVariantSelector_HighestVersionWins(t *testing.T) {
	reg := prompts.NewRegistry()

	v1 := newTestTemplate("test.versions", 1, "", "")
	v2 := newTestTemplate("test.versions", 2, "", "")
	_ = reg.Register(v1)
	_ = reg.Register(v2)

	vs := prompts.VariantSelector{}
	got, err := vs.SelectBest(reg, "test.versions", "de", "civil_law")
	if err != nil {
		t.Fatalf("SelectBest failed: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("expected version 2, got %d", got.Version)
	}
}
