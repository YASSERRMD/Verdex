package evidenceweighing_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

func TestClassifyEvidenceKind(t *testing.T) {
	tests := []struct {
		name string
		text string
		want evidenceweighing.EvidenceKind
	}{
		{
			name: "testimony keyword",
			text: "The witness testified that the light was red.",
			want: evidenceweighing.EvidenceKindTestimony,
		},
		{
			name: "documentary keyword",
			text: "The contract was signed on January 5th.",
			want: evidenceweighing.EvidenceKindDocumentary,
		},
		{
			name: "documentary wins when both present",
			text: "The witness testified that the contract was signed.",
			want: evidenceweighing.EvidenceKindDocumentary,
		},
		{
			name: "unknown when neither present",
			text: "The car was blue.",
			want: evidenceweighing.EvidenceKindUnknown,
		},
		{
			name: "unknown for empty text",
			text: "",
			want: evidenceweighing.EvidenceKindUnknown,
		},
		{
			name: "case-insensitive match",
			text: "According to the neighbor, the dog barked all night.",
			want: evidenceweighing.EvidenceKindTestimony,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evidenceweighing.ClassifyEvidenceKind(tt.text)
			if got != tt.want {
				t.Errorf("ClassifyEvidenceKind(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
