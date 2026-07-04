package alerting_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/alerting"
)

func TestRunbookStep_Validate(t *testing.T) {
	t.Parallel()

	valid := alerting.RunbookStep{Order: 1, Description: "do the thing", OwnerRole: "on-call engineer"}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed step = %v, want nil", err)
	}

	cases := []alerting.RunbookStep{
		{Order: 0, Description: "d", OwnerRole: "r"},
		{Order: -1, Description: "d", OwnerRole: "r"},
		{Order: 1, Description: "", OwnerRole: "r"},
		{Order: 1, Description: "d", OwnerRole: ""},
	}
	for i, c := range cases {
		if err := c.Validate(); !errors.Is(err, alerting.ErrInvalidRunbook) {
			t.Errorf("case %d: Validate() = %v, want ErrInvalidRunbook", i, err)
		}
	}
}

func TestRunbook_Validate(t *testing.T) {
	t.Parallel()

	valid := alerting.Runbook{
		Name: "test-runbook",
		Steps: []alerting.RunbookStep{
			{Order: 1, Description: "first", OwnerRole: "on-call engineer"},
			{Order: 2, Description: "second", OwnerRole: "incident commander"},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed runbook = %v, want nil", err)
	}

	noName := alerting.Runbook{Steps: valid.Steps}
	if err := noName.Validate(); !errors.Is(err, alerting.ErrInvalidRunbook) {
		t.Errorf("Validate() with blank Name = %v, want ErrInvalidRunbook", err)
	}

	noSteps := alerting.Runbook{Name: "empty"}
	if err := noSteps.Validate(); !errors.Is(err, alerting.ErrInvalidRunbook) {
		t.Errorf("Validate() with no steps = %v, want ErrInvalidRunbook", err)
	}

	outOfOrder := alerting.Runbook{
		Name: "bad-order",
		Steps: []alerting.RunbookStep{
			{Order: 2, Description: "first", OwnerRole: "r"},
			{Order: 1, Description: "second", OwnerRole: "r"},
		},
	}
	if err := outOfOrder.Validate(); !errors.Is(err, alerting.ErrInvalidRunbook) {
		t.Errorf("Validate() with out-of-order steps = %v, want ErrInvalidRunbook", err)
	}

	duplicateOrder := alerting.Runbook{
		Name: "dup-order",
		Steps: []alerting.RunbookStep{
			{Order: 1, Description: "first", OwnerRole: "r"},
			{Order: 1, Description: "second", OwnerRole: "r"},
		},
	}
	if err := duplicateOrder.Validate(); !errors.Is(err, alerting.ErrInvalidRunbook) {
		t.Errorf("Validate() with duplicate order values = %v, want ErrInvalidRunbook", err)
	}
}

func TestRunbook_StepCountAndOwnerRoles(t *testing.T) {
	t.Parallel()
	rb := alerting.Runbook{
		Name: "test",
		Steps: []alerting.RunbookStep{
			{Order: 1, Description: "a", OwnerRole: "on-call engineer"},
			{Order: 2, Description: "b", OwnerRole: "incident commander"},
			{Order: 3, Description: "c", OwnerRole: "on-call engineer"},
		},
	}
	if got := rb.StepCount(); got != 3 {
		t.Errorf("StepCount() = %d, want 3", got)
	}
	roles := rb.OwnerRoles()
	if len(roles) != 2 {
		t.Fatalf("len(OwnerRoles()) = %d, want 2 (distinct roles)", len(roles))
	}
	if roles[0] != "on-call engineer" || roles[1] != "incident commander" {
		t.Errorf("OwnerRoles() = %v, want [on-call engineer, incident commander] (first-seen order)", roles)
	}
}

func TestSeededRunbooks(t *testing.T) {
	t.Parallel()
	runbooks := alerting.SeededRunbooks()

	wantNames := []string{"slo-breach", "quality-regression", "cost-budget-exceeded"}
	if len(runbooks) != len(wantNames) {
		t.Fatalf("len(SeededRunbooks()) = %d, want %d", len(runbooks), len(wantNames))
	}
	for _, name := range wantNames {
		rb, ok := runbooks[name]
		if !ok {
			t.Errorf("SeededRunbooks() missing entry %q", name)
			continue
		}
		if err := rb.Validate(); err != nil {
			t.Errorf("SeededRunbooks()[%q].Validate() = %v, want nil", name, err)
		}
		if rb.StepCount() == 0 {
			t.Errorf("SeededRunbooks()[%q] has no steps", name)
		}
	}
}
