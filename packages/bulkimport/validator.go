package bulkimport

import "strings"

// Validator checks a candidate ImportRecord for correctness before it
// is considered for deduplication/import, producing structured
// ValidationError values rather than a bare bool (task 3) so a corpus
// owner's error report names exactly which field failed and why.
type Validator interface {
	// Validate returns every ValidationError found in rec. A nil or
	// empty return means rec passed every check this Validator runs.
	Validate(rec ImportRecord) []ValidationError
}

// DefaultValidator is the built-in Validator this package ships:
// required-field presence and basic structural well-formedness of the
// fields RunBatch actually uses (CaseNumber, Jurisdiction, PartyNames).
// A deployment with a richer source schema can supply its own
// Validator to Engine.SetValidator/NewEngine instead.
type DefaultValidator struct {
	// RequirePartyNames, when true, additionally rejects a record with
	// no PartyNames entries. Off by default, since some historical
	// corpora (e.g. purely administrative dockets) legitimately have
	// no named parties.
	RequirePartyNames bool
}

// Validate implements Validator.
func (v DefaultValidator) Validate(rec ImportRecord) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(rec.CaseNumber) == "" {
		errs = append(errs, ValidationError{Field: "case_number", Reason: "case number is required"})
	}
	if strings.TrimSpace(rec.Jurisdiction) == "" {
		errs = append(errs, ValidationError{Field: "jurisdiction", Reason: "jurisdiction is required"})
	}
	if strings.TrimSpace(rec.PayloadRef) == "" {
		errs = append(errs, ValidationError{Field: "payload_ref", Reason: "payload reference is required"})
	}
	if v.RequirePartyNames && len(rec.PartyNames) == 0 {
		errs = append(errs, ValidationError{Field: "party_names", Reason: "at least one party name is required"})
	}
	for _, p := range rec.PartyNames {
		if strings.TrimSpace(p) == "" {
			errs = append(errs, ValidationError{Field: "party_names", Reason: "party name entry is blank"})
			break
		}
	}

	return errs
}

var _ Validator = DefaultValidator{}
