package ocr_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
)

func TestPreprocess_EmptyImage_ReturnsError(t *testing.T) {
	_, err := ocr.Preprocess(ocr.ImageInput{}, 0)
	if !errors.Is(err, ocr.ErrEmptyImage) {
		t.Errorf("Preprocess() error = %v, want ErrEmptyImage", err)
	}
}

func TestPreprocess_SmallSkew_NotDeskewed(t *testing.T) {
	in := ocr.ImageInput{Data: make([]byte, 32)}

	result, err := ocr.Preprocess(in, 0.1)
	if err != nil {
		t.Fatalf("Preprocess() unexpected error: %v", err)
	}
	if result.Deskewed {
		t.Error("Deskewed = true for a below-threshold skew angle, want false")
	}
}

func TestPreprocess_LargeSkew_Deskewed(t *testing.T) {
	in := ocr.ImageInput{Data: make([]byte, 32)}

	result, err := ocr.Preprocess(in, 3.5)
	if err != nil {
		t.Fatalf("Preprocess() unexpected error: %v", err)
	}
	if !result.Deskewed {
		t.Error("Deskewed = false for an above-threshold skew angle, want true")
	}
	if result.RotationCorrectionDeg != -3.5 {
		t.Errorf("RotationCorrectionDeg = %v, want -3.5", result.RotationCorrectionDeg)
	}
}

func TestPreprocess_NegativeSkew_Deskewed(t *testing.T) {
	in := ocr.ImageInput{Data: make([]byte, 32)}

	result, err := ocr.Preprocess(in, -4.0)
	if err != nil {
		t.Fatalf("Preprocess() unexpected error: %v", err)
	}
	if !result.Deskewed {
		t.Error("Deskewed = false for a large negative skew angle, want true")
	}
	if result.RotationCorrectionDeg != 4.0 {
		t.Errorf("RotationCorrectionDeg = %v, want 4.0", result.RotationCorrectionDeg)
	}
}

func TestPreprocess_TinyPayload_NotDenoised(t *testing.T) {
	in := ocr.ImageInput{Data: make([]byte, 4)}

	result, err := ocr.Preprocess(in, 0)
	if err != nil {
		t.Fatalf("Preprocess() unexpected error: %v", err)
	}
	if result.Denoised {
		t.Error("Denoised = true for a below-minimum payload, want false")
	}
}

func TestPreprocess_LargePayload_Denoised(t *testing.T) {
	in := ocr.ImageInput{Data: make([]byte, 64)}

	result, err := ocr.Preprocess(in, 0)
	if err != nil {
		t.Fatalf("Preprocess() unexpected error: %v", err)
	}
	if !result.Denoised {
		t.Error("Denoised = false for an above-minimum payload, want true")
	}
}

func TestPreprocess_PreservesDataUnchanged(t *testing.T) {
	original := []byte("synthetic page data")
	in := ocr.ImageInput{Data: append([]byte(nil), original...)}

	result, err := ocr.Preprocess(in, 2.0)
	if err != nil {
		t.Fatalf("Preprocess() unexpected error: %v", err)
	}
	if string(result.Image.Data) != string(original) {
		t.Errorf("Preprocess() mutated Data: got %q, want %q", result.Image.Data, original)
	}
}

func TestPreprocessResult_Summary(t *testing.T) {
	tests := []struct {
		name   string
		result ocr.PreprocessResult
		want   string
	}{
		{"none", ocr.PreprocessResult{}, "none"},
		{"deskew_only", ocr.PreprocessResult{Deskewed: true, RotationCorrectionDeg: 1.5}, "deskewed(1.50deg)"},
		{"denoise_only", ocr.PreprocessResult{Denoised: true}, "denoised"},
		{"both", ocr.PreprocessResult{Deskewed: true, RotationCorrectionDeg: -2.25, Denoised: true}, "deskewed(-2.25deg), denoised"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}
