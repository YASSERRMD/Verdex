package multilingual

// Transliterator converts text from one script into another while
// attempting to preserve pronunciation (as opposed to Translator, which
// changes the language/meaning of the text).
//
// Verdex never hardcodes a specific transliteration scheme: real schemes
// (ISO 15919 for Tamil, ALA-LC or Buckwalter for Arabic/Urdu, etc.) are
// extensive, jurisdiction- and vendor-specific, and out of scope for this
// package. Transliterator is a thin, pluggable extension point; concrete
// mapping tables are expected to be supplied by adapters outside this
// package, mirroring the STTProvider / OCRProvider / Translator pattern
// used elsewhere in Verdex.
//
// Implementations MUST be safe for concurrent use from multiple
// goroutines.
type Transliterator interface {
	// ID returns the stable, unique identifier for this transliterator
	// (e.g. "passthrough", "iso15919-tamil").
	ID() string

	// Transliterate converts text (written in fromScript) into its
	// representation in toScript. Implementations that do not support the
	// requested script pair should return the input unchanged along with
	// ErrUnsupportedScript.
	Transliterate(text string, fromScript, toScript Script) (string, error)
}

// PassthroughTransliterator is the deterministic default Transliterator: it
// never changes script and always returns the input unchanged, whether or
// not fromScript equals toScript. It exists so callers can wire a
// Transliterator into the pipeline without requiring a real transliteration
// backend, exactly as NoOpSTTProvider and NoOpOCRProvider stand in for
// their respective backends in packages/stt and packages/ocr.
type PassthroughTransliterator struct{}

// ID implements Transliterator.
func (PassthroughTransliterator) ID() string { return "passthrough" }

// Transliterate implements Transliterator by returning text unchanged.
func (PassthroughTransliterator) Transliterate(text string, _, _ Script) (string, error) {
	return text, nil
}

// exampleTamilToLatinMap is a small, deliberately incomplete worked example
// showing how a real Transliterator implementation could map Tamil
// characters to a Latin approximation. It exists purely for demonstration
// and testing (see ExampleTransliterator in transliterate_test.go); it is
// NOT a complete or standards-compliant transliteration scheme (compare to
// ISO 15919) and must not be relied upon for production transliteration.
var exampleTamilToLatinMap = map[rune]string{
	'அ': "a",
	'ஆ': "aa",
	'இ': "i",
	'ஈ': "ii",
	'உ': "u",
	'ஊ': "uu",
	'க': "ka",
	'த': "tha",
	'ம': "ma",
	'ன': "na",
}

// ExampleTamilTransliterator is a worked-example Transliterator
// demonstrating how a concrete implementation plugs into the
// Transliterator interface. It supports only ScriptTamil -> ScriptLatin,
// using exampleTamilToLatinMap; any other script pair, or any Tamil
// character not present in the map, is passed through unchanged.
//
// This is intentionally minimal: it demonstrates the extension point, it
// does not attempt to be a production-quality transliteration engine.
type ExampleTamilTransliterator struct{}

// ID implements Transliterator.
func (ExampleTamilTransliterator) ID() string { return "example-tamil-latin" }

// Transliterate implements Transliterator for the Tamil -> Latin direction
// using exampleTamilToLatinMap. Unmapped runes are copied through
// unchanged. Any other (fromScript, toScript) pair returns text unchanged
// together with a wrapped ErrUnsupportedScript.
func (ExampleTamilTransliterator) Transliterate(text string, fromScript, toScript Script) (string, error) {
	if fromScript != ScriptTamil || toScript != ScriptLatin {
		return text, ErrUnsupportedScript
	}

	var out []byte
	for _, r := range text {
		if mapped, ok := exampleTamilToLatinMap[r]; ok {
			out = append(out, mapped...)
			continue
		}
		out = append(out, string(r)...)
	}
	return string(out), nil
}
