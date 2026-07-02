package citation

import (
	"fmt"
	"strings"
	"sync"
)

// FormatInput is the data a Formatter needs to produce jurisdiction-
// appropriate citation text. It is deliberately a flat struct of opaque
// strings (not irac.RuleNode or a packages/jurisdiction type) so this
// package's formatting layer stays decoupled from both — mirroring how
// irac.RuleNode.JurisdictionCode/LegalFamily are themselves opaque
// strings rather than packages/jurisdiction types.
type FormatInput struct {
	// CaseName is the party-vs-party or matter name, when the citation
	// concerns a precedent (e.g. "Smith v Jones"). Empty for a statute
	// citation.
	CaseName string

	// Act is the statute/act name or number, when the citation concerns a
	// statute (e.g. "Act 12"). Empty for a precedent citation.
	Act string

	// Section is the statute section identifier (e.g. "5"), when
	// applicable.
	Section string

	// Clause is the statute clause/sub-section identifier (e.g. "a"),
	// when applicable.
	Clause string

	// RawCitation is a raw citation string already carrying
	// jurisdiction-specific reporter/year/court information (e.g.
	// "[2020] UKSC 1"), when the citation concerns a precedent.
	RawCitation string

	// Origin identifies whether this input describes a statute or
	// precedent citation, letting a Formatter pick the right template
	// without inspecting which fields happen to be populated.
	Origin Origin
}

// Formatter formats a FormatInput into jurisdiction- or legal-family-
// appropriate citation text. Implementations are pure functions of their
// input: a Formatter must not perform I/O or depend on package-level
// mutable state.
type Formatter interface {
	// Format returns the formatted citation string for in.
	Format(in FormatInput) string
}

// FormatterFunc adapts a plain function to the Formatter interface,
// mirroring the standard library's http.HandlerFunc convention.
type FormatterFunc func(in FormatInput) string

// Format implements Formatter.
func (f FormatterFunc) Format(in FormatInput) string {
	return f(in)
}

// CommonLawFormatter formats citations in common-law case-citation style:
// "<CaseName> <RawCitation>" for precedents (e.g. "Smith v Jones [2020]
// UKSC 1"), and "<Act>, s.<Section>(<Clause>)" for statutes, matching
// packages/statute's Citation.String() convention (e.g. "Act 12,
// s.5(a)").
var CommonLawFormatter Formatter = FormatterFunc(func(in FormatInput) string {
	if in.Origin == OriginPrecedent {
		return formatCommonLawCase(in.CaseName, in.RawCitation)
	}
	return formatCommonLawStatute(in.Act, in.Section, in.Clause)
})

func formatCommonLawCase(caseName, rawCitation string) string {
	caseName = strings.TrimSpace(caseName)
	rawCitation = strings.TrimSpace(rawCitation)
	switch {
	case caseName == "" && rawCitation == "":
		return ""
	case rawCitation == "":
		return caseName
	case caseName == "":
		return rawCitation
	default:
		return fmt.Sprintf("%s %s", caseName, rawCitation)
	}
}

func formatCommonLawStatute(act, section, clause string) string {
	act = strings.TrimSpace(act)
	section = strings.TrimSpace(section)
	clause = strings.TrimSpace(clause)
	if act == "" {
		return ""
	}
	if section == "" {
		return act
	}
	if clause == "" {
		return fmt.Sprintf("%s, s.%s", act, section)
	}
	return fmt.Sprintf("%s, s.%s(%s)", act, section, clause)
}

// CivilLawFormatter formats citations in civil-law article-citation
// style: "Art. <Section> <Act>" for statutes (e.g. "Art. 5 Code Civil"),
// and "<CaseName>, <RawCitation>" for precedents (civil-law systems treat
// case reports as persuasive rather than binding authority, but this
// package still needs to render one when a precedent is cited for
// context).
var CivilLawFormatter Formatter = FormatterFunc(func(in FormatInput) string {
	if in.Origin == OriginPrecedent {
		return formatCivilLawCase(in.CaseName, in.RawCitation)
	}
	return formatCivilLawStatute(in.Act, in.Section, in.Clause)
})

func formatCivilLawCase(caseName, rawCitation string) string {
	caseName = strings.TrimSpace(caseName)
	rawCitation = strings.TrimSpace(rawCitation)
	switch {
	case caseName == "" && rawCitation == "":
		return ""
	case rawCitation == "":
		return caseName
	case caseName == "":
		return rawCitation
	default:
		return fmt.Sprintf("%s, %s", caseName, rawCitation)
	}
}

func formatCivilLawStatute(act, section, clause string) string {
	act = strings.TrimSpace(act)
	section = strings.TrimSpace(section)
	clause = strings.TrimSpace(clause)
	if section == "" {
		return act
	}
	label := fmt.Sprintf("Art. %s", section)
	if clause != "" {
		label = fmt.Sprintf("%s.%s", label, clause)
	}
	if act == "" {
		return label
	}
	return fmt.Sprintf("%s %s", label, act)
}

// Registry maps an opaque jurisdiction or legal-family key (e.g.
// "common_law", "civil_law", or a specific jurisdiction code such as
// "us-ny") to the Formatter that should render citations for it. This is
// the pluggable extension point this package exposes for jurisdiction-
// aware formatting without taking a hard dependency on
// packages/jurisdiction: callers register whatever keys their deployment
// needs, keyed by whatever opaque string irac.RuleNode.LegalFamily or
// .JurisdictionCode already carries.
//
// Registry is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	byKey    map[string]Formatter
	fallback Formatter
}

// NewRegistry constructs an empty Registry. Use Register to add
// Formatters and WithFallback to set a default used for unrecognized
// keys.
func NewRegistry() *Registry {
	return &Registry{byKey: make(map[string]Formatter)}
}

// NewDefaultRegistry constructs a Registry pre-populated with
// "common_law" -> CommonLawFormatter and "civil_law" -> CivilLawFormatter,
// matching irac.RuleNode.LegalFamily's documented example values.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("common_law", CommonLawFormatter)
	r.Register("civil_law", CivilLawFormatter)
	return r
}

// Register associates key with f, overwriting any previously registered
// Formatter for the same key.
func (r *Registry) Register(key string, f Formatter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKey[key] = f
}

// WithFallback sets the Formatter used by Format when key has no
// registered Formatter. Returns r for chaining.
func (r *Registry) WithFallback(f Formatter) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = f
	return r
}

// Format formats in using the Formatter registered under key, or the
// fallback Formatter (set via WithFallback) if key is unrecognized. It
// returns ErrUnknownFormatter if key is unrecognized and no fallback was
// configured.
func (r *Registry) Format(key string, in FormatInput) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if f, ok := r.byKey[key]; ok {
		return f.Format(in), nil
	}
	if r.fallback != nil {
		return r.fallback.Format(in), nil
	}
	return "", ErrUnknownFormatter
}

// Has reports whether key has a registered Formatter (not counting the
// fallback).
func (r *Registry) Has(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.byKey[key]
	return ok
}
