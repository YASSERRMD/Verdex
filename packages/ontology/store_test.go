package ontology_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestInMemoryOntologyStore_Concept_RoundTrip(t *testing.T) {
	store := ontology.NewInMemoryOntologyStore()
	concept := ontology.Concept{ID: "civil:negligence", Name: "Negligence"}

	if err := store.SaveConcept(concept); err != nil {
		t.Fatalf("SaveConcept returned error: %v", err)
	}

	got, err := store.GetConcept("civil:negligence")
	if err != nil {
		t.Fatalf("GetConcept returned error: %v", err)
	}
	if got.Name != "Negligence" {
		t.Fatalf("got Name %q, want %q", got.Name, "Negligence")
	}

	if _, err := store.GetConcept("nonexistent"); !errors.Is(err, ontology.ErrConceptNotFound) {
		t.Fatalf("expected ErrConceptNotFound, got %v", err)
	}

	if err := store.SaveConcept(ontology.Concept{}); !errors.Is(err, ontology.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput for empty ID, got %v", err)
	}

	list := store.ListConcepts()
	if len(list) != 1 {
		t.Fatalf("expected 1 concept in list, got %d", len(list))
	}
}

func TestInMemoryOntologyStore_Relation_RoundTrip(t *testing.T) {
	store := ontology.NewInMemoryOntologyStore()
	rel := ontology.NewRelation("civil:gross-negligence", "civil:negligence", ontology.RelIsA)

	if err := store.SaveRelation(rel); err != nil {
		t.Fatalf("SaveRelation returned error: %v", err)
	}

	relations := store.ListRelations()
	if len(relations) != 1 || relations[0] != rel {
		t.Fatalf("unexpected relations: %+v", relations)
	}

	if err := store.SaveRelation(ontology.Relation{FromConceptID: "a", ToConceptID: "b", Type: "bogus"}); !errors.Is(err, ontology.ErrInvalidRelationType) {
		t.Fatalf("expected ErrInvalidRelationType, got %v", err)
	}

	if err := store.SaveRelation(ontology.Relation{Type: ontology.RelIsA}); !errors.Is(err, ontology.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestInMemoryOntologyStore_Overlay_RoundTrip(t *testing.T) {
	store := ontology.NewInMemoryOntologyStore()
	overlay := ontology.NewJurisdictionOverlay("US-CA")
	overlay.AddConcept(ontology.Concept{ID: "civil:negligence", Name: "Negligence (CA)"})

	if err := store.SaveOverlay(overlay); err != nil {
		t.Fatalf("SaveOverlay returned error: %v", err)
	}

	got, err := store.GetOverlay("US-CA")
	if err != nil {
		t.Fatalf("GetOverlay returned error: %v", err)
	}
	if got.Concepts["civil:negligence"].Name != "Negligence (CA)" {
		t.Fatalf("unexpected overlay concept: %+v", got.Concepts["civil:negligence"])
	}

	if _, err := store.GetOverlay("unknown"); !errors.Is(err, ontology.ErrJurisdictionNotFound) {
		t.Fatalf("expected ErrJurisdictionNotFound, got %v", err)
	}
}

func TestInMemoryOntologyStore_Link_RoundTrip(t *testing.T) {
	store := ontology.NewInMemoryOntologyStore()
	link := ontology.ConceptLink{ConceptID: "civil:negligence", NodeID: "rule-1", NodeType: "rule", Confidence: 0.8}

	if err := store.SaveLink(link); err != nil {
		t.Fatalf("SaveLink returned error: %v", err)
	}

	links := store.LinksForConcept("civil:negligence")
	if len(links) != 1 || links[0] != link {
		t.Fatalf("unexpected links: %+v", links)
	}

	if err := store.SaveLink(ontology.ConceptLink{}); !errors.Is(err, ontology.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}

	if len(store.ListLinks()) != 1 {
		t.Fatalf("expected 1 link in ListLinks")
	}
}

func TestInMemoryOntologyStore_Aliases(t *testing.T) {
	store := ontology.NewInMemoryOntologyStore()

	if err := store.Aliases().AddAlias("civil:negligence", "carelessness"); err != nil {
		t.Fatalf("AddAlias returned error: %v", err)
	}

	conceptID, ok := store.Aliases().ResolveAlias("carelessness")
	if !ok || conceptID != "civil:negligence" {
		t.Fatalf("ResolveAlias = (%q, %v), want (%q, true)", conceptID, ok, "civil:negligence")
	}
}

func TestInMemoryOntologyStore_Version_RoundTrip(t *testing.T) {
	store := ontology.NewInMemoryOntologyStore()

	if _, err := store.LatestVersion(); !errors.Is(err, ontology.ErrVersionNotFound) {
		t.Fatalf("expected ErrVersionNotFound for empty store, got %v", err)
	}

	now := time.Now()
	v1 := ontology.NewInitialVersion(now)
	if err := store.SaveVersion(v1); err != nil {
		t.Fatalf("SaveVersion returned error: %v", err)
	}
	v2 := ontology.NextVersion(v1, now.Add(time.Minute))
	if err := store.SaveVersion(v2); err != nil {
		t.Fatalf("SaveVersion returned error: %v", err)
	}

	latest, err := store.LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion returned error: %v", err)
	}
	if latest.VersionNumber != 2 {
		t.Fatalf("LatestVersion = %d, want 2", latest.VersionNumber)
	}

	got, err := store.GetVersion(1)
	if err != nil {
		t.Fatalf("GetVersion(1) returned error: %v", err)
	}
	if got.VersionNumber != 1 {
		t.Fatalf("GetVersion(1).VersionNumber = %d, want 1", got.VersionNumber)
	}

	if _, err := store.GetVersion(99); !errors.Is(err, ontology.ErrVersionNotFound) {
		t.Fatalf("expected ErrVersionNotFound, got %v", err)
	}
}
