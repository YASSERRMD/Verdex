package irac

import (
	"testing"
	"time"
)

func TestNewInitialRevision(t *testing.T) {
	now := time.Now().UTC()
	r := NewInitialRevision("case-1", now)
	if r.RevisionNumber != 1 {
		t.Errorf("RevisionNumber = %d, want 1", r.RevisionNumber)
	}
	if r.CaseID != "case-1" {
		t.Errorf("CaseID = %q, want case-1", r.CaseID)
	}
	if !r.IsInitial() {
		t.Errorf("IsInitial() = false, want true")
	}
	if r.ParentRevision != nil {
		t.Errorf("ParentRevision = %v, want nil", r.ParentRevision)
	}
}

func TestNextRevision(t *testing.T) {
	now := time.Now().UTC()
	first := NewInitialRevision("case-1", now)
	later := now.Add(time.Hour)
	second := NextRevision(first, later)

	if second.RevisionNumber != 2 {
		t.Errorf("RevisionNumber = %d, want 2", second.RevisionNumber)
	}
	if second.CaseID != "case-1" {
		t.Errorf("CaseID = %q, want case-1", second.CaseID)
	}
	if second.IsInitial() {
		t.Errorf("IsInitial() = true, want false")
	}
	if second.ParentRevision == nil || *second.ParentRevision != 1 {
		t.Errorf("ParentRevision = %v, want pointer to 1", second.ParentRevision)
	}

	third := NextRevision(second, later.Add(time.Hour))
	if third.RevisionNumber != 3 {
		t.Errorf("RevisionNumber = %d, want 3", third.RevisionNumber)
	}
	if third.ParentRevision == nil || *third.ParentRevision != 2 {
		t.Errorf("ParentRevision = %v, want pointer to 2", third.ParentRevision)
	}
}

func TestTreeRevision_IsValidSuccessorOf(t *testing.T) {
	now := time.Now().UTC()
	first := NewInitialRevision("case-1", now)
	second := NextRevision(first, now.Add(time.Hour))

	if !second.IsValidSuccessorOf(first) {
		t.Errorf("second.IsValidSuccessorOf(first) = false, want true")
	}
	if first.IsValidSuccessorOf(second) {
		t.Errorf("first.IsValidSuccessorOf(second) = true, want false")
	}

	otherCase := NewInitialRevision("case-2", now)
	crossCaseSuccessor := NextRevision(otherCase, now.Add(time.Hour))
	if crossCaseSuccessor.IsValidSuccessorOf(first) {
		t.Errorf("successor from a different case should not validate, but did")
	}

	// A revision with no ParentRevision is never a valid successor.
	noParent := TreeRevision{RevisionNumber: 2, CaseID: "case-1", CreatedAt: now}
	if noParent.IsValidSuccessorOf(first) {
		t.Errorf("revision with nil ParentRevision should not be a valid successor")
	}

	// Skipping a revision number is invalid.
	skipped := TreeRevision{RevisionNumber: 3, CaseID: "case-1", CreatedAt: now}
	parent := 1
	skipped.ParentRevision = &parent
	if skipped.IsValidSuccessorOf(first) {
		t.Errorf("revision skipping a number should not be a valid successor")
	}
}
