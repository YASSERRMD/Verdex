package shared

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// StreamReader reads server-sent events (SSE) from an io.Reader.
// It is safe for sequential use from a single goroutine.
//
// The SSE wire format used by OpenAI and Anthropic is:
//
//	data: <json payload>\n\n
//
// Comment lines (": ping") are skipped automatically.
type StreamReader struct {
	scanner *bufio.Scanner
}

// NewStreamReader wraps r in a StreamReader. The caller is responsible for
// closing r when done.
func NewStreamReader(r io.Reader) *StreamReader {
	s := bufio.NewScanner(r)
	// Increase the default 64 KiB buffer for large SSE payloads.
	s.Buffer(make([]byte, 0, 64*1024), 512*1024)
	return &StreamReader{scanner: s}
}

// ReadEvent reads the next SSE data line and returns its payload.
//
//   - data is the raw string after "data: " (may be "[DONE]" for OpenAI).
//   - done is true when the underlying reader is exhausted.
//   - err is non-nil on scanner errors.
//
// Ping/comment lines (":" prefix) are skipped transparently.
func (sr *StreamReader) ReadEvent() (data string, done bool, err error) {
	for sr.scanner.Scan() {
		line := sr.scanner.Text()

		// Skip empty lines (SSE field separators) and comment/ping lines.
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		const prefix = "data: "
		if !strings.HasPrefix(line, prefix) {
			// Ignore non-data fields (event:, id:, retry:).
			continue
		}

		payload := strings.TrimPrefix(line, prefix)
		return payload, false, nil
	}

	if scanErr := sr.scanner.Err(); scanErr != nil {
		return "", true, fmt.Errorf("SSE scanner error: %w", scanErr)
	}
	return "", true, nil
}
