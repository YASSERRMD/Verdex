package securitytesting_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func TestSeverity_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    securitytesting.Severity
		want bool
	}{
		{"low", securitytesting.SeverityLow, true},
		{"medium", securitytesting.SeverityMedium, true},
		{"high", securitytesting.SeverityHigh, true},
		{"critical", securitytesting.SeverityCritical, true},
		{"unknown", securitytesting.Severity("apocalyptic"), false},
		{"blank", securitytesting.Severity(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.s.IsValid(); got != tt.want {
				t.Errorf("Severity(%q).IsValid() = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestCategory_IsValid(t *testing.T) {
	t.Parallel()

	valid := []securitytesting.Category{
		securitytesting.CategoryRegression,
		securitytesting.CategoryPromptInjection,
		securitytesting.CategoryDataLeakage,
		securitytesting.CategoryAuthzBypass,
		securitytesting.CategoryAbuseCase,
	}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("Category(%q).IsValid() = false, want true", c)
		}
	}
	if securitytesting.Category("unknown").IsValid() {
		t.Error("Category(\"unknown\").IsValid() = true, want false")
	}
}

func TestFindingStatus_IsValid(t *testing.T) {
	t.Parallel()

	valid := []securitytesting.FindingStatus{
		securitytesting.FindingOpen,
		securitytesting.FindingTriaged,
		securitytesting.FindingRemediationPending,
		securitytesting.FindingVerifiedFixed,
		securitytesting.FindingRiskAccepted,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("FindingStatus(%q).IsValid() = false, want true", s)
		}
	}
	if securitytesting.FindingStatus("unknown").IsValid() {
		t.Error("FindingStatus(\"unknown\").IsValid() = true, want false")
	}
}

func TestFindingStatus_IsTerminal(t *testing.T) {
	t.Parallel()

	terminal := []securitytesting.FindingStatus{securitytesting.FindingVerifiedFixed, securitytesting.FindingRiskAccepted}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("FindingStatus(%q).IsTerminal() = false, want true", s)
		}
	}
	nonTerminal := []securitytesting.FindingStatus{securitytesting.FindingOpen, securitytesting.FindingTriaged, securitytesting.FindingRemediationPending}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("FindingStatus(%q).IsTerminal() = true, want false", s)
		}
	}
}

func TestCanTransitionFinding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from securitytesting.FindingStatus
		to   securitytesting.FindingStatus
		want bool
	}{
		{"open to triaged", securitytesting.FindingOpen, securitytesting.FindingTriaged, true},
		{"open to risk accepted", securitytesting.FindingOpen, securitytesting.FindingRiskAccepted, true},
		{"open directly to verified fixed is illegal", securitytesting.FindingOpen, securitytesting.FindingVerifiedFixed, false},
		{"triaged to remediation pending", securitytesting.FindingTriaged, securitytesting.FindingRemediationPending, true},
		{"remediation pending to verified fixed", securitytesting.FindingRemediationPending, securitytesting.FindingVerifiedFixed, true},
		{"remediation pending back to open (re-broke)", securitytesting.FindingRemediationPending, securitytesting.FindingOpen, true},
		{"verified fixed is terminal", securitytesting.FindingVerifiedFixed, securitytesting.FindingOpen, false},
		{"risk accepted is terminal", securitytesting.FindingRiskAccepted, securitytesting.FindingOpen, false},
		{"triaged back to open is not a defined transition", securitytesting.FindingTriaged, securitytesting.FindingOpen, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := securitytesting.CanTransitionFinding(tt.from, tt.to); got != tt.want {
				t.Errorf("CanTransitionFinding(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestOutcome_IsValid(t *testing.T) {
	t.Parallel()

	for _, o := range []securitytesting.Outcome{securitytesting.OutcomePassed, securitytesting.OutcomeFailed, securitytesting.OutcomeError} {
		if !o.IsValid() {
			t.Errorf("Outcome(%q).IsValid() = false, want true", o)
		}
	}
	if securitytesting.Outcome("unknown").IsValid() {
		t.Error("Outcome(\"unknown\").IsValid() = true, want false")
	}
}

func TestResult_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		r := securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: "all good"}
		if err := r.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
	t.Run("blank detail rejected", func(t *testing.T) {
		t.Parallel()
		r := securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: ""}
		if err := r.Validate(); err == nil {
			t.Error("Validate() = nil, want error for blank Detail")
		}
	})
	t.Run("invalid outcome rejected", func(t *testing.T) {
		t.Parallel()
		r := securitytesting.Result{Outcome: securitytesting.Outcome("bogus"), Detail: "x"}
		if err := r.Validate(); err == nil {
			t.Error("Validate() = nil, want error for invalid Outcome")
		}
	})
}

func TestRunRecord_Validate(t *testing.T) {
	t.Parallel()

	valid := securitytesting.RunRecord{
		ScenarioName:     "example",
		ScenarioCategory: securitytesting.CategoryRegression,
		Result:           securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: "ok"},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}

	blankName := valid
	blankName.ScenarioName = ""
	if err := blankName.Validate(); err == nil {
		t.Error("Validate() = nil, want error for blank ScenarioName")
	}

	badCategory := valid
	badCategory.ScenarioCategory = "bogus"
	if err := badCategory.Validate(); err == nil {
		t.Error("Validate() = nil, want error for invalid ScenarioCategory")
	}
}
