package issue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/issue"
	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestRuleBasedIdentifier_Identify_NoSegments(t *testing.T) {
	id := issue.NewRuleBasedIdentifier()
	_, err := id.Identify(context.Background(), nil)
	if !errors.Is(err, issue.ErrNoSegments) {
		t.Fatalf("expected ErrNoSegments, got %v", err)
	}
}

func TestRuleBasedIdentifier_Identify_DisputeLanguageRecall(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"whether marker", "The court must decide whether the contract was validly terminated."},
		{"dispute marker", "The amount of damages is in dispute between the parties."},
		{"claims that marker", "The plaintiff claims that the defendant breached the lease."},
		{"denies marker", "The defendant denies ever receiving the notice."},
		{"alleges marker", "The prosecution alleges that the defendant was present at the scene."},
		{"contends marker", "Counsel contends that the delay was excusable."},
	}

	id := issue.NewRuleBasedIdentifier()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			segs := []segmentation.Segment{
				{ID: "seg-1", Type: segmentation.SegmentParagraph, Text: tc.text},
			}
			got, err := id.Identify(context.Background(), segs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) == 0 {
				t.Fatalf("expected at least one candidate issue for text %q, got none", tc.text)
			}
			if got[0].Confidence <= 0 || got[0].Confidence > 1 {
				t.Errorf("expected confidence in (0,1], got %v", got[0].Confidence)
			}
			if len(got[0].SourceSpans) == 0 {
				t.Errorf("expected at least one source span")
			}
		})
	}
}

func TestRuleBasedIdentifier_Identify_NoFalsePositiveOnPlainText(t *testing.T) {
	id := issue.NewRuleBasedIdentifier()
	segs := []segmentation.Segment{
		{ID: "seg-1", Type: segmentation.SegmentParagraph, Text: "The hearing began at nine in the morning."},
	}
	got, err := id.Identify(context.Background(), segs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no candidate issues for plain text, got %d", len(got))
	}
}

func TestRuleBasedIdentifier_Identify_ContradictoryStatementPairs(t *testing.T) {
	id := issue.NewRuleBasedIdentifier()
	segs := []segmentation.Segment{
		{
			ID:           "seg-1",
			Type:         segmentation.SegmentStatement,
			SpeakerLabel: "plaintiff",
			Text:         "I received the deposit in full on the first of the month.",
		},
		{
			ID:           "seg-2",
			Type:         segmentation.SegmentStatement,
			SpeakerLabel: "defendant",
			Text:         "I did not receive any deposit from the tenant.",
		},
	}
	got, err := id.Identify(context.Background(), segs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, c := range got {
		if len(c.SourceSpans) == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a contradictory-pair candidate issue spanning both segments, got %+v", got)
	}
}

func TestRuleBasedIdentifier_Identify_SkipsEmptySegments(t *testing.T) {
	id := issue.NewRuleBasedIdentifier()
	segs := []segmentation.Segment{
		{ID: "seg-1", Type: segmentation.SegmentParagraph, Text: "   "},
	}
	got, err := id.Identify(context.Background(), segs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no candidates for blank segment, got %d", len(got))
	}
}
