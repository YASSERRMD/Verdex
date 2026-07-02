package ocr

import "fmt"

// PreprocessResult records which pre-processing transforms were applied to
// an ImageInput before it was handed to an OCRProvider, along with the
// corrected metadata.
//
// Like packages/stt's Normalize/Segment, this package implements no real
// image codec: pre-processing operates purely on structured metadata (and
// byte length), tracking which deterministic corrections *would* be applied
// by a real decoder. Concrete adapters remain responsible for any actual
// pixel-level deskew/denoise work.
type PreprocessResult struct {
	// Image is the (metadata-adjusted) ImageInput to hand to the provider.
	// Data is unchanged; only declared metadata may differ from the input.
	Image ImageInput

	// Deskewed reports whether a rotation correction was applied.
	Deskewed bool

	// RotationCorrectionDeg is the rotation angle, in degrees, that was
	// (notionally) applied to straighten the page. Positive values rotate
	// clockwise. Zero when Deskewed is false.
	RotationCorrectionDeg float64

	// Denoised reports whether a denoise pass was (notionally) applied.
	Denoised bool
}

// DefaultDeskewThresholdDeg is the minimum |angle| that Preprocess will
// treat as requiring a deskew correction. Angles smaller than this are
// considered "already straight" and left untouched.
const DefaultDeskewThresholdDeg = 0.5

// DefaultDenoiseMinBytes is the minimum byte length Preprocess requires
// before it will mark an image as denoised. Very small (likely synthetic or
// degenerate) inputs are left as-is.
const DefaultDenoiseMinBytes = 16

// Preprocess returns a PreprocessResult describing the deterministic
// metadata-level corrections applied to input.
//
//   - Deskew: if skewAngleDeg (the caller's estimate of the page's rotation,
//     e.g. from a scanner or a prior detection pass) exceeds
//     DefaultDeskewThresholdDeg in magnitude, Deskewed is set true and
//     RotationCorrectionDeg records the corrective angle (the negation of
//     skewAngleDeg, so applying it would straighten the page).
//   - Denoise: if input.Data is at least DefaultDenoiseMinBytes long,
//     Denoised is set true. No bytes are modified — this package does not
//     implement pixel-level denoising, only tracks that the step ran.
//
// Preprocess returns ErrEmptyImage if input.Data is empty. It does not
// mutate input; input.Data is passed through unchanged in the returned
// PreprocessResult.Image.
func Preprocess(input ImageInput, skewAngleDeg float64) (PreprocessResult, error) {
	if len(input.Data) == 0 {
		return PreprocessResult{}, ErrEmptyImage
	}

	out := input
	result := PreprocessResult{}

	if abs(skewAngleDeg) > DefaultDeskewThresholdDeg {
		result.Deskewed = true
		result.RotationCorrectionDeg = -skewAngleDeg
	}

	if len(input.Data) >= DefaultDenoiseMinBytes {
		result.Denoised = true
	}

	result.Image = out
	return result, nil
}

// Summary returns a short, human-readable description of the applied steps,
// e.g. "deskewed(2.30deg), denoised". Useful for logging and test
// assertions.
func (p PreprocessResult) Summary() string {
	if !p.Deskewed && !p.Denoised {
		return "none"
	}
	s := ""
	if p.Deskewed {
		s += fmt.Sprintf("deskewed(%.2fdeg)", p.RotationCorrectionDeg)
	}
	if p.Denoised {
		if s != "" {
			s += ", "
		}
		s += "denoised"
	}
	return s
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
