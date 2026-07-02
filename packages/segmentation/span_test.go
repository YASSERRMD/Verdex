package segmentation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestSourceSpan_Len(t *testing.T) {
	tests := []struct {
		name string
		span segmentation.SourceSpan
		want int
	}{
		{"normal", segmentation.SourceSpan{Start: 5, End: 10}, 5},
		{"zero-length", segmentation.SourceSpan{Start: 5, End: 5}, 0},
		{"invalid-end-before-start", segmentation.SourceSpan{Start: 10, End: 5}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.span.Len(); got != tt.want {
				t.Errorf("Len() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSourceSpan_Overlaps(t *testing.T) {
	tests := []struct {
		name string
		a, b segmentation.SourceSpan
		want bool
	}{
		{"adjacent-no-overlap", segmentation.SourceSpan{Start: 0, End: 5}, segmentation.SourceSpan{Start: 5, End: 10}, false},
		{"overlapping", segmentation.SourceSpan{Start: 0, End: 6}, segmentation.SourceSpan{Start: 5, End: 10}, true},
		{"disjoint", segmentation.SourceSpan{Start: 0, End: 5}, segmentation.SourceSpan{Start: 10, End: 15}, false},
		{"identical", segmentation.SourceSpan{Start: 0, End: 5}, segmentation.SourceSpan{Start: 0, End: 5}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Overlaps(tt.b); got != tt.want {
				t.Errorf("Overlaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateSpanCoverage(t *testing.T) {
	tests := []struct {
		name       string
		spans      []segmentation.SourceSpan
		totalRunes int
		wantErr    bool
	}{
		{
			name: "full-coverage-no-gaps",
			spans: []segmentation.SourceSpan{
				{Start: 0, End: 5},
				{Start: 5, End: 10},
				{Start: 10, End: 20},
			},
			totalRunes: 20,
			wantErr:    false,
		},
		{
			name:       "empty-spans-empty-text",
			spans:      nil,
			totalRunes: 0,
			wantErr:    false,
		},
		{
			name:       "empty-spans-nonempty-text",
			spans:      nil,
			totalRunes: 5,
			wantErr:    true,
		},
		{
			name: "gap-between-spans",
			spans: []segmentation.SourceSpan{
				{Start: 0, End: 5},
				{Start: 6, End: 10},
			},
			totalRunes: 10,
			wantErr:    true,
		},
		{
			name: "overlap-between-spans",
			spans: []segmentation.SourceSpan{
				{Start: 0, End: 6},
				{Start: 5, End: 10},
			},
			totalRunes: 10,
			wantErr:    true,
		},
		{
			name: "does-not-start-at-zero",
			spans: []segmentation.SourceSpan{
				{Start: 2, End: 10},
			},
			totalRunes: 10,
			wantErr:    true,
		},
		{
			name: "does-not-end-at-total",
			spans: []segmentation.SourceSpan{
				{Start: 0, End: 8},
			},
			totalRunes: 10,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := segmentation.ValidateSpanCoverage(tt.spans, tt.totalRunes)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSpanCoverage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
