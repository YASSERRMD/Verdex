package evidence

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// EvidenceService orchestrates the full evidence-classification pipeline:
//
//	classify -> detect witness/documentary/statutory subtype
//	  -> attribute party -> apply any override -> persist
//	  -> return []Classification
//
// This mirrors packages/segmentation's SegmentationService and
// packages/pii's PIIService orchestration pattern: a single entry point
// wires together this package's otherwise independent, individually
// testable building blocks (Classifier, AttributeParty, ApplyOverride,
// ClassificationStore).
//
// The subtype detection (witness/documentary/statutory) and party
// attribution are already performed inside Classifier.Classify (see
// classifier.go); EvidenceService's job is to run that classification for
// every segment, layer in any caller-supplied ManualOverride, persist the
// result, and return the full batch.
type EvidenceService struct {
	// Classifier assigns an evidentiary Classification to each segment. If
	// nil, NewRuleBasedClassifier() is used.
	Classifier Classifier

	// Store persists every produced Classification. If nil,
	// NewInMemoryClassificationStore() is used.
	Store ClassificationStore
}

// NewEvidenceService constructs an EvidenceService with sensible defaults
// for every pluggable dependency left nil: RuleBasedClassifier and
// InMemoryClassificationStore.
func NewEvidenceService() *EvidenceService {
	return &EvidenceService{
		Classifier: NewRuleBasedClassifier(),
		Store:      NewInMemoryClassificationStore(),
	}
}

// ClassifyRequest carries the input to EvidenceService.ClassifySegments.
type ClassifyRequest struct {
	// Segments is the batch of segmentation.Segment values to classify, in
	// any order.
	Segments []segmentation.Segment

	// Overrides optionally supplies a ManualOverride to apply for specific
	// segment IDs, keyed by SegmentID. A segment with no entry here is
	// classified purely by Classifier.
	Overrides map[string]ManualOverride
}

// ClassifySegments runs the full pipeline over every segment in req.Segments:
// classify (which internally detects witness/documentary/statutory subtype
// and attributes party), apply any matching override from req.Overrides,
// persist via Store, and return the resulting Classification for every
// segment, in the same order as req.Segments.
//
// Segments with empty (whitespace-only) text are skipped (per
// Classifier.Classify's ErrEmptyInput) rather than aborting the whole
// batch; any other Classifier or Store error aborts the batch and is
// returned immediately.
func (s *EvidenceService) ClassifySegments(ctx context.Context, req ClassifyRequest) ([]Classification, error) {
	classifier := s.Classifier
	if classifier == nil {
		classifier = NewRuleBasedClassifier()
	}
	store := s.Store
	if store == nil {
		store = NewInMemoryClassificationStore()
	}

	results := make([]Classification, 0, len(req.Segments))
	for _, seg := range req.Segments {
		c, err := classifier.Classify(ctx, seg)
		if err != nil {
			if err == ErrEmptyInput {
				continue
			}
			return nil, err
		}

		if override, ok := req.Overrides[seg.ID]; ok {
			c, err = ApplyOverride(c, override)
			if err != nil {
				return nil, err
			}
		}

		if err := store.Save(ctx, c); err != nil {
			return nil, err
		}
		results = append(results, c)
	}

	return results, nil
}

// ClassifySegment runs the full pipeline for a single segment, equivalent
// to calling ClassifySegments with a one-element batch. Returns
// ErrEmptyInput if seg.Text is empty or whitespace-only.
func (s *EvidenceService) ClassifySegment(ctx context.Context, seg segmentation.Segment, override *ManualOverride) (Classification, error) {
	req := ClassifyRequest{Segments: []segmentation.Segment{seg}}
	if override != nil {
		req.Overrides = map[string]ManualOverride{seg.ID: *override}
	}
	results, err := s.ClassifySegments(ctx, req)
	if err != nil {
		return Classification{}, err
	}
	if len(results) == 0 {
		return Classification{}, ErrEmptyInput
	}
	return results[0], nil
}
