package backupdr_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/backupdr"
)

func TestRunbookStep_Validate(t *testing.T) {
	t.Parallel()

	valid := backupdr.RunbookStep{Order: 1, Description: "Do the thing.", OwnerRole: "on-call engineer"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid step: Validate() = %v, want nil", err)
	}

	tests := []struct {
		name string
		step backupdr.RunbookStep
	}{
		{"zero order", backupdr.RunbookStep{Order: 0, Description: "x", OwnerRole: "y"}},
		{"negative order", backupdr.RunbookStep{Order: -1, Description: "x", OwnerRole: "y"}},
		{"blank description", backupdr.RunbookStep{Order: 1, Description: "  ", OwnerRole: "y"}},
		{"blank owner role", backupdr.RunbookStep{Order: 1, Description: "x", OwnerRole: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.step.Validate(); !errors.Is(err, backupdr.ErrInvalidRunbook) {
				t.Fatalf("Validate() = %v, want ErrInvalidRunbook", err)
			}
		})
	}
}

func TestRunbook_Validate(t *testing.T) {
	t.Parallel()

	valid := backupdr.Runbook{
		Name: "test scenario",
		Steps: []backupdr.RunbookStep{
			{Order: 1, Description: "first", OwnerRole: "engineer"},
			{Order: 2, Description: "second", OwnerRole: "commander"},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid runbook: Validate() = %v, want nil", err)
	}

	t.Run("blank name", func(t *testing.T) {
		t.Parallel()
		rb := backupdr.Runbook{Name: "  ", Steps: valid.Steps}
		if err := rb.Validate(); !errors.Is(err, backupdr.ErrInvalidRunbook) {
			t.Fatalf("Validate() = %v, want ErrInvalidRunbook", err)
		}
	})

	t.Run("no steps", func(t *testing.T) {
		t.Parallel()
		rb := backupdr.Runbook{Name: "test", Steps: nil}
		if err := rb.Validate(); !errors.Is(err, backupdr.ErrInvalidRunbook) {
			t.Fatalf("Validate() = %v, want ErrInvalidRunbook", err)
		}
	})

	t.Run("duplicate order", func(t *testing.T) {
		t.Parallel()
		rb := backupdr.Runbook{Name: "test", Steps: []backupdr.RunbookStep{
			{Order: 1, Description: "first", OwnerRole: "engineer"},
			{Order: 1, Description: "also first", OwnerRole: "commander"},
		}}
		if err := rb.Validate(); !errors.Is(err, backupdr.ErrInvalidRunbook) {
			t.Fatalf("Validate() = %v, want ErrInvalidRunbook for duplicate order", err)
		}
	})

	t.Run("out of order steps", func(t *testing.T) {
		t.Parallel()
		rb := backupdr.Runbook{Name: "test", Steps: []backupdr.RunbookStep{
			{Order: 2, Description: "second", OwnerRole: "engineer"},
			{Order: 1, Description: "first", OwnerRole: "commander"},
		}}
		if err := rb.Validate(); !errors.Is(err, backupdr.ErrInvalidRunbook) {
			t.Fatalf("Validate() = %v, want ErrInvalidRunbook for descending order", err)
		}
	})

	t.Run("invalid step propagates", func(t *testing.T) {
		t.Parallel()
		rb := backupdr.Runbook{Name: "test", Steps: []backupdr.RunbookStep{
			{Order: 1, Description: "", OwnerRole: "engineer"},
		}}
		if err := rb.Validate(); !errors.Is(err, backupdr.ErrInvalidRunbook) {
			t.Fatalf("Validate() = %v, want ErrInvalidRunbook", err)
		}
	})
}

func TestRunbook_StepCount(t *testing.T) {
	t.Parallel()
	rb := backupdr.Runbook{Steps: []backupdr.RunbookStep{{Order: 1}, {Order: 2}, {Order: 3}}}
	if got, want := rb.StepCount(), 3; got != want {
		t.Fatalf("StepCount() = %d, want %d", got, want)
	}
}

func TestRunbook_OwnerRoles(t *testing.T) {
	t.Parallel()
	rb := backupdr.Runbook{Steps: []backupdr.RunbookStep{
		{Order: 1, OwnerRole: "engineer"},
		{Order: 2, OwnerRole: "commander"},
		{Order: 3, OwnerRole: "engineer"}, // duplicate, should not repeat
	}}
	roles := rb.OwnerRoles()
	want := []string{"engineer", "commander"}
	if len(roles) != len(want) {
		t.Fatalf("OwnerRoles() = %v, want %v", roles, want)
	}
	for i, r := range want {
		if roles[i] != r {
			t.Fatalf("OwnerRoles()[%d] = %q, want %q", i, roles[i], r)
		}
	}
}

// TestDefaultDRRunbook_IsValid proves the shipped starter runbook
// itself passes Validate -- if a future edit to DefaultDRRunbook
// breaks step ordering or leaves a blank field, this test catches it.
func TestDefaultDRRunbook_IsValid(t *testing.T) {
	t.Parallel()
	rb := backupdr.DefaultDRRunbook()
	if err := rb.Validate(); err != nil {
		t.Fatalf("DefaultDRRunbook().Validate() = %v, want nil", err)
	}
	if rb.StepCount() == 0 {
		t.Fatal("DefaultDRRunbook() has zero steps")
	}
}
