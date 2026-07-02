package category

import (
	"context"
	"testing"
)

func TestKeywordSuggester_Suggest(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")

	tests := []struct {
		name        string
		text        string
		wantTop     CategoryCode
		wantAtLeast int
	}{
		{
			name:        "civil contract dispute",
			text:        "The plaintiff alleges a breach of contract and seeks damages.",
			wantTop:     CodeCivil,
			wantAtLeast: 1,
		},
		{
			name:        "criminal prosecution",
			text:        "The prosecution charged the accused with a felony under the penal code.",
			wantTop:     CodeCriminal,
			wantAtLeast: 1,
		},
		{
			name:        "domestic violence",
			text:        "The petitioner sought a protective order alleging domestic violence by her cohabitant.",
			wantTop:     CodeDomesticViolence,
			wantAtLeast: 1,
		},
		{
			name:        "consumer complaint",
			text:        "The consumer complaint alleges a defective product and deceptive advertising.",
			wantTop:     CodeConsumer,
			wantAtLeast: 1,
		},
		{
			name:        "family divorce",
			text:        "The parties seek divorce and a resolution of child custody and child support.",
			wantTop:     CodeFamily,
			wantAtLeast: 1,
		},
		{
			name:        "no keyword matches",
			text:        "The weather was pleasant on the day of the hearing.",
			wantAtLeast: 0,
		},
	}

	s := NewKeywordSuggester()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Suggest(context.Background(), tt.text, tax)
			if err != nil {
				t.Fatalf("Suggest() error = %v, want nil", err)
			}
			if len(got) < tt.wantAtLeast {
				t.Fatalf("got %d suggestions, want at least %d", len(got), tt.wantAtLeast)
			}
			if tt.wantTop != "" {
				if len(got) == 0 {
					t.Fatalf("got no suggestions, want top suggestion %q", tt.wantTop)
				}
				if got[0].Category.Code != tt.wantTop {
					t.Errorf("top suggestion = %q, want %q", got[0].Category.Code, tt.wantTop)
				}
			}
		})
	}
}

func TestKeywordSuggester_Suggest_EmptyInput(t *testing.T) {
	s := NewKeywordSuggester()
	tax := NewDefaultTaxonomy("IN")

	tests := []string{"", "   ", "\t\n"}
	for _, text := range tests {
		_, err := s.Suggest(context.Background(), text, tax)
		if err != ErrEmptyInput {
			t.Errorf("Suggest(%q) error = %v, want %v", text, err, ErrEmptyInput)
		}
	}
}

func TestKeywordSuggester_Suggest_ConfidenceBounds(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")
	s := NewKeywordSuggester()

	text := "breach of contract, tort, negligence, civil suit, plaintiff, damages, injunction, specific performance"
	got, err := s.Suggest(context.Background(), text, tax)
	if err != nil {
		t.Fatalf("Suggest() error = %v, want nil", err)
	}
	if len(got) == 0 {
		t.Fatal("got no suggestions")
	}
	for _, sg := range got {
		if sg.Confidence < 0 || sg.Confidence > 1 {
			t.Errorf("suggestion %q confidence = %v, want within [0, 1]", sg.Category.Code, sg.Confidence)
		}
	}
	// Many keyword hits for civil should cap at 0.95, never claim full 1.0
	// certainty (reserved for ManualOverride).
	if got[0].Confidence > 0.95 {
		t.Errorf("top suggestion confidence = %v, want <= 0.95", got[0].Confidence)
	}
}

func TestKeywordSuggester_Suggest_SortedDescending(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")
	s := NewKeywordSuggester()

	text := "The prosecution charged the accused with a felony. Separately, the plaintiff alleges breach of contract."
	got, err := s.Suggest(context.Background(), text, tax)
	if err != nil {
		t.Fatalf("Suggest() error = %v, want nil", err)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Confidence < got[i].Confidence {
			t.Errorf("suggestions not sorted by descending confidence at index %d: %v < %v", i, got[i-1].Confidence, got[i].Confidence)
		}
	}
}

func TestKeywordSuggester_Suggest_ScopedToTaxonomy(t *testing.T) {
	// A taxonomy that only registers "civil" should never suggest
	// categories outside that set, even if the text matches other
	// categories' keywords lexically.
	tax := make(Taxonomy)
	if err := tax.AddCategory("XX", Category{Code: CodeCivil, Name: "Civil"}); err != nil {
		t.Fatalf("AddCategory() error = %v", err)
	}

	s := NewKeywordSuggester()
	text := "The prosecution charged the accused with a felony, and the plaintiff separately alleges breach of contract."
	got, err := s.Suggest(context.Background(), text, tax)
	if err != nil {
		t.Fatalf("Suggest() error = %v, want nil", err)
	}
	for _, sg := range got {
		if sg.Category.Code != CodeCivil {
			t.Errorf("got suggestion for %q, want only %q (taxonomy only registers civil)", sg.Category.Code, CodeCivil)
		}
	}
}
