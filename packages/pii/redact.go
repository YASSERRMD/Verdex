package pii

import (
	"fmt"
	"sort"
	"strings"
)

// RedactionMode selects how a detected PIIMatch is transformed when applied
// to text by a Redactor.
type RedactionMode string

const (
	// ModeRedact replaces each match with a fixed placeholder of the form
	// "[REDACTED:category]" (e.g. "[REDACTED:email]"). This is a one-way
	// transform: the original value is not retained anywhere by the
	// Redactor itself.
	ModeRedact RedactionMode = "redact"

	// ModePseudonymize replaces each match with a stable per-entity token
	// (e.g. "PERSON_1", "CONTACT_2") allocated from a PseudonymMap (see
	// mapping.go). The same original value always maps to the same token
	// within a given PseudonymMap, and the mapping is retained so an
	// authorized caller can reverse it later (see mapping.go's
	// AccessPolicy).
	ModePseudonymize RedactionMode = "pseudonymize"

	// ModeIrreversibleRedact behaves like ModeRedact (replaces the match
	// with a "[REDACTED:category]" placeholder) but is a distinct mode so
	// callers can express, and Redactor/PIIService can enforce, that no
	// mapping may ever be stored for these matches — see mapping.go and
	// PseudonymMap.Reveal, which returns ErrAlreadyIrreversible for tokens
	// processed under this mode.
	ModeIrreversibleRedact RedactionMode = "irreversible_redact"
)

// RedactionResult is the output of applying a Redactor to text.
type RedactionResult struct {
	// Text is the resulting text with every match replaced according to
	// the configured RedactionMode.
	Text string

	// Applied lists, in the order matches were applied, the mode and (for
	// ModePseudonymize) token used for each match.
	Applied []AppliedRedaction
}

// AppliedRedaction records how a single PIIMatch was transformed.
type AppliedRedaction struct {
	Match       PIIMatch
	Mode        RedactionMode
	Replacement string

	// Token is the pseudonym token allocated for this match, populated only
	// when Mode == ModePseudonymize.
	Token string
}

// Redactor applies a configured RedactionMode over a set of detected
// PIIMatches within a text.
type Redactor struct {
	// Mode is the default redaction mode applied to every match that does
	// not have a more specific override in ModeByCategory. Defaults to
	// ModeRedact if left at the zero value when NewRedactor is used.
	Mode RedactionMode

	// ModeByCategory optionally overrides Mode for specific categories
	// (e.g. always ModeIrreversibleRedact for CategoryFinancial regardless
	// of the package-wide default). Nil or missing entries fall back to
	// Mode.
	ModeByCategory map[PIICategory]RedactionMode

	// Pseudonyms is the PseudonymMap used to allocate and record tokens for
	// matches redacted under ModePseudonymize. Required when Mode (or any
	// ModeByCategory override) is ModePseudonymize; a nil map causes
	// Redact to return ErrInvalidRequest for such matches.
	Pseudonyms *PseudonymMap
}

// NewRedactor constructs a Redactor with the given default mode and
// pseudonym map. pseudonyms may be nil if mode (and every ModeByCategory
// override, if set later) is never ModePseudonymize.
func NewRedactor(mode RedactionMode, pseudonyms *PseudonymMap) *Redactor {
	if mode == "" {
		mode = ModeRedact
	}
	return &Redactor{Mode: mode, Pseudonyms: pseudonyms}
}

// modeFor resolves the effective RedactionMode for a given category.
func (r *Redactor) modeFor(category PIICategory) RedactionMode {
	if r.ModeByCategory != nil {
		if m, ok := r.ModeByCategory[category]; ok {
			return m
		}
	}
	if r.Mode == "" {
		return ModeRedact
	}
	return r.Mode
}

// Redact applies this Redactor's configured mode(s) over matches within
// text, returning the resulting text and a record of what was applied to
// each match. matches need not be sorted; Redact sorts and de-overlaps
// internally so replacement offsets stay valid. Matches must already carry
// a Category (see ClassifyMatches) for ModeByCategory overrides and
// "[REDACTED:category]" placeholders to be meaningful; an empty Category
// falls back to CategoryOther in the placeholder text.
func (r *Redactor) Redact(text string, matches []PIIMatch) (RedactionResult, error) {
	if len(matches) == 0 {
		return RedactionResult{Text: text}, nil
	}

	ordered := make([]PIIMatch, len(matches))
	copy(ordered, matches)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Start < ordered[j].Start })

	runes := []rune(text)
	var b strings.Builder
	applied := make([]AppliedRedaction, 0, len(ordered))

	cursor := 0
	for _, m := range ordered {
		if m.Start < cursor || m.Start < 0 || m.End > len(runes) || m.End < m.Start {
			// Skip malformed/overlapping-with-already-applied matches
			// rather than corrupting output.
			continue
		}
		b.WriteString(string(runes[cursor:m.Start]))

		mode := r.modeFor(m.Category)
		replacement, token, err := r.replacementFor(m, mode)
		if err != nil {
			return RedactionResult{}, err
		}

		b.WriteString(replacement)
		applied = append(applied, AppliedRedaction{Match: m, Mode: mode, Replacement: replacement, Token: token})
		cursor = m.End
	}
	b.WriteString(string(runes[cursor:]))

	return RedactionResult{Text: b.String(), Applied: applied}, nil
}

// replacementFor computes the replacement string (and, for
// ModePseudonymize, the allocated token) for a single match under mode.
func (r *Redactor) replacementFor(m PIIMatch, mode RedactionMode) (replacement string, token string, err error) {
	category := m.Category
	if category == "" {
		category = CategoryOther
	}

	switch mode {
	case ModePseudonymize:
		if r.Pseudonyms == nil {
			return "", "", fmt.Errorf("%w: pseudonymize mode requires a PseudonymMap", ErrInvalidRequest)
		}
		tok := r.Pseudonyms.TokenFor(category, m.Text)
		return tok, tok, nil
	case ModeIrreversibleRedact, ModeRedact, "":
		return fmt.Sprintf("[REDACTED:%s]", category), "", nil
	default:
		return "", "", fmt.Errorf("%w: unknown redaction mode %q", ErrInvalidRequest, mode)
	}
}
