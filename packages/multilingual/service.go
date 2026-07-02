package multilingual

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NormalizedText is the result of running NormalizationService.Normalize
// over a piece of source text.
type NormalizedText struct {
	// Original is the untranslated, source-script text after Unicode
	// normalization and cleanup, but before legal-term normalization.
	// Always populated; never discarded, even when translation runs.
	Original string

	// Text is the fully normalized text: Unicode-normalized,
	// legal-term-normalized, in the source language (NOT translated).
	// This is the primary field downstream segmentation/reasoning stages
	// should consume when they want source-language text.
	Text string

	// Script is the detected dominant Script of Original.
	Script Script

	// Language is the detected candidate Language of Original.
	Language Language

	// RTLRuns is the sequence of directionality-tagged TextRuns derived
	// from Text.
	RTLRuns []TextRun

	// IsRTL reports whether Language is a right-to-left language overall
	// (true for Arabic and Urdu), for callers that want a single flag
	// rather than per-run detail.
	IsRTL bool

	// Translation carries the optional translation pass result.
	// Translation.Original always equals Original — the untranslated
	// source is never discarded even when Translation.Applied is true.
	Translation TranslationResult

	// Tokens is the tokenized form of Text (source-language, legal-term
	// normalized), produced by the configured Tokenizer.
	Tokens []string

	// AuditTrail is the ordered sequence of AuditEvents emitted while
	// producing this result.
	AuditTrail []AuditEvent
}

// NormalizationService orchestrates the full multilingual normalization
// pipeline:
//
//	unicode-normalize -> detect script/language -> RTL-flag ->
//	legal-term-normalize -> optional translate (original retained) ->
//	tokenize -> emit audit trail -> return NormalizedText
//
// It is the primary entry point application code should use rather than
// calling NormalizeUnicode, DetectLanguage, LegalTermNormalizer, Translate,
// and a Tokenizer directly.
type NormalizationService struct {
	// Form is the Unicode normalization form applied. Defaults to FormNFC
	// if the zero value.
	Form NormalizeForm

	// TermNormalizer canonicalizes legal vocabulary. If nil, a
	// NewLegalTermNormalizer(DefaultLegalTermSeed()) instance is used.
	TermNormalizer *LegalTermNormalizer

	// Translator performs the optional translation pass. If nil,
	// NoOpTranslator{} is used.
	Translator Translator

	// Tokenizer splits normalized text into tokens. If nil,
	// RuleBasedTokenizer{} is used.
	Tokenizer Tokenizer

	// Sink receives AuditEvents emitted during normalization. If nil,
	// NoOpAuditSink{} is used.
	Sink AuditSink
}

// NewNormalizationService constructs a NormalizationService with sensible
// defaults for every pluggable dependency left nil.
func NewNormalizationService() *NormalizationService {
	return &NormalizationService{
		Form:           FormNFC,
		TermNormalizer: NewLegalTermNormalizer(DefaultLegalTermSeed()),
		Translator:     NoOpTranslator{},
		Tokenizer:      RuleBasedTokenizer{},
		Sink:           NoOpAuditSink{},
	}
}

// Normalize runs the full pipeline over text for documentID (used only to
// correlate audit events; may be empty) and an optional targetLanguage for
// the translation pass (pass LanguageUnknown or "" to skip translation).
//
// Normalize returns ErrEmptyInput if text is empty or whitespace-only. If
// the translation pass fails, Normalize still returns a fully populated
// NormalizedText (with Translation.Applied == false and Original/Text
// intact) alongside the wrapped translation error, so callers can choose
// to proceed with source-language text rather than losing all pipeline
// output.
func (s *NormalizationService) Normalize(ctx context.Context, documentID, text string, targetLanguage Language) (*NormalizedText, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyInput
	}

	form := s.Form
	if form == "" {
		form = FormNFC
	}
	termNormalizer := s.TermNormalizer
	if termNormalizer == nil {
		termNormalizer = NewLegalTermNormalizer(DefaultLegalTermSeed())
	}
	translator := s.Translator
	if translator == nil {
		translator = NoOpTranslator{}
	}
	tokenizer := s.Tokenizer
	if tokenizer == nil {
		tokenizer = RuleBasedTokenizer{}
	}
	sink := s.Sink
	if sink == nil {
		sink = NoOpAuditSink{}
	}

	var trail []AuditEvent
	emit := func(step AuditStep, lang Language, detail string) {
		event := AuditEvent{
			Step:       step,
			DocumentID: documentID,
			Language:   lang,
			Detail:     detail,
			Timestamp:  time.Now().UTC(),
		}
		trail = append(trail, event)
		// Best-effort: an audit sink failure must not abort
		// normalization. Callers that need to observe sink errors
		// should use a Sink implementation that surfaces them
		// out-of-band (e.g. by recording into a shared slice, as
		// CapturingAuditSink does).
		_ = sink.Emit(ctx, event)
	}

	// 1. Unicode normalization.
	unicodeNormalized := NormalizeUnicode(text, form)
	emit(StepUnicodeNormalized, LanguageUnknown, fmt.Sprintf("form=%s", form))

	// 2. Script/language detection.
	script := DetectScript(unicodeNormalized)
	language := DetectLanguage(unicodeNormalized)
	emit(StepScriptDetected, language, fmt.Sprintf("script=%s language=%s", script, language))

	// 3. RTL flagging.
	runs := DetectRTLRuns(unicodeNormalized)
	isRTL := IsRTLScript(script)
	emit(StepRTLFlagged, language, fmt.Sprintf("is_rtl=%t runs=%d", isRTL, len(runs)))

	// 4. Legal-term normalization.
	termNormalized := termNormalizer.NormalizeText(language, unicodeNormalized)
	emit(StepLegalTermNormalized, language, fmt.Sprintf("changed=%t", termNormalized != unicodeNormalized))

	// 5. Optional translation pass (original always retained).
	translation, translateErr := Translate(ctx, translator, unicodeNormalized, language, targetLanguage)
	emit(StepTranslated, language, fmt.Sprintf("applied=%t target=%s", translation.Applied, translation.TargetLanguage))

	// 6. Tokenize.
	tokens := tokenizer.Tokenize(termNormalized, language)
	emit(StepTokenized, language, fmt.Sprintf("tokens=%d", len(tokens)))

	result := &NormalizedText{
		Original:    unicodeNormalized,
		Text:        termNormalized,
		Script:      script,
		Language:    language,
		RTLRuns:     runs,
		IsRTL:       isRTL,
		Translation: translation,
		Tokens:      tokens,
		AuditTrail:  trail,
	}

	if translateErr != nil {
		return result, fmt.Errorf("multilingual: translate: %w", translateErr)
	}
	return result, nil
}
