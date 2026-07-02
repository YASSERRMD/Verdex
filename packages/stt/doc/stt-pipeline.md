# Verdex Speech-to-Text Pipeline

## Overview

`packages/stt` transcribes audio and video artifacts (hearing recordings,
witness statements, oral arguments) into timestamped, confidence-scored text
for the Verdex judicial reasoning platform. Like the LLM provider abstraction
in `packages/provider`, this package never hardcodes a transcription backend:
all transcription is routed through the `STTProvider` interface so that
concrete adapters (local models, hosted STT APIs, or the deterministic
`NoOpSTTProvider` used for tests and air-gapped deployments) can be
registered and swapped without touching business logic.

Source audio is never retained beyond the transcription call. Verdex's
transcribe-and-discard guarantee (the same guarantee `packages/intake`
provides for uploaded files) means the raw bytes are zeroed immediately after
a transcript is produced, with a SHA-256 provenance hash captured beforehand
so downstream systems can still verify what was transcribed without ever
storing the audio itself.

---

## Model-Agnostic Design

### STTProvider

```go
type STTProvider interface {
    ID() string
    Capabilities() Capability
    Transcribe(ctx context.Context, input AudioInput) (*Transcript, error)
}
```

Every adapter — whether it wraps a local model, a hosted API, or a
deterministic stub — implements this single interface. `Capabilities()`
advertises what the provider supports (supported tasks, max audio duration,
native diarization support, supported languages) so callers can select an
appropriate provider without hardcoding assumptions about a specific vendor.

### Registry

`Registry` is a thread-safe map from provider ID to `STTProvider`, mirroring
`provider.Registry`. `DefaultRegistry` is the process-wide registry;
application startup code registers concrete adapters against it (or against
an isolated `*Registry` for tests), and the rest of the codebase resolves
providers by ID through `STTService`.

### NoOpSTTProvider

`NoOpSTTProvider` is a deterministic stub: it never inspects the audio bytes
and derives its (fixed) output purely from declared metadata
(`AudioInput.DurationMS`). It exists for unit tests, CI, and air-gapped
deployments where no real transcription backend is available. Constraint: no
code in this package calls a real external STT API or SDK — that is left
entirely to adapters implemented outside this package.

---

## Pipeline Stages

`STTService.Transcribe` orchestrates the full pipeline:

```
AudioInput
   │
   ▼
Normalize()            ← fills sample-rate/channel defaults from declared metadata
   │
   ▼
ComputeSourceHash()     ← SHA-256 over the raw bytes, captured before any mutation
   │
   ▼
Segment() (optional)    ← splits audio into bounded-duration Chunks when
   │                       STTService.MaxChunkMS is set and audio exceeds it
   ▼
provider.Transcribe()   ← one call per chunk (or one call for the whole input)
   │
   ▼
AssembleTranscript()    ← offsets each chunk's segment timestamps and merges
   │                       them into a single, StartMS-ordered Transcript
   ▼
Diarizer.Diarize()      ← assigns SpeakerLabel values (NoOpDiarizer by default)
   │
   ▼
Discard()               ← zeroes AudioInput.Data, emits DiscardAuditEvent
   │
   ▼
*Transcript              (SourceHash populated, source bytes gone)
```

Discard runs unconditionally once the provider call has returned — even on
transcription failure — so source audio never outlives an attempt.

### Normalization and Segmentation

`Normalize` and `Segment` operate purely on `AudioInput`'s declared metadata
(byte length, `DurationMS`, `SampleRateHz`, `Channels`). No audio codec is
implemented or required: segmentation splits the byte slice proportionally
against the declared duration, which is sufficient to bound the size of a
single provider request without decoding the payload. Segmentation is
deterministic — the same input and `maxChunkMS` always yield the same chunk
boundaries.

### Language Hints

`LanguageHint` is a plain ISO 639-1 string wrapper. `LanguageSet` is a
minimal, dependency-free mirror of the language data a
`jurisdiction.Jurisdiction` carries (`Languages []string`), so this package
has **no hard module dependency on `packages/jurisdiction`**. Callers that
already hold a `*jurisdiction.Jurisdiction` construct a hint with:

```go
hint := stt.DeriveLanguageHint(j.Languages, preferredUILanguage)
```

### Diarization

`Diarizer` is a narrow interface (`Diarize(ctx, segments) ([]TranscriptSegment, error)`)
applied after assembly. `NoOpDiarizer` is the default: it either leaves every
segment's `Speaker` untouched or assigns a single `DefaultSpeakerLabel`,
depending on configuration. Real diarization backends can be plugged in via
the same interface without changing `STTService`.

### Timestamps and Confidence

`TranscriptSegment` carries `StartMS`/`EndMS` (relative to the start of the
original, unsegmented audio) and `Confidence` (a float in `[0, 1]`).
`AssembleTranscript` guarantees the final `Transcript.Segments` slice is
ordered by `StartMS` ascending regardless of how many chunks were
transcribed. Helpers in `confidence.go` (`AverageConfidence`,
`WeightedConfidence`, `LowConfidenceSegments`) let callers aggregate or flag
low-confidence spans for human review.

---

## Transcribe-and-Discard Guarantee

1. `ComputeSourceHash` computes the SHA-256 digest of `AudioInput.Data`
   **before** any provider call or mutation.
2. The provider transcribes the audio (or its chunks).
3. `Discard` zeroes every byte of `AudioInput.Data` in place and truncates
   the slice, rendering the source audio unrecoverable from the struct.
4. A `DiscardAuditEvent` (event type `stt.discarded`) carrying the
   pre-computed `SourceHash`, byte count, provider ID, and timestamp is
   emitted to the configured `AudioDiscardSink`.
5. The returned `*Transcript.SourceHash` is set to the same digest, so
   provenance can be verified without ever retaining the audio bytes.

This mirrors the guarantee `packages/intake` provides for uploaded binary
artifacts: hash first, discard immediately after use, and always emit an
audit trail — regardless of whether the operation that consumed the bytes
succeeded or failed.

---

## Testing Strategy

- `registry_test.go` — register/get/list/duplicate/not-found/nil-provider
  behaviour, mirroring `packages/provider`'s registry tests.
- `normalize_test.go` — default-filling, deterministic bounded segmentation,
  byte-conservation across chunks, and error cases.
- `transcript_test.go` — `FullText`, `SortSegments`, and chunk-offset
  assembly.
- `diarization_test.go` — `NoOpDiarizer` preserves segment count/order/timing
  and only ever mutates `Speaker`.
- `confidence_test.go` — clamping and aggregation helpers.
- `language_test.go` — hint derivation from a jurisdiction's language codes.
- `service_test.go` — end-to-end pipeline: segments ordered, timestamps
  monotonic, confidence in `[0, 1]`, error wrapping for missing providers,
  empty audio, and nil input.
- `discard_test.go` — source bytes are actually zeroed, discard is
  idempotent, an audit event is emitted with the pre-computed hash, and the
  end-to-end service call discards the caller's `AudioInput`.

No test performs a live network call; all providers used in tests are the
deterministic `NoOpSTTProvider`.
