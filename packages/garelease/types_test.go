package garelease_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/garelease"
)

func TestIsValidSemver(t *testing.T) {
	valid := []string{"1.0.0", "1.100.0", "2.0.0-rc.1", "1.2.3+build.5", "0.0.1-alpha"}
	for _, v := range valid {
		if !garelease.IsValidSemver(v) {
			t.Errorf("IsValidSemver(%q) = false, want true", v)
		}
	}

	invalid := []string{"", "v1.0.0", "1.0", "1", "latest", "1.0.0.0", "abc"}
	for _, v := range invalid {
		if garelease.IsValidSemver(v) {
			t.Errorf("IsValidSemver(%q) = true, want false", v)
		}
	}
}

func TestIsValidCommitSHA(t *testing.T) {
	valid := []string{
		strings.Repeat("a", 40),
		strings.Repeat("f", 64),
	}
	for _, v := range valid {
		if !garelease.IsValidCommitSHA(v) {
			t.Errorf("IsValidCommitSHA(%q) = false, want true", v)
		}
	}

	invalid := []string{"", "abc123", strings.Repeat("A", 40), strings.Repeat("z", 40), strings.Repeat("a", 39)}
	for _, v := range invalid {
		if garelease.IsValidCommitSHA(v) {
			t.Errorf("IsValidCommitSHA(%q) = true, want false", v)
		}
	}
}

func validReadinessCheck() garelease.ReadinessCheck {
	return garelease.ReadinessCheck{
		Dimension:   garelease.DimensionCriticalFindings,
		Status:      garelease.CheckPassed,
		Detail:      "none open",
		EvaluatedAt: time.Now().UTC(),
	}
}

func TestReadinessCheck_Validate(t *testing.T) {
	if err := (&garelease.ReadinessCheck{}).Validate(); err == nil {
		t.Fatalf("Validate() on zero value = nil, want error")
	}

	valid := validReadinessCheck()
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() on valid check = %v, want nil", err)
	}

	cases := []struct {
		name   string
		mutate func(*garelease.ReadinessCheck)
	}{
		{"invalid dimension", func(c *garelease.ReadinessCheck) { c.Dimension = "bogus" }},
		{"invalid status", func(c *garelease.ReadinessCheck) { c.Status = "bogus" }},
		{"failed without detail", func(c *garelease.ReadinessCheck) { c.Status = garelease.CheckFailed; c.Detail = "  " }},
		{"zero evaluated at", func(c *garelease.ReadinessCheck) { c.EvaluatedAt = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validReadinessCheck()
			tc.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Errorf("Validate() = nil, want error")
			}
		})
	}
}

func TestReadinessCheck_Validate_Nil(t *testing.T) {
	var c *garelease.ReadinessCheck
	if err := c.Validate(); !errors.Is(err, garelease.ErrInvalidReadinessCheck) {
		t.Fatalf("Validate() on nil = %v, want ErrInvalidReadinessCheck", err)
	}
}

func TestReleaseReadiness_ComputeReady(t *testing.T) {
	ready := readyReadiness()
	if !ready.Ready {
		t.Fatalf("readyReadiness().Ready = false, want true")
	}
	if len(ready.FailedChecks()) != 0 {
		t.Fatalf("readyReadiness().FailedChecks() = %v, want empty", ready.FailedChecks())
	}

	notReady := notReadyReadiness()
	if notReady.Ready {
		t.Fatalf("notReadyReadiness().Ready = true, want false")
	}
	if len(notReady.FailedChecks()) != 1 {
		t.Fatalf("notReadyReadiness().FailedChecks() = %d entries, want 1", len(notReady.FailedChecks()))
	}
}

func TestReleaseReadiness_EmptyChecksNeverReady(t *testing.T) {
	// An empty-Checks ReleaseReadiness should never present as Ready --
	// this is exercised indirectly via computeReady inside
	// CheckReadiness, but the exported ReleaseReadiness{} zero value
	// should also never be mistaken for ready by a caller inspecting it
	// directly.
	var r garelease.ReleaseReadiness
	if r.Ready {
		t.Fatalf("zero-value ReleaseReadiness.Ready = true, want false")
	}
}

func TestReleaseReadiness_CheckFor(t *testing.T) {
	r := readyReadiness()
	check, ok := r.CheckFor(garelease.DimensionPerfBudget)
	if !ok {
		t.Fatalf("CheckFor(DimensionPerfBudget) ok = false, want true")
	}
	if check.Status != garelease.CheckPassed {
		t.Errorf("CheckFor(DimensionPerfBudget).Status = %v, want CheckPassed", check.Status)
	}

	if _, ok := r.CheckFor("nonexistent_dimension"); ok {
		t.Fatalf("CheckFor(nonexistent) ok = true, want false")
	}
}

func validCandidate() garelease.ReleaseCandidate {
	return garelease.ReleaseCandidate{
		Version:   "1.100.0",
		CommitSHA: strings.Repeat("a", 40),
		Readiness: readyReadiness(),
		FrozenAt:  time.Now().UTC(),
	}
}

func TestReleaseCandidate_Validate(t *testing.T) {
	valid := validCandidate()
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() on valid candidate = %v, want nil", err)
	}

	cases := []struct {
		name   string
		mutate func(*garelease.ReleaseCandidate)
	}{
		{"invalid version", func(c *garelease.ReleaseCandidate) { c.Version = "v1" }},
		{"invalid commit sha", func(c *garelease.ReleaseCandidate) { c.CommitSHA = "abc" }},
		{"no readiness checks", func(c *garelease.ReleaseCandidate) { c.Readiness = garelease.ReleaseReadiness{} }},
		{"zero frozen at", func(c *garelease.ReleaseCandidate) { c.FrozenAt = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validCandidate()
			tc.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Errorf("Validate() = nil, want error")
			}
		})
	}
}

func TestReleaseCandidate_Validate_Nil(t *testing.T) {
	var c *garelease.ReleaseCandidate
	if err := c.Validate(); !errors.Is(err, garelease.ErrInvalidCandidate) {
		t.Fatalf("Validate() on nil = %v, want ErrInvalidCandidate", err)
	}
}

func validRelease() garelease.Release {
	return garelease.Release{
		CandidateID: mustUUID(),
		Version:     "1.100.0",
		CommitSHA:   strings.Repeat("b", 40),
		CutAt:       time.Now().UTC(),
	}
}

func TestRelease_Validate(t *testing.T) {
	valid := validRelease()
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() on valid release = %v, want nil", err)
	}

	cases := []struct {
		name   string
		mutate func(*garelease.Release)
	}{
		{"invalid version", func(r *garelease.Release) { r.Version = "not-semver" }},
		{"invalid commit sha", func(r *garelease.Release) { r.CommitSHA = "xyz" }},
		{"zero cut at", func(r *garelease.Release) { r.CutAt = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validRelease()
			tc.mutate(&r)
			if err := r.Validate(); err == nil {
				t.Errorf("Validate() = nil, want error")
			}
		})
	}
}

func TestRelease_Validate_Nil(t *testing.T) {
	var r *garelease.Release
	if err := r.Validate(); !errors.Is(err, garelease.ErrInvalidRelease) {
		t.Fatalf("Validate() on nil = %v, want ErrInvalidRelease", err)
	}
}

func TestRelease_Validate_EmptyCandidateID(t *testing.T) {
	r := validRelease()
	r.CandidateID = emptyUUID()
	if err := r.Validate(); err == nil {
		t.Fatalf("Validate() with empty CandidateID = nil, want error")
	}
}
