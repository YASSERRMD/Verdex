package ontology_test

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestNewInitialVersion(t *testing.T) {
	now := time.Now()
	v := ontology.NewInitialVersion(now)

	if v.VersionNumber != 1 {
		t.Fatalf("VersionNumber = %d, want 1", v.VersionNumber)
	}
	if !v.IsInitial() {
		t.Fatalf("expected IsInitial() = true")
	}
	if v.ParentVersion != nil {
		t.Fatalf("expected nil ParentVersion, got %v", *v.ParentVersion)
	}
}

func TestNextVersion(t *testing.T) {
	now := time.Now()
	v1 := ontology.NewInitialVersion(now)
	v2 := ontology.NextVersion(v1, now.Add(time.Minute))

	if v2.VersionNumber != 2 {
		t.Fatalf("VersionNumber = %d, want 2", v2.VersionNumber)
	}
	if v2.IsInitial() {
		t.Fatalf("expected IsInitial() = false for v2")
	}
	if v2.ParentVersion == nil || *v2.ParentVersion != 1 {
		t.Fatalf("expected ParentVersion 1, got %v", v2.ParentVersion)
	}
}

func TestOntologyVersion_IsValidSuccessorOf(t *testing.T) {
	now := time.Now()
	v1 := ontology.NewInitialVersion(now)
	v2 := ontology.NextVersion(v1, now.Add(time.Minute))
	v3 := ontology.NextVersion(v2, now.Add(2*time.Minute))

	if !v2.IsValidSuccessorOf(v1) {
		t.Fatalf("expected v2 to be a valid successor of v1")
	}
	if v3.IsValidSuccessorOf(v1) {
		t.Fatalf("expected v3 to NOT be a valid direct successor of v1 (skips v2)")
	}
	if v1.IsValidSuccessorOf(v1) {
		t.Fatalf("expected v1 to not be its own successor")
	}
}
