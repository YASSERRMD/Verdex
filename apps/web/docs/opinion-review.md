# Draft Reasoned Opinion Review

## Overview

`ReasoningOpinionPanel` (mounted on the Case Workspace's "Draft Opinion" tab, see
`docs/case-workspace-ux.md`) is the full reviewer UI for a case's draft reasoned opinion:
one section per issue with both parties' arguments shown side by side, evidence weights
shown inline, weakest-link/uncertainty callouts where uncertainty exists, a trace-link back
to the reasoning tree for every conclusion, a per-issue judge comment box, and export
controls — all beneath the always-rendered, always-first `Disclaimer` (Phase 057 guardrail).

This phase (067) fills in the placeholder/loading-only panel built in Phase 064
(`data-testid="reasoning-opinion-placeholder"`). The empty/loading state contract Phase 064
established is preserved exactly, plus a new `hasDraftOpinion`-only intermediate state and
the new fully-populated `opinion`-driven state.

As with every other reasoning surface in the app, everything this panel renders is draft,
non-binding material. `IssueOpinion.conclusion.text` is expected to have already passed
`packages/guardrail.CheckText`'s verdict-language gate (which wraps
`packages/irac.ContainsVerdictLanguage`) before it ever reaches this panel — but the panel's
own copy (headings, labels, callouts) is independently written to avoid verdict/directive
language, and its test suite re-asserts the rendered output never contains any word from
`packages/irac/guardrail.go`'s real `verdictLanguageWordlist` (`guilty`, `liable`,
`shall pay`, `is ordered`, `is hereby ordered`, `judgment for`, `convicted`, `acquitted`,
`sentenced`), rather than trusting the upstream guarantee blindly.

## Data Shape

There is no `/api/v1/cases/:caseId/opinion` endpoint yet — the same situation
`TreeVisualizationPanel` documents for `/tree` in Phase 065. The types in `src/types/index.ts`
model what that endpoint is expected to serve once built: a UI-side aggregate that joins
`synthesisagent.Opinion`/`TentativeConclusion`, `firstpartyagent`/`secondpartyagent.Argument`,
`evidenceweighing.FactWeight`, and `uncertainty.Uncertainty` by their shared `IssueNodeID` —
conceptually the same join `packages/reasoningtrace` assembles server-side for its own
auditable trace, just not yet exposed over HTTP.

```ts
type OpinionPartyRole = 'first_party' | 'second_party';

interface OpinionArgument {           // firstpartyagent.Argument / secondpartyagent.Argument
  id: string;
  issueNodeId: string;
  partyId: OpinionPartyRole;
  claim: string;
  supportingFactIds: string[];
  supportingRuleIds: string[];
  citations?: OpinionCitation[];
  counterarguments?: string[];
  strength: number;                   // [0, 1]
  grounded: boolean;
}

interface OpinionEvidenceWeight {     // evidenceweighing.FactWeight
  factNodeId: string;
  weight: number;                     // [0, 1]
  kind: string;
  contradicted: boolean;
  corroborationCount: number;
  rationale: string;
}

type OpinionUncertaintySource =       // uncertainty.Source
  | 'issue_framing' | 'evidence' | 'law_application' | 'conclusion';

interface OpinionUncertainty {        // uncertainty.Uncertainty
  issueNodeId: string;
  source: OpinionUncertaintySource;
  severity: number;                   // [0, 1]
  impactRank: number;
  impactScore: number;
  caveat: string;
  detail?: string;
}

interface OpinionConclusion {         // synthesisagent.TentativeConclusion
  issueNodeId: string;
  text: string;
  favoredParty?: OpinionPartyRole;
  confidence: number;                 // [0, 1]
  weakestLink?: string;
  supportingFactIds: string[];
  supportingRuleIds: string[];
  grounded: boolean;
}

interface IssueOpinion {              // UI-side per-issue aggregate
  issueNodeId: string;
  issueText: string;
  firstPartyArguments: OpinionArgument[];
  secondPartyArguments: OpinionArgument[];
  evidenceWeights: OpinionEvidenceWeight[];
  conclusion: OpinionConclusion;
  uncertainties: OpinionUncertainty[];
}

interface CaseOpinion {               // synthesisagent.Opinion's case-level envelope
  caseId: string;
  issues: IssueOpinion[];
  generatedAt: string;
}
```

`OpinionComment` is a separate, purely client-side type (see "Judge Comments" below) — it is
not part of `CaseOpinion` because comments are not yet backed by any API.

## Panel States

`ReasoningOpinionPanel` renders one of four states in its `Card` body, gated in this order:

1. **Loading** (`loading` prop true) — spinner-free placeholder, "Synthesizing draft
   analysis…". `data-testid="reasoning-opinion-placeholder"`.
2. **Draft known, content not yet loaded** (`hasDraftOpinion` true, no `opinion` supplied) —
   an intermediate state kept for callers that only know a draft exists but have not fetched
   its full content. Same placeholder test ID as above.
3. **Empty** (neither `loading`, `hasDraftOpinion`, nor a non-empty `opinion`) — "No draft
   opinion yet" with descriptive copy. Same placeholder test ID.
4. **Full opinion** (`opinion` supplied with at least one issue) — the complete per-issue
   review UI described below, `data-testid="reasoning-opinion-content"`.

The `Disclaimer` component renders unconditionally above the `Card` in every state — it is
never gated behind `opinion` being present, since Phase 057's guardrail requires it visible
regardless of whether there is draft content to show yet.

## Per-Issue Section

Each `IssueOpinion` renders as one `<section data-testid="opinion-issue-<issueNodeId>">`,
top to bottom:

1. **Issue framing** — the issue text as a heading.
2. **Uncertainty callouts** (`data-testid="opinion-uncertainty-<issueNodeId>"`) — only
   rendered when `uncertainties` is non-empty for that issue; each callout is a visually
   distinct red `role="alert"` box showing the uncertainty's `source`, `severity`, and
   `caveat`. Absent entirely for issues with no flagged uncertainty, not shown as an empty
   "no uncertainty" placeholder.
3. **Both parties' arguments, side by side** — a two-column flex layout
   (`opinion-first-party-args-<issueNodeId>` / `opinion-second-party-args-<issueNodeId>`),
   stacking on narrow viewports. Each argument card shows its claim, strength percentage, an
   "Ungrounded reference stripped" badge when `grounded` is false, anticipated
   counterarguments, and (when `onViewTrace` is supplied) a "View supporting nodes" trace
   link. An empty column still renders with an explicit "No arguments recorded for this
   party" message rather than disappearing, so the two-column layout stays stable across
   issues where one side did not argue a point.
4. **Evidence weights inline** (`data-testid="opinion-evidence-weights-<issueNodeId>"`) —
   only rendered when the issue has evidence weights; each row shows the weight's rationale,
   weight percentage, corroboration count, and a "Contradicted" badge when applicable.
5. **Tentative draft conclusion** (`data-testid="opinion-conclusion-<issueNodeId>"`) — the
   conclusion text, confidence percentage, which party it currently favors (if any), a
   weakest-link callout when `weakestLink` is set, and (when `onViewTrace` is supplied and
   the conclusion has at least one supporting node) a "View full trace" action.
6. **Judge comments** (`data-testid="opinion-comments-<issueNodeId>"`) — see below.

## Traceability: "View Full Trace" / "View Supporting Nodes"

Every conclusion and every argument with at least one `supportingFactIds`/`supportingRuleIds`
entry gets a trace-link action, matching Phase 065's `TreeVisualizationPanel`/
`TreeNodeDetail` pattern rather than duplicating tree rendering inside this panel. The panel
itself does not know how to navigate — it calls the optional `onViewTrace(issueNodeId,
nodeId)` prop with the first supporting node ID, and the case workspace page
(`src/app/cases/[caseId]/page.tsx`) supplies the navigation:

```tsx
const handleViewTrace = (_issueNodeId: string, nodeId: string) => {
  setTraceNodeId(nodeId);
  setActiveTab('tree');
};
// ...
<ReasoningOpinionPanel onViewTrace={handleViewTrace} />
// ...
<TreeVisualizationPanel caseId={caseId} initialSelectedNodeId={traceNodeId} />
```

`TreeVisualizationPanel` gained a new optional `initialSelectedNodeId` prop (Phase 067) for
this: it seeds `selectedNodeId`'s initial state and re-applies on every change via a
dedicated effect, so clicking a second trace link while already on the Reasoning Tree tab
re-selects the new node rather than being a no-op. The tree panel itself remains unaware of
opinions — it only ever deals in node IDs, keeping the two panels decoupled per the existing
`docs/tree-visualization.md` design.

When a conclusion or argument is selected as the trace target, the reasoning tree's existing
conclusion-to-evidence path highlighting (Phase 065's `ancestorPath`) takes over from there.

## Judge Comments

`OpinionComment` (`src/types/index.ts`) is a client-side-only annotation: `issueNodeId`,
`text`, `author`, `occurredAt`. There is no annotation API yet, so — following the same
"UI built ahead of the backend" approach `EvidenceAuditEntry` took in Phase 066 — comments
live in the panel's own `useState`, are not persisted across a remount, and `author` is
hardcoded to `"Current Reviewer"` pending real session-derived attribution. Each issue
section has its own comment list and a `<textarea>` + "Add" button
(`aria-label="Add a comment for <issue text>"`); the Add button is disabled while the draft
is empty or whitespace-only, and the textarea clears after a successful add.

## Export Controls

Mirroring `TreeVisualizationPanel`'s Phase 065 export approach exactly (`src/lib/
treeExport.ts`'s `triggerDownload` pattern), `src/lib/opinionExport.ts` provides:

- `exportOpinionAsMarkdown` — `draft-opinion-<caseId>.md`, `text/markdown`
- `exportOpinionAsText` — `draft-opinion-<caseId>.txt`, `text/plain` (same content with
  Markdown emphasis syntax stripped)
- `exportOpinionAsJSON` — `draft-opinion-<caseId>.json`, `application/json` (the full
  `CaseOpinion` plus the disclaimer text and current in-state comments)

All three render the disclaimer text first, then every issue's framing, both parties'
arguments, evidence weights, uncertainty callouts, the tentative conclusion (including its
weakest link), and any judge comments for that issue — so an exported opinion is a complete,
self-contained artifact even without the app around it. Export buttons only appear once a
full `opinion` is loaded (state 4 above), matching `TreeVisualizationPanel`'s convention of
hiding export actions until there is something real to export.

## Directory Structure

```
apps/web/src/
├── app/cases/[caseId]/
│   └── page.tsx                          # Wires onViewTrace <-> initialSelectedNodeId
├── components/workspace/
│   ├── ReasoningOpinionPanel.tsx         # This phase: full per-issue review UI
│   └── TreeVisualizationPanel.tsx        # Gained initialSelectedNodeId (Phase 067)
├── lib/
│   └── opinionExport.ts                  # Markdown/text/JSON export helpers
└── types/index.ts                        # CaseOpinion, IssueOpinion, OpinionArgument, etc.
```

## Testing

`__tests__/ReasoningOpinionPanel.test.tsx` covers:

- The disclaimer always renders, and renders first (before the panel heading) in document
  order — not just present anywhere on the page.
- Loading, draft-known-only, and empty states (preserved from Phase 064).
- Per-issue sections render the issue text and its tentative conclusion.
- Both parties' arguments render side by side, including the explicit "none recorded"
  message for a party with no arguments on an issue.
- Evidence weights render inline with weight, corroboration count, and contradiction badge.
- Uncertainty callouts appear only for issues with at least one flagged uncertainty.
- The weakest-link callout appears only when `weakestLink` is set on the conclusion.
- `onViewTrace` fires with `(issueNodeId, nodeId)` from both a conclusion's "View full
  trace" action and an argument's "View supporting nodes" action; trace-link buttons are
  absent entirely when `onViewTrace` is not supplied.
- Adding a judge comment updates that issue's comment list with author/timestamp, clears the
  input, and does not leak into other issues' comment lists; empty/whitespace-only drafts
  cannot be submitted.
- Export buttons produce the expected Markdown/text/JSON content (asserted against the
  actual `Blob` content passed to the download trigger, not just that a download fired).
- A real assertion that the fully-rendered opinion content never contains any word from
  `packages/irac/guardrail.go`'s actual `verdictLanguageWordlist`.

`__tests__/opinionExport.test.ts` tests the export helpers directly (content generation and
download-trigger file naming), mirroring `treeExport.test.ts`'s style.

`__tests__/TreeVisualizationPanel.test.tsx` gained two cases for `initialSelectedNodeId`:
selecting on initial load, and re-selecting when the prop changes on an already-mounted
panel.

Run: `npm test` from `apps/web/`.
