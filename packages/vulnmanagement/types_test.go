package vulnmanagement_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

func TestCanTransition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from, to vulnmanagement.Status
		want     bool
	}{
		{vulnmanagement.StatusOpen, vulnmanagement.StatusTriaged, true},
		{vulnmanagement.StatusOpen, vulnmanagement.StatusFalsePositive, true},
		{vulnmanagement.StatusOpen, vulnmanagement.StatusAcceptedRisk, true},
		{vulnmanagement.StatusOpen, vulnmanagement.StatusRemediating, false},
		{vulnmanagement.StatusOpen, vulnmanagement.StatusResolved, false},
		{vulnmanagement.StatusTriaged, vulnmanagement.StatusRemediating, true},
		{vulnmanagement.StatusTriaged, vulnmanagement.StatusOpen, false},
		{vulnmanagement.StatusRemediating, vulnmanagement.StatusResolved, true},
		{vulnmanagement.StatusRemediating, vulnmanagement.StatusTriaged, true},
		{vulnmanagement.StatusRemediating, vulnmanagement.StatusAcceptedRisk, false},
		{vulnmanagement.StatusResolved, vulnmanagement.StatusOpen, false},
		{vulnmanagement.StatusResolved, vulnmanagement.StatusTriaged, false},
		{vulnmanagement.StatusAcceptedRisk, vulnmanagement.StatusTriaged, false},
		{vulnmanagement.StatusFalsePositive, vulnmanagement.StatusOpen, false},
	}

	for _, c := range cases {
		c := c
		t.Run(string(c.from)+"_to_"+string(c.to), func(t *testing.T) {
			t.Parallel()
			if got := vulnmanagement.CanTransition(c.from, c.to); got != c.want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
			}
		})
	}
}

func TestStatus_IsTerminal(t *testing.T) {
	t.Parallel()

	terminal := []vulnmanagement.Status{
		vulnmanagement.StatusResolved, vulnmanagement.StatusAcceptedRisk, vulnmanagement.StatusFalsePositive,
	}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%s.IsTerminal() = false, want true", s)
		}
	}

	nonTerminal := []vulnmanagement.Status{
		vulnmanagement.StatusOpen, vulnmanagement.StatusTriaged, vulnmanagement.StatusRemediating,
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%s.IsTerminal() = true, want false", s)
		}
	}
}

func TestStatus_IsValid(t *testing.T) {
	t.Parallel()
	valid := []vulnmanagement.Status{
		vulnmanagement.StatusOpen, vulnmanagement.StatusTriaged, vulnmanagement.StatusRemediating,
		vulnmanagement.StatusResolved, vulnmanagement.StatusAcceptedRisk, vulnmanagement.StatusFalsePositive,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("%s.IsValid() = false, want true", s)
		}
	}
	if vulnmanagement.Status("bogus").IsValid() {
		t.Error(`Status("bogus").IsValid() = true, want false`)
	}
}

func TestSeverity_IsValid(t *testing.T) {
	t.Parallel()
	valid := []vulnmanagement.Severity{
		vulnmanagement.SeverityLow, vulnmanagement.SeverityMedium, vulnmanagement.SeverityHigh, vulnmanagement.SeverityCritical,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("%s.IsValid() = false, want true", s)
		}
	}
	if vulnmanagement.Severity("bogus").IsValid() {
		t.Error(`Severity("bogus").IsValid() = true, want false`)
	}
}

func TestScannerSource_IsValid(t *testing.T) {
	t.Parallel()
	valid := []vulnmanagement.ScannerSource{
		vulnmanagement.ScannerSourceSCA, vulnmanagement.ScannerSourceSAST,
		vulnmanagement.ScannerSourceContainer, vulnmanagement.ScannerSourceLicense,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("%s.IsValid() = false, want true", s)
		}
	}
	if vulnmanagement.ScannerSource("bogus").IsValid() {
		t.Error(`ScannerSource("bogus").IsValid() = true, want false`)
	}
}

func TestFinding_Validate(t *testing.T) {
	t.Parallel()

	base := func() vulnmanagement.Finding {
		return vulnmanagement.Finding{
			TenantID:     uuid.New(),
			Source:       vulnmanagement.ScannerSourceSCA,
			Package:      "example",
			Severity:     vulnmanagement.SeverityHigh,
			AdvisoryID:   "CVE-2024-1",
			Title:        "title",
			Status:       vulnmanagement.StatusOpen,
			DiscoveredAt: time.Now(),
		}
	}

	if err := (func() *vulnmanagement.Finding { f := base(); return &f })().Validate(); err != nil {
		t.Fatalf("valid Finding.Validate() = %v, want nil", err)
	}

	mutations := []func(*vulnmanagement.Finding){
		func(f *vulnmanagement.Finding) { f.TenantID = uuid.Nil },
		func(f *vulnmanagement.Finding) { f.Source = "bogus" },
		func(f *vulnmanagement.Finding) { f.Package = "  " },
		func(f *vulnmanagement.Finding) { f.Severity = "bogus" },
		func(f *vulnmanagement.Finding) { f.AdvisoryID = "" },
		func(f *vulnmanagement.Finding) { f.Title = "" },
		func(f *vulnmanagement.Finding) { f.Status = "bogus" },
		func(f *vulnmanagement.Finding) { f.DiscoveredAt = time.Time{} },
	}
	for i, mutate := range mutations {
		f := base()
		mutate(&f)
		if err := f.Validate(); err == nil {
			t.Errorf("mutation %d: Validate() = nil, want error", i)
		}
	}
}

func TestTriageDecision_Validate(t *testing.T) {
	t.Parallel()

	base := func() vulnmanagement.TriageDecision {
		return vulnmanagement.TriageDecision{
			TenantID:   uuid.New(),
			FindingID:  uuid.New(),
			FromStatus: vulnmanagement.StatusOpen,
			ToStatus:   vulnmanagement.StatusTriaged,
			Notes:      "reviewed and triaged",
			Actor:      uuid.New(),
			DecidedAt:  time.Now(),
		}
	}

	if err := (func() *vulnmanagement.TriageDecision { d := base(); return &d })().Validate(); err != nil {
		t.Fatalf("valid TriageDecision.Validate() = %v, want nil", err)
	}

	t.Run("blank notes", func(t *testing.T) {
		t.Parallel()
		d := base()
		d.Notes = "   "
		err := d.Validate()
		if !errors.Is(err, vulnmanagement.ErrNotesRequired) {
			t.Errorf("Validate() error = %v, want ErrNotesRequired", err)
		}
	})

	mutations := []func(*vulnmanagement.TriageDecision){
		func(d *vulnmanagement.TriageDecision) { d.TenantID = uuid.Nil },
		func(d *vulnmanagement.TriageDecision) { d.FindingID = uuid.Nil },
		func(d *vulnmanagement.TriageDecision) { d.FromStatus = "bogus" },
		func(d *vulnmanagement.TriageDecision) { d.Actor = uuid.Nil },
		func(d *vulnmanagement.TriageDecision) { d.DecidedAt = time.Time{} },
	}
	for i, mutate := range mutations {
		d := base()
		mutate(&d)
		if err := d.Validate(); err == nil {
			t.Errorf("mutation %d: Validate() = nil, want error", i)
		}
	}
}
