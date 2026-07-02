# Verdex Multilingual Normalization Rules

## Overview

`packages/multilingual` normalizes Tamil, Urdu, Arabic, and English
transcripts/text so downstream segmentation and reasoning stages in the
Verdex judicial reasoning platform operate over consistent, well-formed
text regardless of source script. Like `packages/stt`'s `STTProvider` and
`packages/ocr`'s `OCRProvider`, every pluggable concern (transliteration,
translation) is routed through a narrow interface with a deterministic
no-op default — this package never hardcodes a translation vendor, SDK, or
a complete transliteration scheme.

No component in this package uses a machine-learning model. Script
detection, language detection, and tokenization are all deterministic,
rule-based functions of Unicode code points.

---

## Pipeline Stages

`NormalizationService.Normalize` orchestrates the full pipeline:

```
text
  │
  ▼
NormalizeUnicode()        ← NFC/NFKC + whitespace/control-char cleanup
  │
  ▼
DetectScript / DetectLanguage()  ← deterministic code-point-range classifier
  │
  ▼
DetectRTLRuns()            ← right-to-left run detection, IsRTL flag
  │
  ▼
LegalTermNormalizer.NormalizeText() ← canonicalize legal vocabulary
  │
  ▼
Translate()                 ← optional; ORIGINAL TEXT ALWAYS RETAINED
  │
  ▼
Tokenizer.Tokenize()        ← language-aware word-boundary rules
  │
  ▼
emit AuditEvent per step
  │
  ▼
*NormalizedText             (Original + Text + Tokens + AuditTrail)
```

Every step emits an `AuditEvent` to the configured `AuditSink`, mirroring
`packages/intake`'s `IntakeAuditEvent` pattern, so the full normalization
timeline for a document can be reconstructed later.

---

## Per-Language Behaviour

### English (`LanguageEnglish`)

- **Script**: `ScriptLatin`, detected via the Unicode `Latin` code-point
  range.
- **Directionality**: left-to-right; never flagged `IsRTL`.
- **Tokenization**: whitespace and punctuation split tokens, except the
  in-word apostrophe (`'`) and hyphen (`-`), which are retained so
  `"petitioner's"` and `"co-accused"` remain single tokens.
- **Legal terms**: abbreviations and title-case variants (e.g. `"FIR"`,
  `"fir"`, `"first information report"`) canonicalize to a single form
  (`"First Information Report"`).

### Arabic (`LanguageArabic`)

- **Script**: `ScriptArabic`, detected via the Unicode `Arabic` code-point
  range. Shared with Urdu — see disambiguation below.
- **Directionality**: right-to-left. `DetectRTLRuns` tags Arabic-script
  runs `IsRTL: true`; `WrapWithBidiControls` can wrap a run in the
  standard Unicode `RLE ... PDF` embedding markers for renderers that do
  not run their own bidi algorithm.
- **Tokenization**: whitespace-separated, like Latin script, but the
  Arabic tatweel character (`ARABIC TATWEEL`, U+0640) and zero-width
  joiner/non-joiner (U+200C/U+200D) are treated as part of the
  surrounding token rather than as separators, since they modify letter
  joining rather than marking a word boundary.
- **Legal terms**: canonicalized via a pluggable Arabic-language mapping
  (e.g. unifying spelling variants of "the court of first instance").

### Urdu (`LanguageUrdu`)

- **Script**: also `ScriptArabic` — Urdu is written in a Perso-Arabic
  script that is a superset of standard Arabic orthography.
- **Language disambiguation**: `DetectLanguage` distinguishes Urdu from
  Arabic by checking for a small, well-known set of Urdu-only Arabic-script
  letters (retroflex consonants such as `ٹ`/`ڈ`/`ڑ`, `گ` gaf, `ھ` do-chashmi
  he, `ی`/`ے` Farsi/bari ye, etc. — see `urduOnlyRanges` in `script.go`).
  Their presence is treated as a deterministic signal of Urdu; their
  absence defaults to Arabic. This is a heuristic sufficient for routing
  purposes, not a claim of perfect linguistic classification.
- **Directionality**: right-to-left, identical handling to Arabic (Urdu
  shares the `ScriptArabic` RTL classification).
- **Tokenization**: identical rule set to Arabic (shared script-level
  joining-mark handling).
- **Legal terms**: canonicalized via a separate, pluggable Urdu-language
  mapping — Urdu legal vocabulary is not assumed to match Arabic legal
  vocabulary even where scripts overlap.

### Tamil (`LanguageTamil`)

- **Script**: `ScriptTamil`, detected via the Unicode `Tamil` code-point
  range.
- **Directionality**: left-to-right; never flagged `IsRTL`.
- **Tokenization**: whitespace-separated, but Tamil combining vowel signs
  and virama (the "pulli" that suppresses an inherent vowel) are Unicode
  non-spacing marks (`unicode.Mn`) and are kept attached to their base
  consonant rather than treated as separators, so a single orthographic
  syllable (e.g. a consonant + dependent vowel sign) is never split across
  two tokens.
- **Legal terms**: canonicalized via a pluggable Tamil-language mapping.

---

## Unicode Normalization

`NormalizeUnicode` supports two forms:

- **NFC** (`FormNFC`, the default): canonical composition. Keeps the text
  visually and semantically equivalent while ensuring the same underlying
  sequence of code points is produced regardless of how the source encoded
  combining characters.
- **NFKC** (`FormNFKC`): compatibility composition. Additionally folds
  compatibility variants (e.g. full-width Latin letters, certain Arabic
  presentation-form ligatures) to their canonical equivalents — useful
  when exact byte-level term matching (as in `LegalTermNormalizer`) must
  not be defeated by visually-equivalent but distinct code points.

Both forms are implemented via `golang.org/x/text/unicode/norm`, the
standard Go normalization library (chosen over a hand-rolled composition
table, which would be both large and error-prone to maintain correctly).

Alongside normalization form, `NormalizeUnicode` always:

1. Strips C0/C1 control characters (Unicode category `Cc`), other than
   `\t`, `\n`, `\r` which are handled by whitespace collapsing.
2. Collapses any run of Unicode whitespace to a single ASCII space.
3. Trims leading/trailing whitespace.

`NormalizeUnicode` is idempotent: normalizing already-normalized text with
the same form is a no-op.

---

## Transliteration

`Transliterator` is a thin, pluggable interface (`ID()` +
`Transliterate(text, fromScript, toScript)`). Verdex does not embed a
production transliteration scheme in this package — real schemes (ISO
15919 for Tamil, ALA-LC or Buckwalter-style schemes for Arabic/Urdu) are
extensive and best supplied by an adapter outside this package.

Two implementations ship here purely for demonstration and testing:

- `PassthroughTransliterator`: the deterministic default; always returns
  input text unchanged.
- `ExampleTamilTransliterator`: a worked example supporting only
  `ScriptTamil -> ScriptLatin`, backed by a small, explicitly incomplete
  mapping table (`exampleTamilToLatinMap`). It exists to demonstrate how a
  concrete `Transliterator` plugs into the interface, not as a usable
  production transliteration engine.

---

## Translation

`Translator` mirrors `stt.STTProvider`'s provider-agnostic pattern: no
translation vendor or SDK is referenced anywhere in this package, only the
interface (`ID()` + `Translate(ctx, text, source, target)`) plus a
deterministic `NoOpTranslator` default.

The single hard guarantee this package enforces: **the original,
untranslated text is never discarded.** `TranslationResult.Original` is
always populated with the source text, whether or not a real `Translator`
is configured, whether the translation succeeds, fails, or is skipped
because source and target languages already match. `NormalizedText.Text`
(the primary field used by downstream segmentation/reasoning) is always
the source-language, legal-term-normalized text — translation output lives
separately in `NormalizedText.Translation.Translated`, never overwriting
the source.

---

## Testing Strategy

- `unicode_test.go` — NFC/NFKC idempotency, whitespace collapsing,
  control-character stripping, NFKC compatibility folding, across English,
  Arabic, Urdu, and Tamil sample strings.
- `script_test.go` — script/language detection for concrete sample strings
  in all four languages, plus explicit Urdu-vs-Arabic disambiguation cases.
- `transliterate_test.go` — `PassthroughTransliterator` never changes text;
  `ExampleTamilTransliterator` demonstrates its worked-example mapping and
  correctly reports `ErrUnsupportedScript` for unsupported pairs.
- `rtl_test.go` — RTL run detection for pure and mixed-directionality
  text, logical-order preservation (concatenating runs reproduces the
  original text), and bidi control wrapping.
- `legalterm_test.go` — canonicalization across all four languages,
  longest-variant-wins behaviour, and a regression test for the
  replacement-reintroduces-a-match infinite-loop bug fixed during this
  phase.
- `translate_test.go` — original text is always preserved across every
  `Translator` configuration (nil, no-op, failing, and a real
  implementation), including on translation failure.
- `tokenize_test.go` — every sample language produces non-empty tokens;
  English apostrophe/hyphen retention; Arabic/Urdu joining-mark retention;
  Tamil combining-mark retention.
- `audit_test.go` — `NoOpAuditSink`, `CapturingAuditSink`, and
  `LoggingAuditSink` behaviour.
- `service_test.go` — end-to-end `NormalizationService.Normalize` pipeline
  coverage: all four languages, complete audit trail (one event per
  pipeline step), original text retained after translation, and graceful
  handling of a failing `Translator`.

No test depends on network access or a real translation/transliteration
backend; all providers used in tests are deterministic.
