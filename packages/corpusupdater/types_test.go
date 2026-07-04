package corpusupdater_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/corpusupdater"
)

func TestCorpusTargetIsValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		target corpusupdater.CorpusTarget
		want   bool
	}{
		{corpusupdater.CorpusStatute, true},
		{corpusupdater.CorpusPrecedent, true},
		{corpusupdater.CorpusTarget("bogus"), false},
		{corpusupdater.CorpusTarget(""), false},
	}
	for _, tc := range cases {
		if got := tc.target.IsValid(); got != tc.want {
			t.Errorf("CorpusTarget(%q).IsValid() = %v, want %v", tc.target, got, tc.want)
		}
	}
}

func TestJobStatusIsValid(t *testing.T) {
	t.Parallel()
	valid := []corpusupdater.JobStatus{
		corpusupdater.StatusPending, corpusupdater.StatusValidating, corpusupdater.StatusApplying,
		corpusupdater.StatusApplied, corpusupdater.StatusFailed, corpusupdater.StatusRolledBack,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("JobStatus(%q).IsValid() = false, want true", s)
		}
	}
	if corpusupdater.JobStatus("bogus").IsValid() {
		t.Error("JobStatus(\"bogus\").IsValid() = true, want false")
	}
}

func TestIsValidTransition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		from corpusupdater.JobStatus
		to   corpusupdater.JobStatus
		want bool
	}{
		{"pending to validating", corpusupdater.StatusPending, corpusupdater.StatusValidating, true},
		{"pending to failed", corpusupdater.StatusPending, corpusupdater.StatusFailed, true},
		{"validating to applying", corpusupdater.StatusValidating, corpusupdater.StatusApplying, true},
		{"validating to failed", corpusupdater.StatusValidating, corpusupdater.StatusFailed, true},
		{"applying to applied", corpusupdater.StatusApplying, corpusupdater.StatusApplied, true},
		{"applying to failed", corpusupdater.StatusApplying, corpusupdater.StatusFailed, true},
		{"applied to rolled back", corpusupdater.StatusApplied, corpusupdater.StatusRolledBack, true},
		{"pending to applying skips validating", corpusupdater.StatusPending, corpusupdater.StatusApplying, false},
		{"pending to applied", corpusupdater.StatusPending, corpusupdater.StatusApplied, false},
		{"failed to anything", corpusupdater.StatusFailed, corpusupdater.StatusPending, false},
		{"rolled back to anything", corpusupdater.StatusRolledBack, corpusupdater.StatusApplied, false},
		{"applied to applying backwards", corpusupdater.StatusApplied, corpusupdater.StatusApplying, false},
		{"unknown from", corpusupdater.JobStatus("bogus"), corpusupdater.StatusPending, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := corpusupdater.IsValidTransition(tc.from, tc.to); got != tc.want {
				t.Errorf("IsValidTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestCorpusUpdateJobValidate(t *testing.T) {
	t.Parallel()

	valid := corpusupdater.CorpusUpdateJob{
		JurisdictionCode: "AE-DXB",
		TargetCorpus:     corpusupdater.CorpusStatute,
		Status:           corpusupdater.StatusPending,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed job = %v, want nil", err)
	}

	t.Run("nil job", func(t *testing.T) {
		t.Parallel()
		var j *corpusupdater.CorpusUpdateJob
		if err := j.Validate(); !errors.Is(err, corpusupdater.ErrInvalidJob) {
			t.Errorf("Validate() on nil = %v, want ErrInvalidJob", err)
		}
	})

	t.Run("blank jurisdiction", func(t *testing.T) {
		t.Parallel()
		j := valid
		j.JurisdictionCode = "  "
		if err := j.Validate(); !errors.Is(err, corpusupdater.ErrInvalidJob) {
			t.Errorf("Validate() with blank jurisdiction = %v, want ErrInvalidJob", err)
		}
	})

	t.Run("invalid target corpus", func(t *testing.T) {
		t.Parallel()
		j := valid
		j.TargetCorpus = corpusupdater.CorpusTarget("bogus")
		if err := j.Validate(); !errors.Is(err, corpusupdater.ErrInvalidCorpusTarget) {
			t.Errorf("Validate() with invalid corpus = %v, want ErrInvalidCorpusTarget", err)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		t.Parallel()
		j := valid
		j.Status = corpusupdater.JobStatus("bogus")
		if err := j.Validate(); !errors.Is(err, corpusupdater.ErrInvalidJob) {
			t.Errorf("Validate() with invalid status = %v, want ErrInvalidJob", err)
		}
	})
}

func TestAmendmentIsEffective(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name          string
		effectiveDate time.Time
		want          bool
	}{
		{"past date is effective", now.Add(-24 * time.Hour), true},
		{"exactly now is effective", now, true},
		{"future date is not effective", now.Add(24 * time.Hour), false},
		{"far future date is not effective", now.Add(365 * 24 * time.Hour), false},
		{"zero date is never effective", time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := corpusupdater.Amendment{EffectiveDate: tc.effectiveDate}
			if got := a.IsEffective(now); got != tc.want {
				t.Errorf("IsEffective(%v) with EffectiveDate=%v = %v, want %v", now, tc.effectiveDate, got, tc.want)
			}
		})
	}
}

func TestChangeTypeIsValid(t *testing.T) {
	t.Parallel()
	valid := []corpusupdater.ChangeType{
		corpusupdater.ChangeTypeAdd, corpusupdater.ChangeTypeAmend, corpusupdater.ChangeTypeRepeal,
	}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("ChangeType(%q).IsValid() = false, want true", c)
		}
	}
	if corpusupdater.ChangeType("bogus").IsValid() {
		t.Error("ChangeType(\"bogus\").IsValid() = true, want false")
	}
}

func TestAmendmentIDsAreDistinctPerInstance(t *testing.T) {
	t.Parallel()
	a := corpusupdater.Amendment{ID: uuid.New()}
	b := corpusupdater.Amendment{ID: uuid.New()}
	if a.ID == b.ID {
		t.Error("expected distinct UUIDs from uuid.New()")
	}
}
