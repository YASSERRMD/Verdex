package ingestion

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
	"github.com/YASSERRMD/verdex/packages/stt"
)

func TestVerifyAudioDiscard_Success(t *testing.T) {
	input := stt.AudioInput{Data: []byte{}} // zeroed/truncated, as Discard leaves it
	v, err := VerifyAudioDiscard("job-1", input, "deadbeef")
	if err != nil {
		t.Fatalf("VerifyAudioDiscard: %v", err)
	}
	if !v.Verified || !v.BytesZeroed || v.SourceHash != "deadbeef" {
		t.Errorf("v = %+v, want fully verified", v)
	}
}

func TestVerifyAudioDiscard_BytesNotDiscarded(t *testing.T) {
	input := stt.AudioInput{Data: []byte("still here")}
	_, err := VerifyAudioDiscard("job-1", input, "deadbeef")
	if !errors.Is(err, ErrDiscardVerificationFailed) {
		t.Errorf("err = %v, want ErrDiscardVerificationFailed", err)
	}
}

func TestVerifyAudioDiscard_MissingHash(t *testing.T) {
	input := stt.AudioInput{Data: []byte{}}
	_, err := VerifyAudioDiscard("job-1", input, "")
	if !errors.Is(err, ErrDiscardVerificationFailed) {
		t.Errorf("err = %v, want ErrDiscardVerificationFailed", err)
	}
}

func TestVerifyImageDiscard_Success(t *testing.T) {
	input := ocr.ImageInput{Data: []byte{}}
	v, err := VerifyImageDiscard("job-1", input, "cafebabe")
	if err != nil {
		t.Fatalf("VerifyImageDiscard: %v", err)
	}
	if !v.Verified {
		t.Errorf("v = %+v, want Verified=true", v)
	}
}

func TestVerifyImageDiscard_BytesNotDiscarded(t *testing.T) {
	input := ocr.ImageInput{Data: []byte("still here")}
	_, err := VerifyImageDiscard("job-1", input, "cafebabe")
	if !errors.Is(err, ErrDiscardVerificationFailed) {
		t.Errorf("err = %v, want ErrDiscardVerificationFailed", err)
	}
}

func TestVerifyImageDiscard_MissingHash(t *testing.T) {
	input := ocr.ImageInput{Data: []byte{}}
	_, err := VerifyImageDiscard("job-1", input, "")
	if !errors.Is(err, ErrDiscardVerificationFailed) {
		t.Errorf("err = %v, want ErrDiscardVerificationFailed", err)
	}
}

// TestVerifyAudioDiscard_MatchesRealSTTDiscard exercises the real
// packages/stt Discard function to ensure VerifyAudioDiscard's assumptions
// about post-discard AudioInput shape hold against the actual
// implementation, not just a hand-built fixture.
func TestVerifyAudioDiscard_MatchesRealSTTDiscard(t *testing.T) {
	input := stt.AudioInput{Data: []byte("real audio payload")}
	hash := stt.ComputeSourceHash(input.Data)

	if err := stt.Discard(context.Background(), &input, hash, "noop", stt.NoOpDiscardSink{}); err != nil {
		t.Fatalf("stt.Discard: %v", err)
	}

	v, err := VerifyAudioDiscard("job-1", input, hash)
	if err != nil {
		t.Fatalf("VerifyAudioDiscard: %v", err)
	}
	if !v.Verified {
		t.Errorf("v = %+v, want Verified=true", v)
	}
}

// TestVerifyImageDiscard_MatchesRealOCRDiscard exercises the real
// packages/ocr Discard function analogous to the STT case above.
func TestVerifyImageDiscard_MatchesRealOCRDiscard(t *testing.T) {
	input := ocr.ImageInput{Data: []byte("real image payload")}
	hash := ocr.ComputeSourceHash(input.Data)

	if err := ocr.Discard(context.Background(), &input, hash, "noop", ocr.NoOpDiscardSink{}); err != nil {
		t.Fatalf("ocr.Discard: %v", err)
	}

	v, err := VerifyImageDiscard("job-1", input, hash)
	if err != nil {
		t.Fatalf("VerifyImageDiscard: %v", err)
	}
	if !v.Verified {
		t.Errorf("v = %+v, want Verified=true", v)
	}
}
