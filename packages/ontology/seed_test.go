package ontology_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestSeedCoreConcepts_ProducesConceptsPerCategory(t *testing.T) {
	taxonomy := category.NewDefaultTaxonomy("US-CA")

	concepts := ontology.SeedCoreConcepts(taxonomy)
	if len(concepts) == 0 {
		t.Fatalf("expected at least one seeded concept")
	}

	byCategory := map[category.CategoryCode]int{}
	for _, c := range concepts {
		for _, code := range c.CategoryCodes {
			byCategory[category.CategoryCode(code)]++
		}
		if c.ID == "" {
			t.Fatalf("seeded concept has empty ID: %+v", c)
		}
		if c.Name == "" {
			t.Fatalf("seeded concept %q has empty Name", c.ID)
		}
		if c.Description == "" {
			t.Fatalf("seeded concept %q has empty Description", c.ID)
		}
	}

	wantCategories := []category.CategoryCode{
		category.CodeCivil,
		category.CodeCriminal,
		category.CodeDomesticViolence,
		category.CodeConsumer,
		category.CodeFamily,
		category.CodeCommercial,
		category.CodeLabor,
	}
	for _, code := range wantCategories {
		count := byCategory[code]
		if count < 3 || count > 5 {
			t.Fatalf("category %q: expected 3-5 seeded concepts, got %d", code, count)
		}
	}

	// "other" has no seed data and should contribute nothing.
	if byCategory[category.CodeOther] != 0 {
		t.Fatalf("expected no concepts seeded for %q, got %d", category.CodeOther, byCategory[category.CodeOther])
	}
}

func TestSeedCoreConcepts_OmitsAbsentCategories(t *testing.T) {
	taxonomy := category.Taxonomy{
		"US-CA": {
			category.CodeCivil: {Code: category.CodeCivil, Name: "Civil"},
		},
	}

	concepts := ontology.SeedCoreConcepts(taxonomy)
	for _, c := range concepts {
		if !c.HasCategory(string(category.CodeCivil)) {
			t.Fatalf("unexpected concept for non-civil category: %+v", c)
		}
	}
}

func TestSeedCoreConcepts_EmptyTaxonomyProducesNoConcepts(t *testing.T) {
	concepts := ontology.SeedCoreConcepts(category.Taxonomy{})
	if len(concepts) != 0 {
		t.Fatalf("expected no concepts for empty taxonomy, got %d", len(concepts))
	}
}
