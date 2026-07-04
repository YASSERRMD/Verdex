package securitytesting_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func validFinding() securitytesting.Finding {
	return securitytesting.Finding{
		ID:             uuid.New(),
		Title:          "example finding",
		Category:       securitytesting.CategoryAuthzBypass,
		Severity:       securitytesting.SeverityHigh,
		SourceScenario: "authz-bypass/example",
		SourceRunID:    uuid.New(),
		Status:         securitytesting.FindingOpen,
	}
}

func TestFinding_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		f := validFinding()
		if err := f.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("blank title rejected", func(t *testing.T) {
		t.Parallel()
		f := validFinding()
		f.Title = "  "
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error for blank Title")
		}
	})

	t.Run("invalid category rejected", func(t *testing.T) {
		t.Parallel()
		f := validFinding()
		f.Category = "bogus"
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error for invalid Category")
		}
	})

	t.Run("invalid severity rejected", func(t *testing.T) {
		t.Parallel()
		f := validFinding()
		f.Severity = "bogus"
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error for invalid Severity")
		}
	})

	t.Run("blank source scenario rejected", func(t *testing.T) {
		t.Parallel()
		f := validFinding()
		f.SourceScenario = ""
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error for blank SourceScenario")
		}
	})

	t.Run("risk accepted without justification rejected", func(t *testing.T) {
		t.Parallel()
		f := validFinding()
		f.Status = securitytesting.FindingRiskAccepted
		f.RiskAcceptedJustification = ""
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error for FindingRiskAccepted with blank justification")
		}
	})

	t.Run("risk accepted with justification is valid", func(t *testing.T) {
		t.Parallel()
		f := validFinding()
		f.Status = securitytesting.FindingRiskAccepted
		f.RiskAcceptedJustification = "compensating control exists in packages/dataresidency"
		if err := f.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("nil finding rejected", func(t *testing.T) {
		t.Parallel()
		var f *securitytesting.Finding
		if err := f.Validate(); err == nil {
			t.Error("Validate() on nil *Finding = nil, want error")
		}
	})
}

func TestFinding_IsOpenLike(t *testing.T) {
	t.Parallel()

	openLike := []securitytesting.FindingStatus{
		securitytesting.FindingOpen,
		securitytesting.FindingTriaged,
		securitytesting.FindingRemediationPending,
	}
	for _, status := range openLike {
		f := validFinding()
		f.Status = status
		if !f.IsOpenLike() {
			t.Errorf("Finding{Status: %v}.IsOpenLike() = false, want true", status)
		}
	}

	notOpenLike := []securitytesting.FindingStatus{
		securitytesting.FindingVerifiedFixed,
		securitytesting.FindingRiskAccepted,
	}
	for _, status := range notOpenLike {
		f := validFinding()
		f.Status = status
		f.RiskAcceptedJustification = "justification"
		if f.IsOpenLike() {
			t.Errorf("Finding{Status: %v}.IsOpenLike() = true, want false", status)
		}
	}
}

func TestFindingsBySeverityDesc(t *testing.T) {
	t.Parallel()

	low := validFinding()
	low.Severity = securitytesting.SeverityLow
	critical := validFinding()
	critical.Severity = securitytesting.SeverityCritical
	medium := validFinding()
	medium.Severity = securitytesting.SeverityMedium

	sorted := securitytesting.FindingsBySeverityDesc([]securitytesting.Finding{low, critical, medium})
	if len(sorted) != 3 {
		t.Fatalf("len(sorted) = %d, want 3", len(sorted))
	}
	if sorted[0].Severity != securitytesting.SeverityCritical {
		t.Errorf("sorted[0].Severity = %v, want SeverityCritical", sorted[0].Severity)
	}
	if sorted[1].Severity != securitytesting.SeverityMedium {
		t.Errorf("sorted[1].Severity = %v, want SeverityMedium", sorted[1].Severity)
	}
	if sorted[2].Severity != securitytesting.SeverityLow {
		t.Errorf("sorted[2].Severity = %v, want SeverityLow", sorted[2].Severity)
	}

	// Must not mutate the input slice.
	if low.Severity != securitytesting.SeverityLow {
		t.Error("FindingsBySeverityDesc mutated its input")
	}
}
