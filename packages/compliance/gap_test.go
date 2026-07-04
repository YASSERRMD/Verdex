package compliance_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/compliance"
)

func TestStatus_IsValidAndString(t *testing.T) {
	t.Parallel()

	for _, s := range []compliance.Status{compliance.StatusSatisfied, compliance.StatusPartiallyMet, compliance.StatusGap} {
		if !s.IsValid() {
			t.Errorf("Status(%q).IsValid() = false, want true", s)
		}
		if s.String() != string(s) {
			t.Errorf("Status(%q).String() = %q, want %q", s, s.String(), string(s))
		}
	}
	if compliance.Status("not_a_status").IsValid() {
		t.Error("unknown Status.IsValid() = true, want false")
	}
}

func TestEvaluateControl_NoEvidence_Gap(t *testing.T) {
	t.Parallel()
	control := compliance.Control{ID: uuid.New(), Code: "C-01", Title: "T", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	now := time.Now()

	result := compliance.EvaluateControl(control, nil, now)
	if result.Status != compliance.StatusGap {
		t.Fatalf("Status = %q, want %q", result.Status, compliance.StatusGap)
	}
	if result.EvidenceCount != 0 || result.EvidenceKinds != 0 {
		t.Fatalf("EvidenceCount/EvidenceKinds = %d/%d, want 0/0", result.EvidenceCount, result.EvidenceKinds)
	}
}

func TestEvaluateControl_OneEvidenceKind_PartiallyMet(t *testing.T) {
	t.Parallel()
	control := compliance.Control{ID: uuid.New(), Code: "C-01", Title: "T", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	now := time.Now()

	evidence := []compliance.ControlEvidence{
		{ControlID: control.ID, Kind: compliance.EvidenceKindTestName, Reference: "TestFoo", CollectedAt: now.Add(-time.Hour)},
		// A second record of the SAME kind must not push this over the
		// bar -- EvaluateControl counts distinct kinds, not raw record
		// count.
		{ControlID: control.ID, Kind: compliance.EvidenceKindTestName, Reference: "TestBar", CollectedAt: now.Add(-time.Minute)},
	}

	result := compliance.EvaluateControl(control, evidence, now)
	if result.Status != compliance.StatusPartiallyMet {
		t.Fatalf("Status = %q, want %q", result.Status, compliance.StatusPartiallyMet)
	}
	if result.EvidenceCount != 2 {
		t.Fatalf("EvidenceCount = %d, want 2", result.EvidenceCount)
	}
	if result.EvidenceKinds != 1 {
		t.Fatalf("EvidenceKinds = %d, want 1", result.EvidenceKinds)
	}
}

func TestEvaluateControl_TwoDistinctKinds_Satisfied(t *testing.T) {
	t.Parallel()
	control := compliance.Control{ID: uuid.New(), Code: "C-01", Title: "T", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	now := time.Now()

	evidence := []compliance.ControlEvidence{
		{ControlID: control.ID, Kind: compliance.EvidenceKindTestName, Reference: "TestFoo", CollectedAt: now.Add(-time.Hour)},
		{ControlID: control.ID, Kind: compliance.EvidenceKindAuditQuery, Reference: "kind=privacy.sar_transition", CollectedAt: now.Add(-time.Minute)},
	}

	result := compliance.EvaluateControl(control, evidence, now)
	if result.Status != compliance.StatusSatisfied {
		t.Fatalf("Status = %q, want %q", result.Status, compliance.StatusSatisfied)
	}
	if result.EvidenceKinds != 2 {
		t.Fatalf("EvidenceKinds = %d, want 2", result.EvidenceKinds)
	}
}

// TestEvaluateControl_FutureDatedEvidenceExcluded proves evidence
// collected after the evaluation instant cannot inflate a status --
// a clock-skewed or backdated record must never count.
func TestEvaluateControl_FutureDatedEvidenceExcluded(t *testing.T) {
	t.Parallel()
	control := compliance.Control{ID: uuid.New(), Code: "C-01", Title: "T", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	now := time.Now()

	evidence := []compliance.ControlEvidence{
		{ControlID: control.ID, Kind: compliance.EvidenceKindTestName, Reference: "TestFoo", CollectedAt: now.Add(24 * time.Hour)},
		{ControlID: control.ID, Kind: compliance.EvidenceKindAuditQuery, Reference: "kind=x", CollectedAt: now.Add(48 * time.Hour)},
	}

	result := compliance.EvaluateControl(control, evidence, now)
	if result.Status != compliance.StatusGap {
		t.Fatalf("Status = %q, want %q (future-dated evidence must not count)", result.Status, compliance.StatusGap)
	}
}

// TestEvaluateControl_IgnoresOtherControlsEvidence proves evidence
// for a different ControlID never leaks into this control's result.
func TestEvaluateControl_IgnoresOtherControlsEvidence(t *testing.T) {
	t.Parallel()
	control := compliance.Control{ID: uuid.New(), Code: "C-01", Title: "T", Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis}
	other := uuid.New()
	now := time.Now()

	evidence := []compliance.ControlEvidence{
		{ControlID: other, Kind: compliance.EvidenceKindTestName, Reference: "TestOther", CollectedAt: now.Add(-time.Hour)},
		{ControlID: other, Kind: compliance.EvidenceKindAuditQuery, Reference: "kind=other", CollectedAt: now.Add(-time.Minute)},
	}

	result := compliance.EvaluateControl(control, evidence, now)
	if result.Status != compliance.StatusGap {
		t.Fatalf("Status = %q, want %q (other control's evidence must not leak)", result.Status, compliance.StatusGap)
	}
}

func TestGapAnalysisReport_CountByStatusAndGaps(t *testing.T) {
	t.Parallel()
	report := compliance.GapAnalysisReport{
		Results: []compliance.ControlGapResult{
			{Control: compliance.Control{Code: "A"}, Status: compliance.StatusSatisfied},
			{Control: compliance.Control{Code: "B"}, Status: compliance.StatusGap},
			{Control: compliance.Control{Code: "C"}, Status: compliance.StatusGap},
			{Control: compliance.Control{Code: "D"}, Status: compliance.StatusPartiallyMet},
		},
	}

	counts := report.CountByStatus()
	if counts[compliance.StatusSatisfied] != 1 || counts[compliance.StatusGap] != 2 || counts[compliance.StatusPartiallyMet] != 1 {
		t.Fatalf("CountByStatus() = %v, want satisfied=1 gap=2 partially_met=1", counts)
	}

	gaps := report.Gaps()
	if len(gaps) != 2 {
		t.Fatalf("len(Gaps()) = %d, want 2", len(gaps))
	}
	for _, g := range gaps {
		if g.Status != compliance.StatusGap {
			t.Fatalf("Gaps() contained non-gap result: %+v", g)
		}
	}
}

// TestEngine_RunGapAnalysis_NoProfilePermissiveDefault proves that
// when a tenant has never set a Profile, RunGapAnalysis evaluates
// every catalogued control rather than erroring or reporting zero
// applicable controls.
func TestEngine_RunGapAnalysis_NoProfilePermissiveDefault(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	control := registerTestControl(t, engine, tenantID)

	report, err := engine.RunGapAnalysis(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("RunGapAnalysis: %v", err)
	}
	if len(report.Results) != 1 || report.Results[0].Control.ID != control.ID {
		t.Fatalf("RunGapAnalysis() = %v, want exactly the one registered control", report.Results)
	}
	if report.Results[0].Status != compliance.StatusGap {
		t.Fatalf("Status = %q, want %q (no evidence recorded yet)", report.Results[0].Status, compliance.StatusGap)
	}
}

// TestEngine_RunGapAnalysis_WithEvidence proves a full round trip:
// register a control, record two distinct kinds of evidence, run gap
// analysis, and see StatusSatisfied.
func TestEngine_RunGapAnalysis_WithEvidence(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)
	control := registerTestControl(t, engine, tenantID)

	if _, err := engine.RecordEvidence(ctxWithUser(admin), tenantID, compliance.ControlEvidence{
		ControlID: control.ID, Kind: compliance.EvidenceKindTestName, Reference: "TestFoo",
	}); err != nil {
		t.Fatalf("RecordEvidence (test): %v", err)
	}
	if _, err := engine.RecordEvidence(ctxWithUser(admin), tenantID, compliance.ControlEvidence{
		ControlID: control.ID, Kind: compliance.EvidenceKindDocument, Reference: "doc/compliance.md#lawful-basis",
	}); err != nil {
		t.Fatalf("RecordEvidence (doc): %v", err)
	}

	report, err := engine.RunGapAnalysis(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("RunGapAnalysis: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(report.Results))
	}
	if report.Results[0].Status != compliance.StatusSatisfied {
		t.Fatalf("Status = %q, want %q", report.Results[0].Status, compliance.StatusSatisfied)
	}
}

// TestEngine_RunGapAnalysis_RequiresViewPermission proves an
// unauthenticated caller cannot run a gap analysis.
func TestEngine_RunGapAnalysis_RequiresViewPermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	_, err := engine.RunGapAnalysis(t.Context(), tenantID)
	if err == nil {
		t.Fatal("RunGapAnalysis() with no actor = nil error, want an authorization error")
	}
}
