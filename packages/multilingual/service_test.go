package multilingual_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestNormalizationService_Normalize_AllLanguages(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantLanguage multilingual.Language
		wantRTL      bool
	}{
		{"english", "The FIR was filed under the IPC before the magistrate.", multilingual.LanguageEnglish, false},
		{"arabic", "قررت المحكمة الابتدائية رفض الدعوى", multilingual.LanguageArabic, true},
		{"urdu", "عدالت نے درخواست مسترد کر دی", multilingual.LanguageUrdu, true},
		{"tamil", "நீதிமன்றம் மனுவை நிராகரித்தது", multilingual.LanguageTamil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := multilingual.NewNormalizationService()
			sink := &multilingual.CapturingAuditSink{}
			svc.Sink = sink

			result, err := svc.Normalize(context.Background(), "doc-"+tt.name, tt.text, multilingual.LanguageUnknown)
			if err != nil {
				t.Fatalf("Normalize() unexpected error: %v", err)
			}

			if result.Language != tt.wantLanguage {
				t.Errorf("Language = %v, want %v", result.Language, tt.wantLanguage)
			}
			if result.IsRTL != tt.wantRTL {
				t.Errorf("IsRTL = %v, want %v", result.IsRTL, tt.wantRTL)
			}
			if result.Original == "" {
				t.Errorf("Original must not be empty")
			}
			if len(result.Tokens) == 0 {
				t.Errorf("Tokens must not be empty")
			}
			for i, tok := range result.Tokens {
				if tok == "" {
					t.Errorf("token[%d] is empty", i)
				}
			}

			wantSteps := []multilingual.AuditStep{
				multilingual.StepUnicodeNormalized,
				multilingual.StepScriptDetected,
				multilingual.StepRTLFlagged,
				multilingual.StepLegalTermNormalized,
				multilingual.StepTranslated,
				multilingual.StepTokenized,
			}
			if len(sink.Events) != len(wantSteps) {
				t.Fatalf("audit trail has %d events, want %d: %+v", len(sink.Events), len(wantSteps), sink.Events)
			}
			for i, step := range wantSteps {
				if sink.Events[i].Step != step {
					t.Errorf("audit event[%d].Step = %v, want %v", i, sink.Events[i].Step, step)
				}
				if sink.Events[i].Timestamp.IsZero() {
					t.Errorf("audit event[%d].Timestamp is zero", i)
				}
			}
			if len(result.AuditTrail) != len(wantSteps) {
				t.Errorf("result.AuditTrail has %d events, want %d", len(result.AuditTrail), len(wantSteps))
			}
		})
	}
}

func TestNormalizationService_Normalize_OriginalPreservedAfterTranslation(t *testing.T) {
	svc := multilingual.NewNormalizationService()
	svc.Translator = upperTranslator{}

	text := "The petitioner filed an appeal."
	result, err := svc.Normalize(context.Background(), "doc-1", text, multilingual.LanguageArabic)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if result.Original != text {
		t.Errorf("Original = %q, want %q", result.Original, text)
	}
	if !result.Translation.Applied {
		t.Errorf("Translation.Applied = false, want true")
	}
	if result.Translation.Original != text {
		t.Errorf("Translation.Original = %q, want %q", result.Translation.Original, text)
	}
	if result.Translation.Translated == result.Translation.Original {
		t.Errorf("Translation.Translated should differ from Original when a real translator ran")
	}
}

func TestNormalizationService_Normalize_NoTranslationByDefault(t *testing.T) {
	svc := multilingual.NewNormalizationService()

	text := "The petitioner filed an appeal."
	result, err := svc.Normalize(context.Background(), "doc-1", text, multilingual.LanguageUnknown)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if result.Translation.Applied {
		t.Errorf("Translation.Applied = true, want false when no target language given")
	}
	if result.Original != text {
		t.Errorf("Original = %q, want %q", result.Original, text)
	}
}

func TestNormalizationService_Normalize_EmptyInput(t *testing.T) {
	svc := multilingual.NewNormalizationService()

	_, err := svc.Normalize(context.Background(), "doc-1", "   ", multilingual.LanguageUnknown)
	if !errors.Is(err, multilingual.ErrEmptyInput) {
		t.Errorf("Normalize() error = %v, want ErrEmptyInput", err)
	}
}

func TestNormalizationService_Normalize_LegalTermNormalizationApplied(t *testing.T) {
	svc := multilingual.NewNormalizationService()

	result, err := svc.Normalize(context.Background(), "doc-1", "The FIR was registered.", multilingual.LanguageUnknown)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if result.Text == result.Original {
		t.Errorf("Text should differ from Original after legal-term normalization expands FIR")
	}
	want := "The First Information Report was registered."
	if result.Text != want {
		t.Errorf("Text = %q, want %q", result.Text, want)
	}
}

func TestNormalizationService_Normalize_TranslationFailureStillReturnsResult(t *testing.T) {
	svc := multilingual.NewNormalizationService()
	svc.Translator = failingTranslator{}

	text := "The petitioner filed an appeal."
	result, err := svc.Normalize(context.Background(), "doc-1", text, multilingual.LanguageArabic)
	if err == nil {
		t.Fatalf("Normalize() expected error from failing translator, got nil")
	}
	if result == nil {
		t.Fatalf("Normalize() returned nil result alongside translation error, want non-nil result with original text preserved")
	}
	if result.Original != text {
		t.Errorf("Original = %q, want %q", result.Original, text)
	}
	if len(result.Tokens) == 0 {
		t.Errorf("Tokens must still be populated even when translation fails")
	}
}

func TestNormalizationService_Normalize_RTLRunsPopulatedForArabic(t *testing.T) {
	svc := multilingual.NewNormalizationService()

	result, err := svc.Normalize(context.Background(), "doc-1", "قررت المحكمة", multilingual.LanguageUnknown)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if len(result.RTLRuns) == 0 {
		t.Fatalf("RTLRuns is empty, want at least one run")
	}
	if !result.RTLRuns[0].IsRTL {
		t.Errorf("RTLRuns[0].IsRTL = false, want true for Arabic text")
	}
}
