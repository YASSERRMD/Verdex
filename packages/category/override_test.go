package category

import (
	"testing"
	"time"
)

func TestManualOverride_Validate(t *testing.T) {
	tests := []struct {
		name    string
		o       ManualOverride
		wantErr error
	}{
		{
			name:    "valid override",
			o:       ManualOverride{CaseID: "case-1", Category: Category{Code: CodeCivil, Name: "Civil"}},
			wantErr: nil,
		},
		{
			name:    "empty case id",
			o:       ManualOverride{CaseID: "", Category: Category{Code: CodeCivil}},
			wantErr: ErrInvalidOverride,
		},
		{
			name:    "empty category code",
			o:       ManualOverride{CaseID: "case-1", Category: Category{Code: ""}},
			wantErr: ErrInvalidOverride,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.o.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyOverride(t *testing.T) {
	original := CategoryAssignment{
		CaseID:      "case-1",
		Category:    Category{Code: CodeCivil, Name: "Civil"},
		Confidence:  0.6,
		Suggestions: []Suggestion{{Category: Category{Code: CodeCivil, Name: "Civil"}, Confidence: 0.6}},
	}

	t.Run("valid override takes precedence, retains original", func(t *testing.T) {
		override := ManualOverride{
			CaseID:     "case-1",
			Category:   Category{Code: CodeConsumer, Name: "Consumer"},
			ReviewedBy: "reviewer-1",
			Reason:     "misclassified; is a consumer complaint",
		}

		got, err := ApplyOverride(original, override)
		if err != nil {
			t.Fatalf("ApplyOverride() error = %v, want nil", err)
		}

		if got.Category.Code != CodeConsumer {
			t.Errorf("got category %q, want %q", got.Category.Code, CodeConsumer)
		}
		if got.Confidence != 1.0 {
			t.Errorf("got confidence %v, want 1.0", got.Confidence)
		}
		if got.Override == nil {
			t.Fatal("got nil Override, want non-nil")
		}
		if got.Override.Previous == nil {
			t.Fatal("got nil Override.Previous, want non-nil (original must be retained)")
		}
		if got.Override.Previous.Category.Code != CodeCivil {
			t.Errorf("Override.Previous.Category = %q, want %q (original suggestion retained)", got.Override.Previous.Category.Code, CodeCivil)
		}
		if len(got.Suggestions) != len(original.Suggestions) {
			t.Errorf("got %d suggestions, want %d (suggestions carried forward)", len(got.Suggestions), len(original.Suggestions))
		}
		if got.Override.ReviewedAt.IsZero() {
			t.Error("Override.ReviewedAt is zero, want a set timestamp")
		}
	})

	t.Run("explicit ReviewedAt is preserved", func(t *testing.T) {
		fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		override := ManualOverride{
			CaseID:     "case-1",
			Category:   Category{Code: CodeConsumer, Name: "Consumer"},
			ReviewedAt: fixed,
		}
		got, err := ApplyOverride(original, override)
		if err != nil {
			t.Fatalf("ApplyOverride() error = %v, want nil", err)
		}
		if !got.Override.ReviewedAt.Equal(fixed) {
			t.Errorf("Override.ReviewedAt = %v, want %v", got.Override.ReviewedAt, fixed)
		}
	})

	t.Run("mismatched case id rejected", func(t *testing.T) {
		override := ManualOverride{CaseID: "case-2", Category: Category{Code: CodeConsumer}}
		_, err := ApplyOverride(original, override)
		if err != ErrInvalidOverride {
			t.Errorf("ApplyOverride() error = %v, want %v", err, ErrInvalidOverride)
		}
	})

	t.Run("invalid override rejected", func(t *testing.T) {
		override := ManualOverride{CaseID: "case-1", Category: Category{Code: ""}}
		_, err := ApplyOverride(original, override)
		if err != ErrInvalidOverride {
			t.Errorf("ApplyOverride() error = %v, want %v", err, ErrInvalidOverride)
		}
	})

	t.Run("previous does not nest a prior override", func(t *testing.T) {
		firstOverride := ManualOverride{CaseID: "case-1", Category: Category{Code: CodeConsumer, Name: "Consumer"}}
		once, err := ApplyOverride(original, firstOverride)
		if err != nil {
			t.Fatalf("first ApplyOverride() error = %v", err)
		}

		secondOverride := ManualOverride{CaseID: "case-1", Category: Category{Code: CodeFamily, Name: "Family"}}
		twice, err := ApplyOverride(once, secondOverride)
		if err != nil {
			t.Fatalf("second ApplyOverride() error = %v", err)
		}

		if twice.Override.Previous.Override != nil {
			t.Error("Override.Previous.Override is non-nil, want nil (no nested overrides)")
		}
	})
}
