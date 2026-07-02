package precedent

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// RawPrecedent is the loader's output shape: a single prior case/precedent
// as read from a corpus source, before holding/ratio extraction (see
// holding.go) or irac.RuleNode construction (see rule.go) is applied.
//
// RawPrecedent deliberately keeps FullText as unparsed prose rather than an
// already-extracted holding/ratio pair, mirroring packages/statute's
// RawStatute/Body split: loading (finding and decoding the raw units of
// input) is kept separate from structural/semantic extraction.
type RawPrecedent struct {
	// CaseName is the human-readable case name (e.g. "Donoghue v
	// Stevenson"). Required.
	CaseName string `json:"case_name"`

	// Citation is the raw case citation as it appears in the corpus (e.g.
	// "[1932] AC 562"). Formatting into this package's own Citation shape
	// happens in rule.go.
	Citation string `json:"citation"`

	// Court identifies the deciding court or tribunal (e.g. "House of
	// Lords", "Supreme Court"). Free text at load time; hierarchy.go maps
	// this to a CourtLevel later in the pipeline.
	Court string `json:"court"`

	// DecidedDate is the date the case was decided. Zero value means
	// unknown/unspecified.
	DecidedDate time.Time `json:"decided_date"`

	// FullText is the raw, unparsed judgment text, including any
	// "HELD:"/"HOLDING:" markers understood by holding.go's extractor.
	FullText string `json:"full_text"`
}

// Loader reads a precedent corpus from source and returns the
// RawPrecedents it contains. Implementations are pure parsers: no network
// fetch, no filesystem traversal — source is supplied by the caller.
type Loader interface {
	Load(ctx context.Context, source io.Reader) ([]RawPrecedent, error)
}

// Ensure DefaultLoader satisfies Loader at compile time.
var _ Loader = (*DefaultLoader)(nil)

// DefaultLoader is the default Loader implementation. It accepts two input
// formats, auto-detected from the source's first non-whitespace byte:
//
//   - JSON: a top-level JSON array of RawPrecedent objects (or an object
//     with a top-level "precedents" array field), decoded directly via
//     encoding/json. DecidedDate is parsed from RFC3339 strings by
//     encoding/json's default time.Time unmarshaling.
//   - Structured text: a simple line-oriented format where each case
//     begins with a line of the form "CASE <citation>: <case name>",
//     optionally followed by a "COURT: <court>" line and a
//     "DECIDED: <YYYY-MM-DD>" line, with every subsequent line up to the
//     next "CASE" line (or end of input) forming that case's FullText.
//
// DefaultLoader performs no holding extraction, tagging, or hierarchy
// classification — see holding.go, tagging.go, and hierarchy.go for those
// stages.
type DefaultLoader struct{}

// NewDefaultLoader constructs a DefaultLoader.
func NewDefaultLoader() *DefaultLoader {
	return &DefaultLoader{}
}

// jsonPrecedentEnvelope supports the "object with a precedents field" JSON
// input shape, as an alternative to a bare top-level array.
type jsonPrecedentEnvelope struct {
	Precedents []RawPrecedent `json:"precedents"`
}

// Load implements Loader. Returns ErrMalformedCorpus if source contains no
// recognizable case content, or if JSON-shaped input fails to decode.
func (l *DefaultLoader) Load(ctx context.Context, source io.Reader) ([]RawPrecedent, error) {
	if source == nil {
		return nil, ErrMalformedCorpus
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(source)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, ErrMalformedCorpus
	}

	if trimmed[0] == '{' || trimmed[0] == '[' {
		return parseJSONCorpus(trimmed)
	}
	return parseTextCorpus(trimmed)
}

func parseJSONCorpus(text string) ([]RawPrecedent, error) {
	if text[0] == '[' {
		var precedents []RawPrecedent
		if err := json.Unmarshal([]byte(text), &precedents); err != nil {
			return nil, ErrMalformedCorpus
		}
		if len(precedents) == 0 {
			return nil, ErrMalformedCorpus
		}
		return precedents, nil
	}

	var envelope jsonPrecedentEnvelope
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return nil, ErrMalformedCorpus
	}
	if len(envelope.Precedents) == 0 {
		return nil, ErrMalformedCorpus
	}
	return envelope.Precedents, nil
}

// caseHeaderPrefix marks the start of a new case in the structured text
// input format: "CASE <citation>: <case name>".
const caseHeaderPrefix = "CASE "

// courtLinePrefix marks the optional court line within a case block.
const courtLinePrefix = "COURT:"

// decidedLinePrefix marks the optional decided-date line within a case
// block.
const decidedLinePrefix = "DECIDED:"

func parseTextCorpus(text string) ([]RawPrecedent, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var precedents []RawPrecedent
	var current *RawPrecedent
	var body strings.Builder

	flush := func() {
		if current == nil {
			return
		}
		current.FullText = strings.TrimSpace(body.String())
		precedents = append(precedents, *current)
		current = nil
		body.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmedLine, caseHeaderPrefix):
			flush()
			header := strings.TrimPrefix(trimmedLine, caseHeaderPrefix)
			citation, name := splitHeader(header)
			current = &RawPrecedent{CaseName: name, Citation: citation}
			continue
		case current != nil && strings.HasPrefix(trimmedLine, courtLinePrefix):
			current.Court = strings.TrimSpace(strings.TrimPrefix(trimmedLine, courtLinePrefix))
			continue
		case current != nil && strings.HasPrefix(trimmedLine, decidedLinePrefix):
			raw := strings.TrimSpace(strings.TrimPrefix(trimmedLine, decidedLinePrefix))
			if t, err := time.Parse("2006-01-02", raw); err == nil {
				current.DecidedDate = t
			}
			continue
		}
		if current == nil {
			// Content before any CASE header is not a recognized corpus
			// shape.
			continue
		}
		body.WriteString(line)
		body.WriteString("\n")
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(precedents) == 0 {
		return nil, ErrMalformedCorpus
	}
	return precedents, nil
}

// splitHeader splits "<citation>: <case name>" into its two parts. If no
// colon is present, the whole string is treated as the citation and the
// case name is left empty.
func splitHeader(header string) (citation, name string) {
	idx := strings.Index(header, ":")
	if idx < 0 {
		return strings.TrimSpace(header), ""
	}
	return strings.TrimSpace(header[:idx]), strings.TrimSpace(header[idx+1:])
}
