package stt

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrProviderNotFound is returned by Registry.Get when no provider is
	// registered under the requested ID.
	ErrProviderNotFound = errors.New("stt: provider not found")

	// ErrEmptyAudio is returned when an AudioInput has no data to transcribe.
	ErrEmptyAudio = errors.New("stt: audio input is empty")

	// ErrAlreadyDiscarded is returned when an operation is attempted on an
	// AudioInput whose source bytes have already been discarded.
	ErrAlreadyDiscarded = errors.New("stt: audio source already discarded")

	// ErrInvalidRequest is returned when a request contains invalid or
	// missing fields (e.g. registering a provider under an empty ID).
	ErrInvalidRequest = errors.New("stt: invalid request")

	// ErrProviderUnavailable is returned when the provider's transcription
	// backend is unreachable or returning errors.
	ErrProviderUnavailable = errors.New("stt: provider unavailable")
)
