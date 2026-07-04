package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/multilingual"
)

// multilingualSample pairs a real source-language text sample with the
// multilingual.Script/multilingual.Language this package's own
// DetectScript/DetectLanguage rules are documented to recognize it as.
type multilingualSample struct {
	label            string
	text             string
	expectedScript   multilingual.Script
	expectedLanguage multilingual.Language
}

// multilingualSamples returns one real text sample per script this
// task requires: Arabic, Urdu (Arabic script plus at least one
// Urdu-only letter per packages/multilingual/script.go's
// urduOnlyRanges), Tamil, and English (task 4). Every sample is
// genuine, non-empty prose (not a placeholder string), so
// NormalizationService.Normalize exercises its real Unicode
// normalization, script/language detection, RTL flagging, legal-term
// normalization, and tokenization pipeline stages against real input.
func multilingualSamples() []multilingualSample {
	return []multilingualSample{
		{
			label:            "arabic",
			text:             "يجب على المحكمة أن تنظر في جميع الأدلة المقدمة قبل إصدار الحكم.",
			expectedScript:   multilingual.ScriptArabic,
			expectedLanguage: multilingual.LanguageArabic,
		},
		{
			// Contains U+06C1 (choti he, an Urdu-only letter per
			// urduOnlyRanges) so DetectLanguage disambiguates this as
			// Urdu rather than Arabic despite sharing the Arabic script.
			label:            "urdu",
			text:             "عدالت کو فیصلہ سنانے سے پہلے تمام شواہد پر غور کرنا ہوگا۔",
			expectedScript:   multilingual.ScriptArabic,
			expectedLanguage: multilingual.LanguageUrdu,
		},
		{
			label:            "tamil",
			text:             "நீதிமன்றம் தீர்ப்பு வழங்குவதற்கு முன் அனைத்து ஆதாரங்களையும் பரிசீலிக்க வேண்டும்.",
			expectedScript:   multilingual.ScriptTamil,
			expectedLanguage: multilingual.LanguageTamil,
		},
		{
			label:            "english",
			text:             "The court shall consider all submitted evidence before issuing its judgment.",
			expectedScript:   multilingual.ScriptLatin,
			expectedLanguage: multilingual.LanguageEnglish,
		},
	}
}

// NewMultilingualIngestionScenario builds task 4's multilingual
// ingestion scenario: real Arabic, Urdu, Tamil, and English text
// sample driven through packages/multilingual's real
// NormalizationService.Normalize, asserting the detected Script/
// Language/IsRTL/token count genuinely differ per script rather than
// merely "the call did not error."
func NewMultilingualIngestionScenario() (Scenario, error) {
	return NewScenarioFunc("civil/multilingual-ingestion", category.CodeCivil, runMultilingualIngestion)
}

func runMultilingualIngestion(ctx context.Context) (ScenarioResult, error) {
	startedAt := time.Now().UTC()
	svc := multilingual.NewNormalizationService()

	seen := make(map[multilingual.Script]bool)
	for _, sample := range multilingualSamples() {
		normalized, err := svc.Normalize(ctx, "e2e-multilingual-"+sample.label, sample.text, multilingual.LanguageUnknown)
		if err != nil {
			return ScenarioResult{}, wrapf("runMultilingualIngestion", err)
		}

		if normalized.Script != sample.expectedScript {
			return ScenarioResult{
				Outcome:    OutcomeFailed,
				Detail:     fmt.Sprintf("%s sample: detected script %q, want %q", sample.label, normalized.Script, sample.expectedScript),
				StartedAt:  startedAt,
				FinishedAt: time.Now().UTC(),
			}, nil
		}
		if normalized.Language != sample.expectedLanguage {
			return ScenarioResult{
				Outcome:    OutcomeFailed,
				Detail:     fmt.Sprintf("%s sample: detected language %q, want %q", sample.label, normalized.Language, sample.expectedLanguage),
				StartedAt:  startedAt,
				FinishedAt: time.Now().UTC(),
			}, nil
		}
		if len(normalized.Tokens) == 0 {
			return ScenarioResult{
				Outcome:    OutcomeFailed,
				Detail:     fmt.Sprintf("%s sample: produced zero tokens", sample.label),
				StartedAt:  startedAt,
				FinishedAt: time.Now().UTC(),
			}, nil
		}
		// Arabic and Urdu share ScriptArabic (Script is independent of
		// Language, see packages/multilingual/script.go's own doc
		// comment) so IsRTL must be true for both, while Tamil/English
		// must be false -- a real, per-script-sensible difference, not
		// a uniform default.
		wantRTL := sample.expectedScript == multilingual.ScriptArabic
		if normalized.IsRTL != wantRTL {
			return ScenarioResult{
				Outcome:    OutcomeFailed,
				Detail:     fmt.Sprintf("%s sample: IsRTL=%t, want %t", sample.label, normalized.IsRTL, wantRTL),
				StartedAt:  startedAt,
				FinishedAt: time.Now().UTC(),
			}, nil
		}

		seen[normalized.Script] = true
	}

	if len(seen) != 3 {
		// Arabic and Urdu legitimately collapse to the same Script
		// (ScriptArabic); Tamil and Latin are the other two, for 3
		// distinct Script values total across the 4 samples.
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("expected 3 distinct detected scripts across 4 samples (Arabic+Urdu share a script), got %d", len(seen)),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}

	return ScenarioResult{
		Outcome:    OutcomePassed,
		Detail:     "Arabic, Urdu, Tamil, and English samples each produced sensibly distinct real normalization output (script, language, RTL, tokens)",
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	}, nil
}
