package treevalidation

import (
	"errors"
	"testing"
)

func TestCanFinalize(t *testing.T) {
	t.Run("no findings passes", func(t *testing.T) {
		ok, err := CanFinalize(Report{CaseID: "case-1"})
		if !ok || err != nil {
			t.Fatalf("expected (true, nil), got (%v, %v)", ok, err)
		}
	})

	t.Run("only warning/info findings passes", func(t *testing.T) {
		report := Report{
			CaseID: "case-1",
			Findings: []Finding{
				{Severity: SeverityWarning, Code: "x"},
				{Severity: SeverityInfo, Code: "y"},
			},
		}
		ok, err := CanFinalize(report)
		if !ok || err != nil {
			t.Fatalf("expected (true, nil), got (%v, %v)", ok, err)
		}
	})

	t.Run("one critical finding blocks", func(t *testing.T) {
		report := Report{
			CaseID: "case-1",
			Findings: []Finding{
				{Severity: SeverityWarning, Code: "x"},
				{Severity: SeverityCritical, Code: "y"},
			},
		}
		ok, err := CanFinalize(report)
		if ok {
			t.Fatal("expected ok=false")
		}
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(err, ErrCriticalFindings) {
			t.Errorf("expected errors.Is(err, ErrCriticalFindings), got %v", err)
		}
	})

	t.Run("multiple critical findings blocks with all counted", func(t *testing.T) {
		report := Report{
			CaseID: "case-1",
			Findings: []Finding{
				{Severity: SeverityCritical, Code: "a"},
				{Severity: SeverityCritical, Code: "b"},
				{Severity: SeverityCritical, Code: "c"},
			},
		}
		ok, err := CanFinalize(report)
		if ok {
			t.Fatal("expected ok=false")
		}
		if err == nil {
			t.Fatal("expected non-nil error")
		}
	})
}
