package pii

// PIICategory classifies the kind of personally identifiable information a
// PIIMatch represents. Categorization drives redaction labeling
// (see redact.go), jurisdiction-specific sensitivity overrides
// (see jurisdiction_rules.go), and storage-boundary policy decisions
// (see policy.go).
type PIICategory string

const (
	// CategoryName covers person names: full names, and heuristically
	// detected capitalized name sequences, with or without honorifics.
	CategoryName PIICategory = "name"

	// CategoryContact covers direct contact channels: email addresses and
	// phone numbers.
	CategoryContact PIICategory = "contact"

	// CategoryIdentifier covers government- or institution-issued
	// identifier numbers: national IDs, passport numbers, social security
	// numbers, and similar long numeric/alphanumeric identifiers.
	CategoryIdentifier PIICategory = "identifier"

	// CategoryAddress covers physical/postal addresses.
	CategoryAddress PIICategory = "address"

	// CategoryFinancial covers financial account identifiers: bank account
	// numbers, card numbers, IBANs, and similar.
	CategoryFinancial PIICategory = "financial"

	// CategoryOther covers PII that does not fit a more specific category
	// above, or PII detected by a pluggable Detector implementation that
	// does not map cleanly onto the categories above.
	CategoryOther PIICategory = "other"
)

// categoryByPattern maps a RuleBasedDetector Pattern name (see detector.go)
// to the PIICategory it represents. Used by ClassifyMatches to attach a
// Category to matches produced by the default detector.
var categoryByPattern = map[string]PIICategory{
	"email":                 CategoryContact,
	"phone":                 CategoryContact,
	"national_id":           CategoryIdentifier,
	"address":               CategoryAddress,
	"person_name_heuristic": CategoryName,
}

// ClassifyMatches attaches a PIICategory to every match in matches whose
// Category is currently unset (the zero value ""), based on its Pattern.
// Matches produced by a pluggable Detector with an unrecognized Pattern (or
// no Pattern) are classified as CategoryOther. Matches that already carry a
// non-empty Category are left unchanged, so a Detector implementation may
// classify its own matches directly and skip this step. Returns a new
// slice; does not mutate matches.
func ClassifyMatches(matches []PIIMatch) []PIIMatch {
	out := make([]PIIMatch, len(matches))
	for i, m := range matches {
		if m.Category == "" {
			if cat, ok := categoryByPattern[m.Pattern]; ok {
				m.Category = cat
			} else {
				m.Category = CategoryOther
			}
		}
		out[i] = m
	}
	return out
}
