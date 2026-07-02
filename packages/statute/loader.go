package statute

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// RawStatute is the loader's output shape: a single top-level act (or
// standalone statute) as read from a corpus source, before hierarchy
// parsing (see hierarchy.go) turns its Body into a StatuteNode tree.
//
// RawStatute deliberately keeps Body as unparsed text/structure rather
// than an already-built tree, mirroring packages/fact's
// SegmentInput/BuildFactNode split: loading (finding and decoding the raw
// units of input) is kept separate from structural parsing.
type RawStatute struct {
	// ActNumber is the machine-readable identifier of the act (e.g.
	// "12" or "CPC-1908"). Required.
	ActNumber string `json:"act_number"`

	// ActTitle is the human-readable title of the act.
	ActTitle string `json:"act_title"`

	// JurisdictionCode identifies which jurisdiction this statute
	// belongs to, matching packages/jurisdiction's CountryCode-style
	// convention. May be empty if the corpus does not tag jurisdiction
	// at load time (tagging.go can fill it in later).
	JurisdictionCode string `json:"jurisdiction_code,omitempty"`

	// Body is the raw, unparsed act text, including any section/clause
	// markers understood by hierarchy.go's parser (see ParseHierarchy).
	Body string `json:"body"`
}

// Loader reads a statute corpus from source and returns the RawStatutes it
// contains. Implementations are pure parsers: no network fetch, no
// filesystem traversal — source is supplied by the caller.
type Loader interface {
	Load(ctx context.Context, source io.Reader) ([]RawStatute, error)
}

// Ensure DefaultLoader satisfies Loader at compile time.
var _ Loader = (*DefaultLoader)(nil)

// DefaultLoader is the default Loader implementation. It accepts two
// input formats, auto-detected from the source's first non-whitespace
// byte:
//
//   - JSON: a top-level JSON array of RawStatute objects (or an object
//     with a top-level "statutes" array field), decoded directly via
//     encoding/json.
//   - Structured text: a simple line-oriented format where each act
//     begins with a line of the form "ACT <number>: <title>" and every
//     subsequent line up to the next "ACT" line (or end of input) is
//     that act's Body, preserving section/clause markers for
//     hierarchy.go to parse.
//
// DefaultLoader performs no jurisdiction tagging or hierarchy parsing —
// see tagging.go and hierarchy.go for those stages.
type DefaultLoader struct{}

// NewDefaultLoader constructs a DefaultLoader.
func NewDefaultLoader() *DefaultLoader {
	return &DefaultLoader{}
}

// jsonStatuteEnvelope supports the "object with a statutes field" JSON
// input shape, as an alternative to a bare top-level array.
type jsonStatuteEnvelope struct {
	Statutes []RawStatute `json:"statutes"`
}

// Load implements Loader. Returns ErrMalformedCorpus if source contains
// no recognizable act content, or if JSON-shaped input fails to decode.
func (l *DefaultLoader) Load(ctx context.Context, source io.Reader) ([]RawStatute, error) {
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

func parseJSONCorpus(text string) ([]RawStatute, error) {
	if text[0] == '[' {
		var statutes []RawStatute
		if err := json.Unmarshal([]byte(text), &statutes); err != nil {
			return nil, ErrMalformedCorpus
		}
		if len(statutes) == 0 {
			return nil, ErrMalformedCorpus
		}
		return statutes, nil
	}

	var envelope jsonStatuteEnvelope
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return nil, ErrMalformedCorpus
	}
	if len(envelope.Statutes) == 0 {
		return nil, ErrMalformedCorpus
	}
	return envelope.Statutes, nil
}

// actHeaderPrefix marks the start of a new act in the structured text
// input format: "ACT <number>: <title>".
const actHeaderPrefix = "ACT "

func parseTextCorpus(text string) ([]RawStatute, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var statutes []RawStatute
	var current *RawStatute
	var body strings.Builder

	flush := func() {
		if current == nil {
			return
		}
		current.Body = strings.TrimSpace(body.String())
		statutes = append(statutes, *current)
		current = nil
		body.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, actHeaderPrefix) {
			flush()
			header := strings.TrimPrefix(trimmedLine, actHeaderPrefix)
			number, title := splitHeader(header)
			current = &RawStatute{ActNumber: number, ActTitle: title}
			continue
		}
		if current == nil {
			// Content before any ACT header is not a recognized corpus
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
	if len(statutes) == 0 {
		return nil, ErrMalformedCorpus
	}
	return statutes, nil
}

// splitHeader splits "<number>: <title>" into its two parts. If no colon
// is present, the whole string is treated as the number and title is
// left empty.
func splitHeader(header string) (number, title string) {
	idx := strings.Index(header, ":")
	if idx < 0 {
		return strings.TrimSpace(header), ""
	}
	return strings.TrimSpace(header[:idx]), strings.TrimSpace(header[idx+1:])
}
