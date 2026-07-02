package multilingual

import "context"

// Translator is the contract every concrete translation adapter must
// satisfy. It mirrors the provider-agnostic pattern used by
// stt.STTProvider and ocr.OCRProvider: Verdex never hardcodes a
// translation vendor or SDK, all translation calls are routed through this
// interface so that adapters (hosted translation APIs, local models, or
// the deterministic NoOpTranslator) can be registered and swapped without
// touching business logic.
//
// Implementations MUST be safe for concurrent use from multiple
// goroutines.
type Translator interface {
	// ID returns the stable, unique identifier for this translator (e.g.
	// "noop", "hosted-mt-v1").
	ID() string

	// Translate converts text (written in source) into targetLanguage.
	// Implementations should honour ctx.Done() and return a wrapped
	// context error promptly. Translate must not mutate or discard the
	// caller's original text; callers (NormalizationService) are
	// responsible for retaining the original alongside the translated
	// result.
	Translate(ctx context.Context, text string, source, targetLanguage Language) (string, error)
}

// NoOpTranslator is the deterministic default Translator. It performs no
// real translation: Translate returns the input text unchanged. It exists
// so callers can wire a Translator into the pipeline without requiring a
// real translation backend, exactly as NoOpSTTProvider and NoOpOCRProvider
// stand in for their respective backends elsewhere in Verdex.
type NoOpTranslator struct{}

// ID implements Translator.
func (NoOpTranslator) ID() string { return "noop" }

// Translate implements Translator by returning text unchanged, regardless
// of source or targetLanguage.
func (NoOpTranslator) Translate(_ context.Context, text string, _, _ Language) (string, error) {
	return text, nil
}

// TranslationResult carries the outcome of an optional translation pass.
//
// Original is ALWAYS populated with the untranslated source text — Verdex
// never discards the source when translating, so that provenance,
// auditing, and any downstream reasoning that must reference the original
// wording remain possible. Translated and TargetLanguage are populated
// only when a non-trivial translation pass actually ran (i.e. Translator
// is not NoOpTranslator and SourceLanguage != TargetLanguage); Applied
// records whether translation was attempted.
type TranslationResult struct {
	// Original is the untranslated source text. Always populated.
	Original string

	// Translated is the translated text. Equal to Original when no
	// translation was applied (e.g. NoOpTranslator, or source and target
	// languages match).
	Translated string

	// SourceLanguage is the language Original is written in.
	SourceLanguage Language

	// TargetLanguage is the language Translated was produced in. Equal to
	// SourceLanguage when no translation was applied.
	TargetLanguage Language

	// Applied reports whether a translation pass was attempted (a
	// Translator other than the trivial identity case was invoked).
	Applied bool
}

// Translate runs an optional translation pass using t, always retaining
// the original text in the returned TranslationResult.Original regardless
// of whether translation succeeds, fails, or is a no-op. If t is nil,
// NoOpTranslator{} is used. If source == target, Translate short-circuits
// without invoking t and returns Applied = false.
func Translate(ctx context.Context, t Translator, text string, source, target Language) (TranslationResult, error) {
	if t == nil {
		t = NoOpTranslator{}
	}

	result := TranslationResult{
		Original:       text,
		Translated:     text,
		SourceLanguage: source,
		TargetLanguage: source,
	}

	if source == target || target == "" || target == LanguageUnknown {
		return result, nil
	}

	translated, err := t.Translate(ctx, text, source, target)
	if err != nil {
		// Original is preserved even on failure; Translated falls back
		// to Original rather than being left inconsistent.
		return result, err
	}

	result.Translated = translated
	result.TargetLanguage = target
	result.Applied = true
	return result, nil
}
