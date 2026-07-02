package ontology_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestConcept_Label_FallbackBehavior(t *testing.T) {
	tests := []struct {
		name    string
		concept ontology.Concept
		lang    string
		want    string
	}{
		{
			name: "exact language match",
			concept: ontology.Concept{
				Name:   "Negligence",
				Labels: map[string]string{"en": "Negligence", "ar": "إهمال"},
			},
			lang: "ar",
			want: "إهمال",
		},
		{
			name: "falls back to english label when requested language absent",
			concept: ontology.Concept{
				Name:   "Negligence",
				Labels: map[string]string{"en": "Negligence"},
			},
			lang: "ur",
			want: "Negligence",
		},
		{
			name: "falls back to Name when no labels at all",
			concept: ontology.Concept{
				Name: "Negligence",
			},
			lang: "ta",
			want: "Negligence",
		},
		{
			name: "falls back to Name when labels map present but empty",
			concept: ontology.Concept{
				Name:   "Negligence",
				Labels: map[string]string{},
			},
			lang: "en",
			want: "Negligence",
		},
		{
			name: "requesting english returns english label directly",
			concept: ontology.Concept{
				Name:   "Negligence",
				Labels: map[string]string{"en": "Negligence (EN)"},
			},
			lang: "en",
			want: "Negligence (EN)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.concept.Label(tt.lang)
			if got != tt.want {
				t.Fatalf("Label(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestConcept_SetLabel(t *testing.T) {
	c := ontology.Concept{ID: "civil:negligence", Name: "Negligence"}

	c = c.SetLabel("ar", "إهمال")

	if c.Labels["ar"] != "إهمال" {
		t.Fatalf("SetLabel did not persist: %+v", c.Labels)
	}
	if c.Label("ar") != "إهمال" {
		t.Fatalf("Label(\"ar\") = %q, want %q", c.Label("ar"), "إهمال")
	}
}
