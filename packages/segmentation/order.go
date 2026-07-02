package segmentation

// AssignOrder assigns a stable, zero-based Sequence to each segment in segs
// (in the order given — callers must pass segments already in document
// order) and populates PrevID/NextID linkage fields from each segment's
// neighbors. Segment IDs must already be populated (see AssignIDs) before
// calling AssignOrder, since PrevID/NextID reference IDs.
//
// The first segment's PrevID and the last segment's NextID are left empty.
// AssignOrder returns a new slice; it does not mutate segs.
func AssignOrder(segs []Segment) []Segment {
	out := make([]Segment, len(segs))
	copy(out, segs)

	for i := range out {
		out[i].Sequence = i

		if i > 0 {
			out[i].PrevID = out[i-1].ID
		} else {
			out[i].PrevID = ""
		}
		if i < len(out)-1 {
			out[i].NextID = out[i+1].ID
		} else {
			out[i].NextID = ""
		}
	}

	return out
}

// ValidateOrder reports whether segs is strictly ordered: Sequence values
// are 0, 1, 2, ... in slice order with no gaps or repeats, and PrevID/NextID
// linkage is internally consistent with neighboring IDs. Returns nil if
// valid, or ErrInvalidRequest describing the first inconsistency found.
func ValidateOrder(segs []Segment) error {
	for i, s := range segs {
		if s.Sequence != i {
			return ErrInvalidRequest
		}
		if i > 0 && s.PrevID != segs[i-1].ID {
			return ErrInvalidRequest
		}
		if i == 0 && s.PrevID != "" {
			return ErrInvalidRequest
		}
		if i < len(segs)-1 && s.NextID != segs[i+1].ID {
			return ErrInvalidRequest
		}
		if i == len(segs)-1 && s.NextID != "" {
			return ErrInvalidRequest
		}
	}
	return nil
}
