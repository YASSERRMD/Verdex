package bulkimport_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

func TestDefaultValidator_PassesWellFormedRecord(t *testing.T) {
	t.Parallel()
	v := bulkimport.DefaultValidator{}
	rec := bulkimport.ImportRecord{
		PayloadRef:   "ref-1",
		CaseNumber:   "CASE-1",
		Jurisdiction: "dubai-courts",
		PartyNames:   []string{"Jane Doe"},
	}
	if errs := v.Validate(rec); len(errs) != 0 {
		t.Fatalf("Validate() on well-formed record = %+v, want none", errs)
	}
}

func TestDefaultValidator_RequiredFieldsMissing(t *testing.T) {
	t.Parallel()
	v := bulkimport.DefaultValidator{}
	rec := bulkimport.ImportRecord{}

	errs := v.Validate(rec)
	if len(errs) < 3 {
		t.Fatalf("Validate() on empty record returned %d errors, want at least 3 (case_number, jurisdiction, payload_ref)", len(errs))
	}

	fields := make(map[string]bool, len(errs))
	for _, e := range errs {
		fields[e.Field] = true
		if e.Reason == "" {
			t.Errorf("ValidationError for field %q has empty Reason", e.Field)
		}
	}
	for _, want := range []string{"case_number", "jurisdiction", "payload_ref"} {
		if !fields[want] {
			t.Errorf("Validate() missing expected error for field %q; got %+v", want, errs)
		}
	}
}

func TestDefaultValidator_RequirePartyNames(t *testing.T) {
	t.Parallel()
	v := bulkimport.DefaultValidator{RequirePartyNames: true}
	rec := bulkimport.ImportRecord{
		PayloadRef:   "ref-1",
		CaseNumber:   "CASE-1",
		Jurisdiction: "dubai-courts",
	}
	errs := v.Validate(rec)
	found := false
	for _, e := range errs {
		if e.Field == "party_names" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Validate() with RequirePartyNames=true and no parties = %+v, want a party_names error", errs)
	}

	// Without RequirePartyNames, the same record should not flag
	// missing party names.
	v2 := bulkimport.DefaultValidator{}
	errs2 := v2.Validate(rec)
	for _, e := range errs2 {
		if e.Field == "party_names" {
			t.Fatalf("Validate() with RequirePartyNames=false flagged party_names: %+v", errs2)
		}
	}
}

func TestDefaultValidator_BlankPartyNameEntry(t *testing.T) {
	t.Parallel()
	v := bulkimport.DefaultValidator{}
	rec := bulkimport.ImportRecord{
		PayloadRef:   "ref-1",
		CaseNumber:   "CASE-1",
		Jurisdiction: "dubai-courts",
		PartyNames:   []string{"Jane Doe", "   "},
	}
	errs := v.Validate(rec)
	found := false
	for _, e := range errs {
		if e.Field == "party_names" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Validate() with a blank party name entry = %+v, want a party_names error", errs)
	}
}
