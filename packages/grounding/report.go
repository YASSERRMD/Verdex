package grounding

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/citation"
)

// HasCritical reports whether r contains at least one SeverityCritical
// Finding, counting both Findings and CitationFindings.
func (r ConclusionResult) HasCritical() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityCritical {
			return true
		}
	}
	for _, f := range r.CitationFindings {
		if f.Severity == citation.SeverityCritical {
			return true
		}
	}
	return false
}

// AllCitationFindings flattens every ConclusionResult.CitationFindings in
// r into a single slice, in conclusion order.
func (r Report) AllCitationFindings() []citation.Finding {
	var out []citation.Finding
	for _, c := range r.Conclusions {
		out = append(out, c.CitationFindings...)
	}
	return out
}

// HasCritical reports whether r contains at least one SeverityCritical
// Finding, in either Findings or any ConclusionResult.CitationFindings.
func (r Report) HasCritical() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityCritical {
			return true
		}
	}
	for _, f := range r.AllCitationFindings() {
		if f.Severity == citation.SeverityCritical {
			return true
		}
	}
	return false
}

// CountBySeverity returns the number of Findings in r with the given
// severity, counting both Findings and every ConclusionResult's
// CitationFindings (mapped onto this package's Severity values by string
// equality, since both enums share the same three string values).
func (r Report) CountBySeverity(s Severity) int {
	count := 0
	for _, f := range r.Findings {
		if f.Severity == s {
			count++
		}
	}
	for _, f := range r.AllCitationFindings() {
		if string(f.Severity) == string(s) {
			count++
		}
	}
	return count
}

// Summary returns a short human-readable count of this Report's Findings
// by severity, e.g. "3 critical, 2 warning, 0 info (5 total)". Counts
// both Findings and every ConclusionResult's CitationFindings.
func (r Report) Summary() string {
	critical := r.CountBySeverity(SeverityCritical)
	warning := r.CountBySeverity(SeverityWarning)
	info := r.CountBySeverity(SeverityInfo)
	total := critical + warning + info
	return fmt.Sprintf("%d critical, %d warning, %d info (%d total)", critical, warning, info, total)
}
