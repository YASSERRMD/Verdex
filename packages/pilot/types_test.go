package pilot_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

func TestDeploymentStatus_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    pilot.DeploymentStatus
		want bool
	}{
		{"provisioning", pilot.DeploymentStatusProvisioning, true},
		{"corpus onboarding", pilot.DeploymentStatusCorpusOnboarding, true},
		{"active", pilot.DeploymentStatusActive, true},
		{"concluded", pilot.DeploymentStatusConcluded, true},
		{"unknown", pilot.DeploymentStatus("archived"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.s.IsValid(); got != tt.want {
				t.Errorf("DeploymentStatus(%q).IsValid() = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func validDeployment() pilot.PilotDeployment {
	return pilot.PilotDeployment{
		TenantID:         uuid.New(),
		Name:             "Test pilot",
		JurisdictionCode: "AE-DXB-COMM",
		Status:           pilot.DeploymentStatusProvisioning,
		StartDate:        time.Now(),
	}
}

func TestPilotDeployment_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid deployment passes", func(t *testing.T) {
		t.Parallel()
		d := validDeployment()
		if err := d.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
	t.Run("blank name rejected", func(t *testing.T) {
		t.Parallel()
		d := validDeployment()
		d.Name = "  "
		if err := d.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
	t.Run("blank jurisdiction code rejected", func(t *testing.T) {
		t.Parallel()
		d := validDeployment()
		d.JurisdictionCode = ""
		if err := d.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
	t.Run("zero tenant id rejected", func(t *testing.T) {
		t.Parallel()
		d := validDeployment()
		d.TenantID = uuid.Nil
		if err := d.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
	t.Run("end date before start date rejected", func(t *testing.T) {
		t.Parallel()
		d := validDeployment()
		d.EndDate = d.StartDate.Add(-1 * time.Hour)
		if err := d.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
	t.Run("nil deployment rejected", func(t *testing.T) {
		t.Parallel()
		var d *pilot.PilotDeployment
		if err := d.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
}

func validPilotCase() pilot.PilotCase {
	return pilot.PilotCase{
		TenantID:         uuid.New(),
		DeploymentID:     uuid.New(),
		CaseID:           uuid.New(),
		SupervisorUserID: uuid.New(),
		AssignedAt:       time.Now(),
	}
}

func TestPilotCase_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid case passes", func(t *testing.T) {
		t.Parallel()
		c := validPilotCase()
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
	t.Run("zero case id rejected", func(t *testing.T) {
		t.Parallel()
		c := validPilotCase()
		c.CaseID = uuid.Nil
		if err := c.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
	t.Run("zero supervisor rejected", func(t *testing.T) {
		t.Parallel()
		c := validPilotCase()
		c.SupervisorUserID = uuid.Nil
		if err := c.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
}

func TestDimensionName_IsValid(t *testing.T) {
	t.Parallel()
	if !pilot.DimensionGrounding.IsValid() {
		t.Error("DimensionGrounding.IsValid() = false, want true")
	}
	if !pilot.DimensionUsefulness.IsValid() {
		t.Error("DimensionUsefulness.IsValid() = false, want true")
	}
	if pilot.DimensionName("not_a_dimension").IsValid() {
		t.Error("unknown DimensionName.IsValid() = true, want false")
	}
}

func TestTrustRating_IsValid(t *testing.T) {
	t.Parallel()
	if !pilot.TrustModerate.IsValid() {
		t.Error("TrustModerate.IsValid() = false, want true")
	}
	if pilot.TrustRating(0).IsValid() {
		t.Error("TrustRating(0).IsValid() = true, want false")
	}
	if pilot.TrustRating(6).IsValid() {
		t.Error("TrustRating(6).IsValid() = true, want false")
	}
}

func validFeedbackEntry() pilot.FeedbackEntry {
	return pilot.FeedbackEntry{
		TenantID:       uuid.New(),
		PilotCaseID:    uuid.New(),
		ReviewerUserID: uuid.New(),
		Ratings:        []pilot.DimensionRating{{Dimension: pilot.DimensionGrounding, Score: 0.5}},
		Trust:          pilot.TrustModerate,
		SubmittedAt:    time.Now(),
	}
}

func TestFeedbackEntry_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid entry passes", func(t *testing.T) {
		t.Parallel()
		f := validFeedbackEntry()
		if err := f.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
	t.Run("no ratings rejected", func(t *testing.T) {
		t.Parallel()
		f := validFeedbackEntry()
		f.Ratings = nil
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
	t.Run("out of range score rejected", func(t *testing.T) {
		t.Parallel()
		f := validFeedbackEntry()
		f.Ratings = []pilot.DimensionRating{{Dimension: pilot.DimensionGrounding, Score: -0.1}}
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
}

func validPilotFinding() pilot.PilotFinding {
	return pilot.PilotFinding{
		TenantID:          uuid.New(),
		DeploymentID:      uuid.New(),
		SourceFeedbackIDs: []uuid.UUID{uuid.New()},
		Title:             "Test finding",
		Priority:          pilot.PriorityMedium,
		Status:            pilot.FindingStatusOpen,
		DiscoveredAt:      time.Now(),
	}
}

func TestPilotFinding_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid finding passes", func(t *testing.T) {
		t.Parallel()
		f := validPilotFinding()
		if err := f.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
	t.Run("no source feedback ids rejected", func(t *testing.T) {
		t.Parallel()
		f := validPilotFinding()
		f.SourceFeedbackIDs = nil
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
	t.Run("blank title rejected", func(t *testing.T) {
		t.Parallel()
		f := validPilotFinding()
		f.Title = ""
		if err := f.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
}

func validRefinementRecord() pilot.RefinementRecord {
	return pilot.RefinementRecord{
		TenantID:    uuid.New(),
		FindingID:   uuid.New(),
		Description: "A concrete change.",
		AppliedBy:   uuid.New(),
		AppliedAt:   time.Now(),
	}
}

func TestRefinementRecord_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid record passes", func(t *testing.T) {
		t.Parallel()
		r := validRefinementRecord()
		if err := r.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
	t.Run("blank description rejected", func(t *testing.T) {
		t.Parallel()
		r := validRefinementRecord()
		r.Description = "   "
		if err := r.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
	t.Run("verified without note rejected", func(t *testing.T) {
		t.Parallel()
		r := validRefinementRecord()
		r.VerifiedFixed = true
		r.VerificationNote = ""
		if err := r.Validate(); err == nil {
			t.Error("Validate() = nil, want error")
		}
	})
}

func TestPriority_IsValid(t *testing.T) {
	t.Parallel()
	if !pilot.PriorityCritical.IsValid() {
		t.Error("PriorityCritical.IsValid() = false, want true")
	}
	if pilot.Priority("urgent").IsValid() {
		t.Error("unknown Priority.IsValid() = true, want false")
	}
}
