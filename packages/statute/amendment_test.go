package statute

import (
	"testing"
	"time"
)

func TestAmendmentRecord_AddAmendmentAndSortHistory(t *testing.T) {
	rec := NewAmendmentRecord("rule-1")
	newer := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	older := time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)

	rec = rec.AddAmendment(Amendment{PriorText: "newer text", EffectiveDate: newer})
	rec = rec.AddAmendment(Amendment{PriorText: "older text", EffectiveDate: older})
	rec = rec.SortHistory()

	if len(rec.History) != 2 {
		t.Fatalf("len(History) = %d, want 2", len(rec.History))
	}
	if !rec.History[0].EffectiveDate.Equal(older) {
		t.Errorf("History[0].EffectiveDate = %v, want %v (oldest first)", rec.History[0].EffectiveDate, older)
	}
	if !rec.History[1].EffectiveDate.Equal(newer) {
		t.Errorf("History[1].EffectiveDate = %v, want %v", rec.History[1].EffectiveDate, newer)
	}
}

func TestAmendmentRecord_WithEffectiveDate(t *testing.T) {
	rec := NewAmendmentRecord("rule-1")
	if rec.EffectiveDate != nil {
		t.Fatal("EffectiveDate should start nil")
	}
	date := time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)
	rec = rec.WithEffectiveDate(date)
	if rec.EffectiveDate == nil || !rec.EffectiveDate.Equal(date) {
		t.Errorf("EffectiveDate = %v, want %v", rec.EffectiveDate, date)
	}
}

func TestAmendmentRecord_SupersessionChain(t *testing.T) {
	records := map[string]AmendmentRecord{
		"rule-1": NewAmendmentRecord("rule-1").SupersedeBy("rule-2"),
		"rule-2": NewAmendmentRecord("rule-2").SupersedeBy("rule-3"),
		"rule-3": NewAmendmentRecord("rule-3"), // current, not superseded
	}

	chain, cycle := SupersessionChain(records, "rule-1")
	if cycle {
		t.Fatal("cycle = true, want false")
	}
	want := []string{"rule-1", "rule-2", "rule-3"}
	if len(chain) != len(want) {
		t.Fatalf("chain = %v, want %v", chain, want)
	}
	for i := range want {
		if chain[i] != want[i] {
			t.Errorf("chain[%d] = %q, want %q", i, chain[i], want[i])
		}
	}

	if !records["rule-1"].IsSuperseded() {
		t.Error("rule-1 should be superseded")
	}
	if records["rule-3"].IsSuperseded() {
		t.Error("rule-3 should not be superseded")
	}
}

func TestAmendmentRecord_SupersessionChain_CycleDetected(t *testing.T) {
	records := map[string]AmendmentRecord{
		"rule-1": NewAmendmentRecord("rule-1").SupersedeBy("rule-2"),
		"rule-2": NewAmendmentRecord("rule-2").SupersedeBy("rule-1"), // cycle
	}
	chain, cycle := SupersessionChain(records, "rule-1")
	if !cycle {
		t.Fatal("cycle = false, want true")
	}
	if len(chain) == 0 {
		t.Error("chain should not be empty even when a cycle is detected")
	}
}

func TestAmendmentRecord_SupersessionChain_UnresolvableTarget(t *testing.T) {
	records := map[string]AmendmentRecord{
		"rule-1": NewAmendmentRecord("rule-1").SupersedeBy("rule-missing"),
	}
	chain, cycle := SupersessionChain(records, "rule-1")
	if cycle {
		t.Fatal("cycle = true, want false")
	}
	want := []string{"rule-1", "rule-missing"}
	if len(chain) != len(want) || chain[len(chain)-1] != "rule-missing" {
		t.Errorf("chain = %v, want %v", chain, want)
	}
}

func TestApplyAmendments(t *testing.T) {
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{CaseID: "statute:AE"})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}
	tagged := TagRules(built, TagOptions{CategoryCode: "civil"})

	records := map[string]AmendmentRecord{
		tagged[0].Node.ID: NewAmendmentRecord(tagged[0].Node.ID).SupersedeBy("rule-x"),
	}
	amended := ApplyAmendments(tagged, records)
	if len(amended) != len(tagged) {
		t.Fatalf("len(amended) = %d, want %d", len(amended), len(tagged))
	}
	if !amended[0].Amendment.IsSuperseded() {
		t.Error("amended[0] should be superseded")
	}
	// Rules with no matching record get a fresh empty AmendmentRecord.
	if amended[1].Amendment.IsSuperseded() {
		t.Error("amended[1] should not be superseded (no record supplied)")
	}
	if amended[1].Amendment.RuleID != tagged[1].Node.ID {
		t.Errorf("amended[1].Amendment.RuleID = %q, want %q", amended[1].Amendment.RuleID, tagged[1].Node.ID)
	}
}
