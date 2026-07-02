package segmentation

// DefaultConfidence is the confidence assigned to a segment produced from
// plain text with no upstream OCR/STT confidence signal available.
const DefaultConfidence = 1.0

// ClampConfidence clamps c into the closed interval [0, 1], mirroring
// packages/ocr's ClampConfidence helper.
func ClampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}

// AssignDefaultConfidence sets Confidence to DefaultConfidence on every
// segment in segs whose Confidence is currently the zero value (0),
// distinguishing "no upstream signal" (plain text) from a genuine
// zero-confidence OCR/STT result. Returns a new slice; does not mutate segs.
func AssignDefaultConfidence(segs []Segment) []Segment {
	out := make([]Segment, len(segs))
	for i, s := range segs {
		if s.Confidence == 0 {
			s.Confidence = DefaultConfidence
		} else {
			s.Confidence = ClampConfidence(s.Confidence)
		}
		out[i] = s
	}
	return out
}

// AggregateConfidence combines the per-unit confidences (e.g. OCR TextBlock
// or STT TranscriptSegment confidences) that a single Segment's text was
// assembled from into one Segment-level Confidence score: the unweighted
// arithmetic mean, clamped to [0, 1]. Returns DefaultConfidence for an empty
// input, matching the "no upstream signal" default.
func AggregateConfidence(unitConfidences []float64) float64 {
	if len(unitConfidences) == 0 {
		return DefaultConfidence
	}
	var sum float64
	for _, c := range unitConfidences {
		sum += c
	}
	return ClampConfidence(sum / float64(len(unitConfidences)))
}

// LowConfidenceSegments returns the subset of segs whose Confidence is
// strictly below threshold, preserving order. Useful for flagging portions
// of a segmentation result that may need human review, mirroring
// packages/ocr's LowConfidenceBlocks.
func LowConfidenceSegments(segs []Segment, threshold float64) []Segment {
	var out []Segment
	for _, s := range segs {
		if s.Confidence < threshold {
			out = append(out, s)
		}
	}
	return out
}
