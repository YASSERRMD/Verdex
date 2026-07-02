package agentframework_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/agentframework"
)

func TestNewScratchpad_EmptyCaseID_ReturnsError(t *testing.T) {
	_, err := agentframework.NewScratchpad("", "tenant-1")
	if !errors.Is(err, agentframework.ErrEmptyCaseID) {
		t.Fatalf("NewScratchpad() error = %v, want ErrEmptyCaseID", err)
	}
}

func TestScratchpad_AppendStep_AccumulatesStepsAndObservations(t *testing.T) {
	pad, err := agentframework.NewScratchpad("case-1", "")
	if err != nil {
		t.Fatalf("NewScratchpad() error = %v, want nil", err)
	}

	pad.AppendStep(agentframework.Step{
		Index: 0,
		Observations: []agentframework.Observation{
			{Call: agentframework.ToolCall{Name: "t1"}, Result: agentframework.ToolResult{Content: "r1"}},
		},
	})
	pad.AppendStep(agentframework.Step{
		Index: 1,
		Observations: []agentframework.Observation{
			{Call: agentframework.ToolCall{Name: "t2"}, Result: agentframework.ToolResult{Content: "r2"}},
		},
	})

	if got := pad.StepCount(); got != 2 {
		t.Fatalf("StepCount() = %d, want 2", got)
	}
	steps := pad.Steps()
	if len(steps) != 2 || steps[0].Index != 0 || steps[1].Index != 1 {
		t.Fatalf("Steps() = %+v, want two steps in order", steps)
	}
	obs := pad.Observations()
	if len(obs) != 2 || obs[0].Call.Name != "t1" || obs[1].Call.Name != "t2" {
		t.Fatalf("Observations() = %+v, want flattened in call order", obs)
	}
}

func TestScratchpad_Steps_ReturnsCopyNotAliasingInternalSlice(t *testing.T) {
	pad, err := agentframework.NewScratchpad("case-1", "")
	if err != nil {
		t.Fatalf("NewScratchpad() error = %v, want nil", err)
	}
	pad.AppendStep(agentframework.Step{Index: 0})

	steps := pad.Steps()
	steps[0].Index = 99 // mutate the copy

	fresh := pad.Steps()
	if fresh[0].Index != 0 {
		t.Fatalf("Steps()[0].Index = %d after external mutation, want 0 (internal state must not alias)", fresh[0].Index)
	}
}

func TestScratchpad_Notes_SetAndGet(t *testing.T) {
	pad, err := agentframework.NewScratchpad("case-1", "")
	if err != nil {
		t.Fatalf("NewScratchpad() error = %v, want nil", err)
	}

	if _, ok := pad.Note("summary"); ok {
		t.Fatal("Note() ok = true before SetNote, want false")
	}

	pad.SetNote("summary", "first draft")
	pad.SetNote("summary", "revised draft")

	v, ok := pad.Note("summary")
	if !ok || v != "revised draft" {
		t.Fatalf("Note() = (%q, %v), want (%q, true)", v, ok, "revised draft")
	}

	notes := pad.Notes()
	if len(notes) != 1 || notes["summary"] != "revised draft" {
		t.Fatalf("Notes() = %+v, want map with one revised entry", notes)
	}
}

func TestScratchpad_CaseIDAndTenantID(t *testing.T) {
	pad, err := agentframework.NewScratchpad("case-42", "tenant-7")
	if err != nil {
		t.Fatalf("NewScratchpad() error = %v, want nil", err)
	}
	if pad.CaseID() != "case-42" {
		t.Fatalf("CaseID() = %q, want %q", pad.CaseID(), "case-42")
	}
	if pad.TenantID() != "tenant-7" {
		t.Fatalf("TenantID() = %q, want %q", pad.TenantID(), "tenant-7")
	}
}

func TestStep_Duration(t *testing.T) {
	pad, err := agentframework.NewScratchpad("case-1", "")
	if err != nil {
		t.Fatalf("NewScratchpad() error = %v, want nil", err)
	}
	_ = pad
	// Duration is a thin wrapper over EndedAt.Sub(StartedAt); exercised
	// indirectly via Runner tests. This test just confirms zero-value
	// steps produce a zero duration without panicking.
	var s agentframework.Step
	if d := s.Duration(); d != 0 {
		t.Fatalf("zero-value Step.Duration() = %v, want 0", d)
	}
}
