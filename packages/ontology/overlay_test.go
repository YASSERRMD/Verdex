package ontology_test

import (
	"reflect"
	"testing"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestJurisdictionOverlay_AddConcept(t *testing.T) {
	overlay := ontology.NewJurisdictionOverlay("US-CA")
	overlay.AddConcept(ontology.Concept{ID: "civil:negligence", Name: "Negligence (CA)"})

	if len(overlay.Concepts) != 1 {
		t.Fatalf("expected 1 concept in overlay, got %d", len(overlay.Concepts))
	}
	if overlay.Concepts["civil:negligence"].Name != "Negligence (CA)" {
		t.Fatalf("unexpected concept stored: %+v", overlay.Concepts["civil:negligence"])
	}
}

func TestMergeOverlay_DoesNotClobberUntouchedCoreConcepts(t *testing.T) {
	taxonomy := category.NewDefaultTaxonomy("US-CA")
	core := ontology.SeedCoreConcepts(taxonomy)

	overlay := ontology.NewJurisdictionOverlay("US-CA")
	overlay.AddConcept(ontology.Concept{
		ID:          "family:custody",
		Name:        "Custody (CA overlay)",
		Description: "California-specific custody standard.",
	})

	merged := ontology.MergeOverlay(core, overlay)

	if len(merged) != len(core) {
		t.Fatalf("expected merge to only replace/add, got %d concepts vs %d core", len(merged), len(core))
	}

	// Every core concept other than the overlaid one must be untouched.
	coreByID := map[string]ontology.Concept{}
	for _, c := range core {
		coreByID[c.ID] = c
	}
	for _, c := range merged {
		if c.ID == "family:custody" {
			continue
		}
		want, ok := coreByID[c.ID]
		if !ok {
			t.Fatalf("merged concept %q not present in core", c.ID)
		}
		if !reflect.DeepEqual(c, want) {
			t.Fatalf("core concept %q was unexpectedly modified: got %+v, want %+v", c.ID, c, want)
		}
	}

	// The overlaid concept must reflect the overlay's version.
	found := false
	for _, c := range merged {
		if c.ID == "family:custody" {
			found = true
			if c.Name != "Custody (CA overlay)" {
				t.Fatalf("overlay did not replace concept: got Name %q", c.Name)
			}
		}
	}
	if !found {
		t.Fatalf("expected overlaid concept %q in merged result", "family:custody")
	}
}

func TestMergeOverlay_AddsNewConcepts(t *testing.T) {
	core := []ontology.Concept{
		{ID: "civil:negligence", Name: "Negligence"},
	}
	overlay := ontology.NewJurisdictionOverlay("AE-DXB")
	overlay.AddConcept(ontology.Concept{ID: "civil:sharia-compensation", Name: "Sharia-based Compensation"})

	merged := ontology.MergeOverlay(core, overlay)

	if len(merged) != 2 {
		t.Fatalf("expected 2 concepts after merge, got %d", len(merged))
	}

	var foundNew bool
	for _, c := range merged {
		if c.ID == "civil:sharia-compensation" {
			foundNew = true
		}
	}
	if !foundNew {
		t.Fatalf("expected new overlay concept to be appended")
	}
}

func TestMergeOverlay_EmptyOverlayReturnsCoreUnchanged(t *testing.T) {
	core := []ontology.Concept{
		{ID: "civil:negligence", Name: "Negligence"},
	}
	overlay := ontology.NewJurisdictionOverlay("US-CA")

	merged := ontology.MergeOverlay(core, overlay)
	if len(merged) != 1 || !reflect.DeepEqual(merged[0], core[0]) {
		t.Fatalf("expected core unchanged, got %+v", merged)
	}
}
