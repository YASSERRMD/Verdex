package ocr

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrProviderNotFound is returned by Registry.Get when no provider is
	// registered under the requested ID.
	ErrProviderNotFound = errors.New("ocr: provider not found")

	// ErrEmptyImage is returned when an ImageInput has no data to extract
	// text from.
	ErrEmptyImage = errors.New("ocr: image input is empty")

	// ErrAlreadyDiscarded is returned when an operation is attempted on an
	// ImageInput whose source bytes have already been discarded.
	ErrAlreadyDiscarded = errors.New("ocr: image source already discarded")

	// ErrInvalidRequest is returned when a request contains invalid or
	// missing fields (e.g. registering a provider under an empty ID).
	ErrInvalidRequest = errors.New("ocr: invalid request")

	// ErrProviderUnavailable is returned when the provider's extraction
	// backend is unreachable or returning errors.
	ErrProviderUnavailable = errors.New("ocr: provider unavailable")
)
