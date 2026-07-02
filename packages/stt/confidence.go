package stt

// ClampConfidence clamps c into the closed interval [0, 1].
func ClampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}

// AverageConfidence returns the unweighted arithmetic mean of every
// segment's Confidence score. It returns 0 for an empty slice.
func AverageConfidence(segments []TranscriptSegment) float64 {
	if len(segments) == 0 {
		return 0
	}
	var sum float64
	for _, s := range segments {
		sum += s.Confidence
	}
	return sum / float64(len(segments))
}

// WeightedConfidence returns the duration-weighted average of every
// segment's Confidence score, weighting each segment by (EndMS - StartMS).
// Segments with non-positive duration are excluded from the weighting; if no
// segment has positive duration this falls back to AverageConfidence.
func WeightedConfidence(segments []TranscriptSegment) float64 {
	var weightedSum float64
	var totalWeight float64
	for _, s := range segments {
		w := float64(s.EndMS - s.StartMS)
		if w <= 0 {
			continue
		}
		weightedSum += s.Confidence * w
		totalWeight += w
	}
	if totalWeight == 0 {
		return AverageConfidence(segments)
	}
	return weightedSum / totalWeight
}

// LowConfidenceSegments returns the subset of segments whose Confidence is
// strictly below threshold, preserving order. Useful for flagging portions
// of a transcript that may need human review.
func LowConfidenceSegments(segments []TranscriptSegment, threshold float64) []TranscriptSegment {
	var out []TranscriptSegment
	for _, s := range segments {
		if s.Confidence < threshold {
			out = append(out, s)
		}
	}
	return out
}
