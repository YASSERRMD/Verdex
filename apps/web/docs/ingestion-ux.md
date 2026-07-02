# Verdex Ingestion UX

## Overview

The ingestion UI is the judicial-facing flow for opening a new case and bringing source
materials (documents and audio recordings) into Verdex for transcription, extraction, and
review. It lives under `src/components/ingestion/` and the `/cases/new` route, and follows
the same 4-step wizard shape established by the setup flow at `/setup`.

Everything the ingestion UI surfaces — extracted text, evidence classifications, timeline
events — is **draft material only**. None of it has been reviewed or signed off, and none
of it should ever read as a finding, ruling, or verdict. This mirrors the platform-wide
non-binding guardrail documented in `docs/frontend-architecture.md`.

## Directory Structure

```
apps/web/src/
├── app/cases/new/
│   └── page.tsx                          # 4-step ingestion wizard route
├── components/ingestion/
│   ├── CaseCreationForm.tsx              # Step 1: category + parties
│   ├── FileUploadPanel.tsx               # Step 2: drag-and-drop / picker upload
│   ├── DiscardConfirmationBanner.tsx     # Step 2: hash-then-discard notice
│   ├── IngestionStatusPanel.tsx          # Step 3: live pipeline status
│   ├── ExtractedTextReview.tsx           # Step 4: extracted segment review
│   ├── ClassificationCorrectionPanel.tsx # Step 4: evidence classification override
│   ├── PartyTimelineEditor.tsx           # Step 4: party & timeline editing
│   └── validation.ts                     # Shared validation helpers
└── types/index.ts                        # IngestionStage, UploadedFile, SegmentReview, …
```

## The Four Steps

1. **Case** — `CaseCreationForm` collects the case category and the first/second party
   names, then calls `POST /api/v1/cases` via `apiFetch`. Required-field validation is
   enforced client-side via `validateCaseCreationInput` before submission.

2. **Upload** — `FileUploadPanel` accepts documents and audio recordings via drag-and-drop
   or a file picker, showing a queued-file list with per-file status chips (`queued`,
   `uploading`, `uploaded`, `failed`). Directly above it, `DiscardConfirmationBanner`
   explains — accurately, without overstatement — that each file is cryptographically
   hashed for provenance and then discarded once its content has been transcribed or
   extracted, matching the transcribe-and-discard guarantee implemented in
   `packages/provenance` and `packages/ingestion`. The banner reuses the visual language of
   `Disclaimer.tsx` so the two read as part of the same guardrail family.

3. **Processing** — `IngestionStatusPanel` renders the current pipeline stage (intake,
   extraction, normalize, segment, classify, complete, failed) with a linear progress
   indicator and a breadcrumb trail of stages, following the reassuring
   "processing, please wait" pattern used by upload-progress screens. The component is
   presentation-only: the page that renders it owns polling/subscribing to the job status
   and passes the latest `IngestionStatus` snapshot down as a prop.

4. **Review** — three panels, stacked:
   - `ExtractedTextReview` pages through extracted/transcribed segments, each with a
     source-span reference back to its position in the original artifact.
   - `ClassificationCorrectionPanel` shows the evidence-type and party classification for
     each segment with dropdowns to override either, mirroring
     `packages/evidence`'s `ManualOverride` concept. It calls a stub
     `PUT /api/v1/segments/:id/classification` handler — the real persistence wiring is a
     later phase's concern.
   - `PartyTimelineEditor` lets a reviewer add/edit case parties and timeline events,
     mirroring `packages/timeline`'s `Party`/`Event` concepts, via stub `POST` handlers to
     `/api/v1/parties` and `/api/v1/timeline/events`.

## Shared Types

Ingestion-specific types live in `src/types/index.ts` alongside the rest of the app's
shared types:

- `CaseCategory`, `CaseCreationInput` — case creation form state
- `UploadStatus`, `UploadedFile` — per-file upload state
- `IngestionStage`, `IngestionStatus` — pipeline stage vocabulary, kept in lockstep with
  `packages/ingestion`'s `Stage` constants (`intake`, `extraction`, `normalize`, `segment`,
  `classify`, `complete`, `failed`)
- `SegmentReview` — an extracted segment with a source-span reference
- `EvidenceType`, `PartyRole`, `SegmentClassification` — evidence classification, mirroring
  `packages/evidence`
- `TimelineParty`, `TimelineEvent` — party/timeline editing, mirroring `packages/timeline`

## Validation & Error States

`src/components/ingestion/validation.ts` centralizes validation logic so it can be unit
tested independently of any component:

- `validateCaseCreationInput` — required-field checks for case creation
- `validateFile` — rejects empty files and files over the 100 MB upload limit
- `hasNoFailedUploads` / `allUploadsSettled` — gate progression past the upload step
- `validateParty` / `validateEvent` — required-field and date-format checks for the
  party/timeline editor

Each component renders its own inline error state next to the relevant field
(`role="alert"` for form-level errors, a `failed` status chip plus message for individual
files) and an empty-state message when there is nothing yet to show (no files queued, no
segments extracted, no classifications available, no parties/events added). This keeps the
ingestion UI legible at every stage of a job's lifecycle, including before it has started
and after a partial failure.

## Testing

Tests live in `__tests__/` and follow the existing Jest + `@testing-library/react`
convention established by `LoginForm.test.tsx` and `StepIndicator.test.tsx`:

- `CaseCreationForm.test.tsx`, `FileUploadPanel.test.tsx`, `IngestionStatusPanel.test.tsx`,
  `DiscardConfirmationBanner.test.tsx`, `ExtractedTextReview.test.tsx`,
  `ClassificationCorrectionPanel.test.tsx`, `PartyTimelineEditor.test.tsx` — render and
  interaction tests for each component, including empty states, error states, and
  non-binding-guardrail assertions (status/discard messaging must never use
  verdict/ruling/decision language).
- `validation.test.ts` — unit tests for every exported validation helper.

Run: `npm test` from `apps/web/`.
