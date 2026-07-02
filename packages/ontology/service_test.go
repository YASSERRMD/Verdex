package ontology_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestOntologyService_Bootstrap_SeedsAndVersions(t *testing.T) {
	svc := ontology.NewOntologyService()
	taxonomy := category.NewDefaultTaxonomy("US-CA")

	concepts, version, err := svc.Bootstrap(ontology.BootstrapRequest{
		Taxonomy:  taxonomy,
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}
	if len(concepts) == 0 {
		t.Fatalf("expected concepts to be seeded")
	}
	if version.VersionNumber != 1 || !version.IsInitial() {
		t.Fatalf("expected initial version 1, got %+v", version)
	}

	stored := svc.Store.ListConcepts()
	if len(stored) != len(concepts) {
		t.Fatalf("expected all seeded concepts persisted, got %d vs %d", len(stored), len(concepts))
	}
}

func TestOntologyService_Bootstrap_WithOverlayMergesAndVersionsAgain(t *testing.T) {
	svc := ontology.NewOntologyService()
	taxonomy := category.NewDefaultTaxonomy("US-CA")

	_, v1, err := svc.Bootstrap(ontology.BootstrapRequest{Taxonomy: taxonomy, CreatedAt: time.Now()})
	if err != nil {
		t.Fatalf("first Bootstrap returned error: %v", err)
	}

	overlay := ontology.NewJurisdictionOverlay("US-CA")
	overlay.AddConcept(ontology.Concept{ID: "civil:negligence", Name: "Negligence (CA overlay)"})

	concepts, v2, err := svc.Bootstrap(ontology.BootstrapRequest{
		Taxonomy:  taxonomy,
		Overlay:   overlay,
		CreatedAt: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("second Bootstrap returned error: %v", err)
	}
	if !v2.IsValidSuccessorOf(v1) {
		t.Fatalf("expected v2 to be a valid successor of v1: v1=%+v v2=%+v", v1, v2)
	}

	var found bool
	for _, c := range concepts {
		if c.ID == "civil:negligence" {
			found = true
			if c.Name != "Negligence (CA overlay)" {
				t.Fatalf("expected overlay to replace concept name, got %q", c.Name)
			}
		}
	}
	if !found {
		t.Fatalf("expected overlaid concept present in bootstrap result")
	}
}

func TestOntologyService_Bootstrap_RejectsNilTaxonomy(t *testing.T) {
	svc := ontology.NewOntologyService()

	_, _, err := svc.Bootstrap(ontology.BootstrapRequest{})
	if !errors.Is(err, ontology.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestOntologyService_RegisterAlias_ResolveConcept(t *testing.T) {
	svc := ontology.NewOntologyService()
	taxonomy := category.NewDefaultTaxonomy("US-CA")
	if _, _, err := svc.Bootstrap(ontology.BootstrapRequest{Taxonomy: taxonomy, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	if err := svc.RegisterAlias("civil:negligence", "carelessness"); err != nil {
		t.Fatalf("RegisterAlias returned error: %v", err)
	}

	c, err := svc.ResolveConcept("carelessness")
	if err != nil {
		t.Fatalf("ResolveConcept returned error: %v", err)
	}
	if c.ID != "civil:negligence" {
		t.Fatalf("resolved concept ID = %q, want %q", c.ID, "civil:negligence")
	}

	if _, err := svc.ResolveConcept("nonexistent"); !errors.Is(err, ontology.ErrConceptNotFound) {
		t.Fatalf("expected ErrConceptNotFound, got %v", err)
	}
}

func TestOntologyService_RegisterAlias_UnknownConceptFails(t *testing.T) {
	svc := ontology.NewOntologyService()

	err := svc.RegisterAlias("nonexistent", "alias")
	if !errors.Is(err, ontology.ErrConceptNotFound) {
		t.Fatalf("expected ErrConceptNotFound, got %v", err)
	}
}

func TestOntologyService_RegisterLabel(t *testing.T) {
	svc := ontology.NewOntologyService()
	taxonomy := category.NewDefaultTaxonomy("US-CA")
	if _, _, err := svc.Bootstrap(ontology.BootstrapRequest{Taxonomy: taxonomy, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	c, err := svc.RegisterLabel("civil:negligence", "ar", "إهمال")
	if err != nil {
		t.Fatalf("RegisterLabel returned error: %v", err)
	}
	if c.Label("ar") != "إهمال" {
		t.Fatalf("Label(ar) = %q, want %q", c.Label("ar"), "إهمال")
	}

	stored, err := svc.Store.GetConcept("civil:negligence")
	if err != nil {
		t.Fatalf("GetConcept returned error: %v", err)
	}
	if stored.Label("ar") != "إهمال" {
		t.Fatalf("persisted concept label = %q, want %q", stored.Label("ar"), "إهمال")
	}
}

func TestOntologyService_LinkConceptToNode(t *testing.T) {
	svc := ontology.NewOntologyService()
	taxonomy := category.NewDefaultTaxonomy("US-CA")
	if _, _, err := svc.Bootstrap(ontology.BootstrapRequest{Taxonomy: taxonomy, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	rule := irac.NewRuleNode("rule-1", "case-1", "A duty of care is owed...", "US-CA", "common_law", time.Now(), 0.9, irac.Provenance{})

	link, err := svc.LinkConceptToNode("civil:negligence", rule, 0.85)
	if err != nil {
		t.Fatalf("LinkConceptToNode returned error: %v", err)
	}
	if link.NodeType != irac.NodeRule {
		t.Fatalf("NodeType = %q, want %q", link.NodeType, irac.NodeRule)
	}

	links := svc.LinksForConcept("civil:negligence")
	if len(links) != 1 || links[0].NodeID != "rule-1" {
		t.Fatalf("unexpected links: %+v", links)
	}

	if _, err := svc.LinkConceptToNode("nonexistent", rule, 0.5); !errors.Is(err, ontology.ErrConceptNotFound) {
		t.Fatalf("expected ErrConceptNotFound, got %v", err)
	}
}

func TestOntologyService_ConceptsByCategory(t *testing.T) {
	svc := ontology.NewOntologyService()
	taxonomy := category.NewDefaultTaxonomy("US-CA")
	if _, _, err := svc.Bootstrap(ontology.BootstrapRequest{Taxonomy: taxonomy, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	concepts := svc.ConceptsByCategory(string(category.CodeCriminal))
	if len(concepts) == 0 {
		t.Fatalf("expected criminal concepts")
	}
	for _, c := range concepts {
		if !c.HasCategory(string(category.CodeCriminal)) {
			t.Fatalf("concept %q missing criminal category: %+v", c.ID, c)
		}
	}
}
