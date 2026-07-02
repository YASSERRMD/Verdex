// Package multilingual normalizes Tamil, Urdu, Arabic, and English
// transcripts/text so downstream segmentation and reasoning stages in the
// Verdex judicial reasoning platform operate over consistent, well-formed
// text regardless of source script.
//
// Core concepts:
//
//   - NormalizeUnicode: Unicode NFC/NFKC normalization plus whitespace and
//     control-character cleanup (unicode.go).
//   - Script / Language: deterministic, code-point-range-based
//     classification of a text span's writing script and candidate
//     language (script.go).
//   - Transliterator: a pluggable interface for converting text between
//     scripts, with a deterministic passthrough default (transliterate.go).
//   - RTL handling: right-to-left run detection and bidi metadata for
//     Arabic/Urdu segments (rtl.go).
//   - LegalTermNormalizer: canonicalizes variant spellings/abbreviations of
//     legal vocabulary per language (legalterm.go).
//   - Translator: a provider-agnostic interface (mirroring
//     stt.STTProvider) for an optional translation pass that always
//     retains the original text alongside any translation (translate.go).
//   - Tokenizer: language-aware, rule-based tokenization (tokenize.go).
//   - AuditEvent: a record of every normalization step applied to a piece
//     of text, mirroring packages/intake's audit-event pattern (audit.go).
//   - NormalizationService: orchestrates unicode-normalize -> detect
//     script/language -> RTL-flag -> legal-term-normalize -> optional
//     translate (original retained) -> tokenize -> emit audit trail ->
//     return a NormalizedText result (service.go).
//
// Design principles:
//
//   - No ML models. Script and language detection are deterministic,
//     Unicode-code-point-range classifiers; tokenization is rule-based.
//   - No hardcoded translation provider. Translation is routed through the
//     Translator interface, mirroring the provider-agnostic pattern used
//     by packages/stt (STTProvider) and packages/provider (LLMProvider).
//   - Original text is never discarded. Every NormalizedText result
//     retains the source text even when a translation pass runs.
//
// See doc/normalization-rules.md for a detailed per-language design
// write-up.
package multilingual
