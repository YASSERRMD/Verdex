package intake

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// ComputeSHA256 reads all bytes from r and returns their hex-encoded SHA-256
// digest together with the number of bytes consumed.  The full content is never
// held in memory simultaneously; the hash is updated in streaming fashion.
func ComputeSHA256(r io.Reader) (hash string, bytesRead int64, err error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", n, fmt.Errorf("intake: hash computation failed after %d bytes: %w", n, err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// StreamingHashReader wraps an io.Reader and computes a running SHA-256 digest
// as bytes flow through it.  Use Hash() after the underlying reader is
// exhausted to retrieve the final digest.  This lets callers write data to a
// TempBuffer and compute the provenance hash in a single pass without
// re-reading the file.
type StreamingHashReader struct {
	r         io.Reader
	h         io.Writer // sha256.Hash implements io.Writer
	hasher    interface{ Sum(b []byte) []byte }
	bytesRead int64
}

// NewStreamingHashReader wraps r so that every Read call also updates an
// internal SHA-256 state.
func NewStreamingHashReader(r io.Reader) *StreamingHashReader {
	h := sha256.New()
	return &StreamingHashReader{
		r:      r,
		h:      h,
		hasher: h,
	}
}

// Read implements io.Reader.  Bytes are forwarded from the underlying reader
// and simultaneously fed into the hash state.
func (s *StreamingHashReader) Read(p []byte) (n int, err error) {
	n, err = s.r.Read(p)
	if n > 0 {
		// The SHA-256 Write never returns an error.
		_, _ = s.h.Write(p[:n])
		s.bytesRead += int64(n)
	}
	return n, err
}

// Hash returns the hex-encoded SHA-256 digest of all bytes read so far.  It
// may be called multiple times; each call returns the digest up to the current
// position in the stream.
func (s *StreamingHashReader) Hash() string {
	return hex.EncodeToString(s.hasher.Sum(nil))
}

// BytesRead returns the total number of bytes consumed from the underlying
// reader.
func (s *StreamingHashReader) BytesRead() int64 {
	return s.bytesRead
}

// VerifyHash reads all remaining bytes from data and checks that their
// SHA-256 digest matches expectedHash (hex-encoded).  Returns (true, nil) on
// match and (false, nil) on mismatch.  A non-nil error indicates an I/O
// failure during reading.
func VerifyHash(data io.Reader, expectedHash string) (bool, error) {
	actual, _, err := ComputeSHA256(data)
	if err != nil {
		return false, err
	}
	return actual == expectedHash, nil
}
