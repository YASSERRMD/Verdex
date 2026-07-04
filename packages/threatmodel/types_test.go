package threatmodel_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

func TestStrideCategory_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    threatmodel.StrideCategory
		want bool
	}{
		{"spoofing", threatmodel.StrideSpoofing, true},
		{"tampering", threatmodel.StrideTampering, true},
		{"repudiation", threatmodel.StrideRepudiation, true},
		{"information disclosure", threatmodel.StrideInformationDisclosure, true},
		{"denial of service", threatmodel.StrideDenialOfService, true},
		{"elevation of privilege", threatmodel.StrideElevationOfPrivilege, true},
		{"unknown category", threatmodel.StrideCategory("not_a_category"), false},
		{"blank", threatmodel.StrideCategory(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.c.IsValid(); got != tt.want {
				t.Errorf("StrideCategory(%q).IsValid() = %v, want %v", tt.c, got, tt.want)
			}
		})
	}
}

func TestSeverity_IsValid(t *testing.T) {
	t.Parallel()

	if !threatmodel.SeverityCritical.IsValid() {
		t.Error("SeverityCritical.IsValid() = false, want true")
	}
	if threatmodel.Severity("extreme").IsValid() {
		t.Error("unknown Severity.IsValid() = true, want false")
	}
}

func TestMitigationStatus_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    threatmodel.MitigationStatus
		want bool
	}{
		{"planned", threatmodel.MitigationPlanned, true},
		{"implemented", threatmodel.MitigationImplemented, true},
		{"verified", threatmodel.MitigationVerified, true},
		{"unknown", threatmodel.MitigationStatus("done"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.s.IsValid(); got != tt.want {
				t.Errorf("MitigationStatus(%q).IsValid() = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestCanTransitionMitigation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from threatmodel.MitigationStatus
		to   threatmodel.MitigationStatus
		want bool
	}{
		{"planned to implemented", threatmodel.MitigationPlanned, threatmodel.MitigationImplemented, true},
		{"implemented to verified", threatmodel.MitigationImplemented, threatmodel.MitigationVerified, true},
		{"implemented back to planned (regression)", threatmodel.MitigationImplemented, threatmodel.MitigationPlanned, true},
		{"planned directly to verified is not allowed", threatmodel.MitigationPlanned, threatmodel.MitigationVerified, false},
		{"verified is terminal - cannot move to implemented directly", threatmodel.MitigationVerified, threatmodel.MitigationImplemented, false},
		{"verified is terminal - cannot move to planned directly", threatmodel.MitigationVerified, threatmodel.MitigationPlanned, false},
		{"same status is not a transition", threatmodel.MitigationPlanned, threatmodel.MitigationPlanned, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := threatmodel.CanTransitionMitigation(tt.from, tt.to); got != tt.want {
				t.Errorf("CanTransitionMitigation(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func validMitigation() threatmodel.Mitigation {
	return threatmodel.Mitigation{
		Title:        "Test mitigation",
		Description:  "A mitigation used in tests.",
		Status:       threatmodel.MitigationPlanned,
		ReferenceTag: "packages/identity.RequirePermission",
	}
}

func TestMitigation_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid mitigation passes", func(t *testing.T) {
		t.Parallel()
		if err := validMitigation().Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("blank title", func(t *testing.T) {
		t.Parallel()
		m := validMitigation()
		m.Title = "   "
		if err := m.Validate(); !errors.Is(err, threatmodel.ErrInvalidMitigation) {
			t.Errorf("Validate() = %v, want ErrInvalidMitigation", err)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		t.Parallel()
		m := validMitigation()
		m.Status = "bogus"
		if err := m.Validate(); !errors.Is(err, threatmodel.ErrInvalidMitigation) {
			t.Errorf("Validate() = %v, want ErrInvalidMitigation", err)
		}
	})

	t.Run("blank reference tag", func(t *testing.T) {
		t.Parallel()
		m := validMitigation()
		m.ReferenceTag = ""
		if err := m.Validate(); !errors.Is(err, threatmodel.ErrInvalidMitigation) {
			t.Errorf("Validate() = %v, want ErrInvalidMitigation", err)
		}
	})
}

func validThreat() threatmodel.Threat {
	return threatmodel.Threat{
		Title:       "Test threat",
		Description: "A threat used in tests.",
		Category:    threatmodel.StrideSpoofing,
		Severity:    threatmodel.SeverityMedium,
	}
}

func TestThreat_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid threat with no mitigations passes", func(t *testing.T) {
		t.Parallel()
		if err := validThreat().Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("valid threat with mitigations passes", func(t *testing.T) {
		t.Parallel()
		th := validThreat()
		th.Mitigations = []threatmodel.Mitigation{validMitigation()}
		if err := th.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("blank title", func(t *testing.T) {
		t.Parallel()
		th := validThreat()
		th.Title = ""
		if err := th.Validate(); !errors.Is(err, threatmodel.ErrInvalidThreat) {
			t.Errorf("Validate() = %v, want ErrInvalidThreat", err)
		}
	})

	t.Run("invalid category", func(t *testing.T) {
		t.Parallel()
		th := validThreat()
		th.Category = "bogus"
		if err := th.Validate(); !errors.Is(err, threatmodel.ErrInvalidThreat) {
			t.Errorf("Validate() = %v, want ErrInvalidThreat", err)
		}
	})

	t.Run("invalid severity", func(t *testing.T) {
		t.Parallel()
		th := validThreat()
		th.Severity = "bogus"
		if err := th.Validate(); !errors.Is(err, threatmodel.ErrInvalidThreat) {
			t.Errorf("Validate() = %v, want ErrInvalidThreat", err)
		}
	})

	t.Run("invalid nested mitigation propagates error", func(t *testing.T) {
		t.Parallel()
		th := validThreat()
		badMitigation := validMitigation()
		badMitigation.Title = ""
		th.Mitigations = []threatmodel.Mitigation{badMitigation}
		if err := th.Validate(); !errors.Is(err, threatmodel.ErrInvalidMitigation) {
			t.Errorf("Validate() = %v, want ErrInvalidMitigation", err)
		}
	})
}

func TestThreat_StrongestMitigationStatus(t *testing.T) {
	t.Parallel()

	t.Run("no mitigations returns false", func(t *testing.T) {
		t.Parallel()
		th := validThreat()
		_, ok := th.StrongestMitigationStatus()
		if ok {
			t.Error("StrongestMitigationStatus() ok = true, want false for threat with no mitigations")
		}
	})

	t.Run("returns highest ranked status among mitigations", func(t *testing.T) {
		t.Parallel()
		th := validThreat()
		planned := validMitigation()
		planned.Status = threatmodel.MitigationPlanned
		verified := validMitigation()
		verified.Status = threatmodel.MitigationVerified
		implemented := validMitigation()
		implemented.Status = threatmodel.MitigationImplemented
		th.Mitigations = []threatmodel.Mitigation{planned, implemented, verified}

		got, ok := th.StrongestMitigationStatus()
		if !ok {
			t.Fatal("StrongestMitigationStatus() ok = false, want true")
		}
		if got != threatmodel.MitigationVerified {
			t.Errorf("StrongestMitigationStatus() = %v, want MitigationVerified", got)
		}
	})
}

func validComponent() threatmodel.Component {
	return threatmodel.Component{
		Name:        "test-component",
		PackageTag:  "packages/test",
		Description: "A component used in tests.",
	}
}

func TestThreatModel_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid threat model passes", func(t *testing.T) {
		t.Parallel()
		tm := threatmodel.ThreatModel{
			Component: validComponent(),
			Threats:   []threatmodel.Threat{validThreat()},
		}
		if err := tm.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("invalid component", func(t *testing.T) {
		t.Parallel()
		tm := threatmodel.ThreatModel{
			Component: threatmodel.Component{},
			Threats:   []threatmodel.Threat{validThreat()},
		}
		if err := tm.Validate(); !errors.Is(err, threatmodel.ErrInvalidComponent) {
			t.Errorf("Validate() = %v, want ErrInvalidComponent", err)
		}
	})

	t.Run("zero threats is invalid", func(t *testing.T) {
		t.Parallel()
		tm := threatmodel.ThreatModel{
			Component: validComponent(),
			Threats:   nil,
		}
		if err := tm.Validate(); !errors.Is(err, threatmodel.ErrInvalidThreatModel) {
			t.Errorf("Validate() = %v, want ErrInvalidThreatModel", err)
		}
	})

	t.Run("invalid nested threat propagates error", func(t *testing.T) {
		t.Parallel()
		badThreat := validThreat()
		badThreat.Title = ""
		tm := threatmodel.ThreatModel{
			Component: validComponent(),
			Threats:   []threatmodel.Threat{badThreat},
		}
		if err := tm.Validate(); !errors.Is(err, threatmodel.ErrInvalidThreat) {
			t.Errorf("Validate() = %v, want ErrInvalidThreat", err)
		}
	})
}

func TestThreatModel_UnmitigatedThreats(t *testing.T) {
	t.Parallel()

	mitigated := validThreat()
	mitigated.Title = "mitigated threat"
	mitigated.Mitigations = []threatmodel.Mitigation{validMitigation()}

	unmitigated := validThreat()
	unmitigated.Title = "unmitigated threat"

	tm := threatmodel.ThreatModel{
		Component: validComponent(),
		Threats:   []threatmodel.Threat{mitigated, unmitigated},
	}

	got := tm.UnmitigatedThreats()
	if len(got) != 1 {
		t.Fatalf("UnmitigatedThreats() returned %d threats, want 1", len(got))
	}
	if got[0].Title != "unmitigated threat" {
		t.Errorf("UnmitigatedThreats()[0].Title = %q, want %q", got[0].Title, "unmitigated threat")
	}
}

func TestThreatModel_ThreatsBySeverity(t *testing.T) {
	t.Parallel()

	critical := validThreat()
	critical.Title = "critical threat"
	critical.Severity = threatmodel.SeverityCritical

	low := validThreat()
	low.Title = "low threat"
	low.Severity = threatmodel.SeverityLow

	tm := threatmodel.ThreatModel{
		Component: validComponent(),
		Threats:   []threatmodel.Threat{critical, low},
	}

	got := tm.ThreatsBySeverity(threatmodel.SeverityCritical)
	if len(got) != 1 || got[0].Title != "critical threat" {
		t.Errorf("ThreatsBySeverity(SeverityCritical) = %v, want exactly [critical threat]", got)
	}
}

func TestThreatModel_ThreatsBySeverityDesc(t *testing.T) {
	t.Parallel()

	low := validThreat()
	low.Title = "low"
	low.Severity = threatmodel.SeverityLow

	critical := validThreat()
	critical.Title = "critical"
	critical.Severity = threatmodel.SeverityCritical

	medium := validThreat()
	medium.Title = "medium"
	medium.Severity = threatmodel.SeverityMedium

	high := validThreat()
	high.Title = "high"
	high.Severity = threatmodel.SeverityHigh

	tm := threatmodel.ThreatModel{
		Component: validComponent(),
		// Declared out of severity order, on purpose.
		Threats: []threatmodel.Threat{low, critical, medium, high},
	}

	got := tm.ThreatsBySeverityDesc()
	wantOrder := []string{"critical", "high", "medium", "low"}
	if len(got) != len(wantOrder) {
		t.Fatalf("ThreatsBySeverityDesc() returned %d threats, want %d", len(got), len(wantOrder))
	}
	for i, title := range wantOrder {
		if got[i].Title != title {
			t.Errorf("ThreatsBySeverityDesc()[%d].Title = %q, want %q", i, got[i].Title, title)
		}
	}

	// The original slice must be left untouched (a defensive copy, not
	// an in-place sort).
	if tm.Threats[0].Title != "low" {
		t.Errorf("ThreatsBySeverityDesc() mutated tm.Threats in place; tm.Threats[0].Title = %q, want %q", tm.Threats[0].Title, "low")
	}
}

func TestSeedThreatModels(t *testing.T) {
	t.Parallel()

	models := threatmodel.SeedThreatModels()
	if len(models) < 3 {
		t.Fatalf("SeedThreatModels() returned %d models, want at least 3", len(models))
	}

	wantComponents := map[string]bool{
		"gateway":                 false,
		"ingestion":               false,
		"reasoning-orchestration": false,
	}
	for _, tm := range models {
		if err := tm.Validate(); err != nil {
			t.Errorf("SeedThreatModels() model %q failed Validate(): %v", tm.Component.Name, err)
		}
		if tm.ID == uuid.Nil {
			t.Errorf("SeedThreatModels() model %q has zero ID after AllocateIDs", tm.Component.Name)
		}
		if _, ok := wantComponents[tm.Component.Name]; ok {
			wantComponents[tm.Component.Name] = true
		}
		for _, th := range tm.Threats {
			if th.ID == uuid.Nil {
				t.Errorf("SeedThreatModels() model %q has a threat with zero ID", tm.Component.Name)
			}
			for _, m := range th.Mitigations {
				if m.ID == uuid.Nil {
					t.Errorf("SeedThreatModels() model %q threat %q has a mitigation with zero ID", tm.Component.Name, th.Title)
				}
				if m.ReferenceTag == "" {
					t.Errorf("SeedThreatModels() model %q threat %q has a mitigation with blank ReferenceTag", tm.Component.Name, th.Title)
				}
			}
		}
	}
	for name, found := range wantComponents {
		if !found {
			t.Errorf("SeedThreatModels() missing expected component %q", name)
		}
	}
}

func TestAllocateIDs_Idempotent(t *testing.T) {
	t.Parallel()

	tm := threatmodel.ThreatModel{
		Component: validComponent(),
		Threats:   []threatmodel.Threat{validThreat()},
	}
	threatmodel.AllocateIDs(&tm)
	firstID := tm.ID
	firstThreatID := tm.Threats[0].ID

	threatmodel.AllocateIDs(&tm)
	if tm.ID != firstID {
		t.Errorf("AllocateIDs() second call changed ThreatModel.ID from %v to %v", firstID, tm.ID)
	}
	if tm.Threats[0].ID != firstThreatID {
		t.Errorf("AllocateIDs() second call changed Threat.ID from %v to %v", firstThreatID, tm.Threats[0].ID)
	}
}
