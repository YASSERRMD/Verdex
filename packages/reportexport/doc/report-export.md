# Report export

`packages/reportexport` implements Phase 073: exporting a case's draft
analysis — facts, issues, analysis, and citations — as a structured,
downloadable report in PDF, DOCX, Markdown, or plain-text form.

## Why this package exists now

By this phase, three earlier phases already produce the content a
report needs, but nothing assembles or renders it for a human to take
away from the platform:

| Upstream package             | What it contributes                                          |
| ----------------------------- | -------------------------------------------------------------- |
| `packages/caselifecycle`      | `Case` — title, reference, tenant/jurisdiction identity        |
| `packages/synthesisagent`     | `Opinion` — the draft, non-binding conclusions per issue        |
| `packages/citation`           | `Registry`/`Formatter` — jurisdiction-correct citation text    |
| `packages/reasoningtrace`     | `Trace` — the full auditable narrative behind those conclusions |
| `packages/guardrail`          | The mandatory non-binding disclaimer text                       |
| `packages/pii`                | PII detection/redaction, reused for an optional export pass     |

This package's job is narrow and deliberate: assemble those into one
`Report` value, render that `Report` into bytes in four formats, and
record every export attempt — never re-deriving analysis, citation
formatting, disclaimer wording, or PII detection that another package
already owns.

## The model

```
Report
  CaseID, TenantID, CaseTitle, CaseReference
  JurisdictionKey    string            (opaque key into a citation.Registry)
  Issues             []ReportIssue
  SkippedIssueNodeIDs []string
  TraceAppendix      string            (reasoningtrace.ExportMarkdown output, or empty)
  OpinionGeneratedAt, AssembledAt time.Time

ReportIssue
  IssueNodeID, Analysis, FavoredParty, Confidence, WeakestLink
  SupportingFactIDs  []string
  Citations          []ReportCitation

ReportCitation
  RuleID, Text (jurisdiction-formatted), Resolved, Verified
```

`Assemble(c *caselifecycle.Case, opinion *synthesisagent.Opinion, input AssembleInput) (*Report, error)`
is the single entry point tying a case and its opinion together. Every
`TentativeConclusion` in `opinion.Conclusions` becomes one
`ReportIssue`, copying `Text`, `FavoredParty`, `Confidence`,
`WeakestLink`, and `SupportingFactIDs` verbatim — this package never
rewrites or reinterprets synthesis output.

Citations are supplied via `AssembleInput.AuthorityTrailsByIssue`
(issue node ID -> `[]AuthorityCitationInput`) and formatted through a
`citation.Registry` keyed by `AssembleInput.JurisdictionKey` — the
same `Registry.Format` jurisdiction-family lookup
`packages/citation` already exposes. If no registry is supplied,
`citation.NewDefaultRegistry()` (common-law/civil-law) is used. An
unregistered jurisdiction key with no fallback formatter falls back to
the citation's raw text rather than silently dropping it.

### Attaching a reasoning-trace appendix

`WithTrace(input AssembleInput, trace reasoningtrace.Trace) (AssembleInput, error)`
populates both `TraceAppendix` (via `reasoningtrace.ExportMarkdown` —
embedded verbatim, never re-derived) and `AuthorityTrailsByIssue` (from
`trace.AuthorityTrails`) in one call:

```go
input, err := reportexport.WithTrace(reportexport.AssembleInput{
    JurisdictionKey: "common_law",
}, trace)
report, err := reportexport.Assemble(caseRecord, opinion, input)
```

## Rendering

Four renderers, one per `Format` constant, each embedding the mandatory
non-binding disclaimer via `guardrail.RequireDisclaimer` — the same
function every other human-facing output surface in this platform
uses:

- **`FormatMarkdown`** (`RenderMarkdown`) — a standalone Markdown
  document: title, one section per issue, a skipped-issues section, the
  trace appendix, and the disclaimer.
- **`FormatText`** (`RenderText`) — the same content with Markdown
  emphasis/heading syntax stripped.
- **`FormatPDF`** (`RenderPDF`) — a real PDF via
  `github.com/jung-kurt/gofpdf`, a small, well-known, pure-Go PDF
  generator (no cgo, no external renderer process, no license
  concerns for this platform). Page content-stream compression is
  deliberately disabled (`pdf.SetCompression(false)`) so every page's
  text-showing operators stay as literal, greppable bytes — this is
  what lets tests (and any downstream consumer) verify a PDF actually
  contains expected text without a full PDF-parsing dependency.
  Output always starts with the `%PDF-` magic header.
- **`FormatDOCX`** (`RenderDOCX`) — a real Office Open XML document
  built directly from the standard library (`archive/zip` +
  `encoding/xml`), not a third-party DOCX package: a `.docx` is just a
  zip archive of a small, stable set of XML parts
  (`[Content_Types].xml`, `_rels/.rels`, `word/document.xml`,
  `word/_rels/document.xml.rels`), and the plain-paragraph subset a
  report needs doesn't warrant an external dependency. Every
  paragraph's text is escaped via `encoding/xml.EscapeText` before
  being written into a `<w:t>` run, so report content (including
  redaction placeholders and citation punctuation) can never break the
  XML structure. Output always starts with the `PK` zip signature.

## Redaction

`Redact(ctx, report, opts RedactionOptions) (*Report, error)` returns a
copy of `report` with every free-text field (`Issue.Analysis`,
`Issue.WeakestLink`, `TraceAppendix`) passed through
`pii.PIIService.Process` — this package's redaction pass is a thin
wrapper over `packages/pii`'s own detect-classify-redact pipeline, not
a reimplementation. Structural fields (IDs, citation text, party
labels, confidence) are left untouched, since PII does not originate
there. The original `Report` is never mutated.

## Export audit

Every `Service.Export` call — redacted or not — persists an
`AuditRecord`:

```
AuditRecord
  ID, TenantID, CaseID, ActorID
  Format     (pdf | docx | markdown | text)
  Redacted   bool
  ExportedAt time.Time
```

`AuditRepository` is tenant-scoped exactly like
`packages/notifications.Repository`; `InMemoryAuditRepository` is the
in-process implementation for tests and fixtures. `ToAuditEvent`
projects an `AuditRecord` onto `observability.AuditEvent` so exports
flow through the platform's one audit channel rather than a second,
parallel logging path. `Service.AuditLog` exposes the queryable
history, filterable by case, actor, format, and time.

## Access control

`Service.Export` and `Service.AuditLog` are gated on
`identity.PermViewCase` — the same permission that already gates
reading a case's details, filings, and attached documents. Exporting a
report is a read of the case's analysis, not a mutation; there is no
separate export-specific permission in the RBAC matrix.

## End-to-end example

```go
input, err := reportexport.WithTrace(reportexport.AssembleInput{
    JurisdictionKey: "common_law",
    Citations:       citation.NewDefaultRegistry(),
}, trace)
report, err := reportexport.Assemble(caseRecord, opinion, input)

svc, err := reportexport.NewService(reportexport.NewInMemoryAuditRepository())
result, err := svc.Export(ctx, reportexport.ExportRequest{
    Report: report,
    Format: reportexport.FormatPDF,
    Redact: true,
})
// result.Bytes is a valid PDF; result.AuditRecord records who/when/format/redacted.
```

## Web UI

`apps/web`'s case workspace `ReasoningOpinionPanel` already had a
client-side "export as Markdown/text/JSON" control (Phase 067) built
from data already in the browser. This phase extends that control with
a redaction toggle and PDF/DOCX export helpers on the client so a
reviewer can pull a formatted copy of the same opinion currently on
screen without leaving the workspace.
