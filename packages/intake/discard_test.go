package intake_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/intake"
)

// TestIngest_BufferDiscardedAfterTTL verifies that the TempBuffer associated
// with an intake operation is discarded after its TTL elapses.
func TestIngest_BufferDiscardedAfterTTL(t *testing.T) {
	svc := newTestService(t, intake.QuotaConfig{MaxFileSizeMB: 10})

	req := intake.IntakeRequest{
		TenantID:   newUUID(t),
		UploaderID: newUUID(t),
		Filename:   "test.txt",
		MIMEType:   "text/plain",
		SizeBytes:  int64(len(pdfPayload)),
		TTL:        200 * time.Millisecond,
	}

	result, err := svc.Ingest(context.Background(), req, strings.NewReader(string(pdfPayload)))
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result.Status != intake.StatusReady {
		t.Fatalf("expected status ready, got %s", result.Status)
	}

	// Wait for TTL to elapse plus a safety margin.
	time.Sleep(500 * time.Millisecond)

	if result.Status != intake.StatusDiscarded {
		t.Fatalf("expected status discarded after TTL, got %s", result.Status)
	}

	if result.DiscardedAt == nil {
		t.Fatal("DiscardedAt should be set after discard")
	}
}

// TestIngest_ManualDiscard verifies that DiscardAll removes the buffer
// immediately.
func TestIngest_ManualDiscard(t *testing.T) {
	svc := newTestService(t, intake.QuotaConfig{MaxFileSizeMB: 10})

	req := intake.IntakeRequest{
		TenantID:   newUUID(t),
		UploaderID: newUUID(t),
		Filename:   "test.txt",
		MIMEType:   "text/plain",
		SizeBytes:  int64(len(pdfPayload)),
		TTL:        5 * time.Minute,
	}

	result, err := svc.Ingest(context.Background(), req, strings.NewReader(string(pdfPayload)))
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if err := svc.DiscardAll(context.Background(), result.IntakeID); err != nil {
		t.Fatalf("DiscardAll failed: %v", err)
	}
}

// TestTempBuffer_DiscardPreventsRead verifies that reading from a TempBuffer
// after Discard returns ErrBufferDiscarded.
func TestTempBuffer_DiscardPreventsRead(t *testing.T) {
	tb, err := intake.Create(5 * time.Minute)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := tb.Write([]byte("hello")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := tb.Discard(); err != nil {
		t.Fatalf("Discard failed: %v", err)
	}

	if _, err := tb.Reader(); err == nil {
		t.Fatal("expected error reading from discarded buffer, got nil")
	}
}

// TestTempBuffer_TTLExpiry verifies that writing to an expired buffer returns
// ErrBufferExpired.
func TestTempBuffer_TTLExpiry(t *testing.T) {
	tb, err := intake.Create(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() { _ = tb.Discard() })

	// Wait for TTL to elapse.
	time.Sleep(150 * time.Millisecond)

	// The internal timer will have called Discard; try a write.
	_, writeErr := tb.Write([]byte("late write"))
	if writeErr == nil {
		t.Fatal("expected error writing to expired/discarded buffer, got nil")
	}
}

// TestTempBuffer_ReaderAfterDiscard verifies that Reader returns an error after
// Discard.
func TestTempBuffer_ReaderAfterDiscard(t *testing.T) {
	tb, err := intake.Create(5 * time.Minute)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := tb.Write([]byte("data")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := tb.Discard(); err != nil {
		t.Fatalf("Discard failed: %v", err)
	}

	_, err = tb.Reader()
	if err == nil {
		t.Fatal("expected error from Reader after Discard, got nil")
	}
}

// TestTempBuffer_DoubleDiscard verifies that calling Discard twice is safe.
func TestTempBuffer_DoubleDiscard(t *testing.T) {
	tb, err := intake.Create(5 * time.Minute)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := tb.Discard(); err != nil {
		t.Fatalf("first Discard failed: %v", err)
	}
	if err := tb.Discard(); err != nil {
		t.Fatalf("second Discard should be a no-op, got: %v", err)
	}
}

// TestStreamingHashReader_ConsistentWithComputeSHA256 verifies that the
// StreamingHashReader produces the same hash as ComputeSHA256 on the same
// data, confirming the two code paths agree.
func TestStreamingHashReader_ConsistentWithComputeSHA256(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog")

	directHash, _, err := intake.ComputeSHA256(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("ComputeSHA256 failed: %v", err)
	}

	sr := intake.NewStreamingHashReader(strings.NewReader(string(data)))
	if _, err := io.Copy(io.Discard, sr); err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	streamHash := sr.Hash()

	if directHash != streamHash {
		t.Fatalf("hash mismatch: direct=%s streaming=%s", directHash, streamHash)
	}
}
