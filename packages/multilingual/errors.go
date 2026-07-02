package multilingual

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyInput is returned when a normalization operation is given
	// empty (or whitespace-only, after cleanup) text.
	ErrEmptyInput = errors.New("multilingual: input is empty")

	// ErrUnsupportedLanguage is returned when an operation is requested
	// for a Language this package (or the configured pluggable
	// component) does not support.
	ErrUnsupportedLanguage = errors.New("multilingual: unsupported language")

	// ErrUnsupportedScript is returned when a Transliterator is asked to
	// convert between a script pair it does not support.
	ErrUnsupportedScript = errors.New("multilingual: unsupported script pair")

	// ErrInvalidRequest is returned when a request contains invalid or
	// missing fields.
	ErrInvalidRequest = errors.New("multilingual: invalid request")
)
