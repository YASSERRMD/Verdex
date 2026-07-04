# User guide for judges and advocates

This guide walks a judicial practitioner (judge, advocate, or case
reviewer) through the Verdex case workspace, end to end: opening a case,
reviewing evidence, reading the draft reasoning tree and opinion, and
signing off. It is a practitioner-facing companion to the UI design
docs under [`apps/web/docs/`](../../apps/web/docs) — see those for full
implementation detail; this guide only walks through what a practitioner
actually does, screen by screen.

> **Every output described below is draft analysis only.** Nothing in
> the case workspace is a finding, ruling, or verdict. The disclaimer
> banner is always the first thing rendered on any reasoning surface,
> and no output is usable until a qualified human practitioner reviews
> and signs off on it (see "Step 5 — Sign off" below). This is enforced
> in code by [`packages/guardrail`](../../packages/guardrail) (Phase
> 057) and [`packages/signoff`](../../packages/signoff) (Phase 068), not
> a UI convention that can be dismissed.

## Step 1 — Open a new case

Phase 030's ingestion wizard at `/cases/new` — see
[`apps/web/docs/ingestion-ux.md`](../../apps/web/docs/ingestion-ux.md) —
walks you through four steps:

1. **Case** — enter the case category and the first/second party names.
2. **Upload** — drag and drop, or pick, source documents and audio
   recordings. Once uploaded, each file is hashed for provenance and
   the binary is discarded after extraction — only the extracted text
   persists (the "transcribe-and-discard" guarantee; see
   [`packages/provenance`](../../packages/provenance)).
3. **Processing** — a live status panel shows the pipeline working
   through transcription/OCR, normalization, and segmentation.
4. **Review** — review the extracted text segments, correct any
   evidence classification, and edit the party/timeline model before
   the case is created.

## Step 2 — The case workspace

Once a case exists, it lives at `/cases/[caseId]` — see
[`apps/web/docs/case-workspace-ux.md`](../../apps/web/docs/case-workspace-ux.md)
(Phase 064). The workspace header shows the case title, reference,
lifecycle state (`draft` → `active` → `under_review` → `closed` →
`archived`), category, and jurisdiction, followed by a status/actions
bar showing whichever transitions are legal from the current state
(e.g. reopening a `closed` case requires an explicit, non-blank
justification).

Seven tabs organize the rest of the workspace:

| Tab | What you do there |
|---|---|
| Overview | Review parties (role, counsel) and category/subcategory. |
| Evidence & Timeline | Read (not edit) evidence segments and the chronological event timeline. |
| Evidence Review | Correct evidence classifications and party attribution, flag disputes, bulk-tag, and see a per-segment change history — see Step 3 below. |
| Reasoning Tree | Explore the case's IRAC reasoning tree interactively — see Step 4 below. |
| Draft Opinion | Read the full draft reasoned opinion — see Step 4 below. |
| Discussion | Leave threaded case notes for other reviewers, and resolve/reopen discussion threads. |
| History | Browse a chronological version history across case metadata, the reasoning tree, evidence, and the draft opinion, with per-entry diff and (for case metadata) restore. |

## Step 3 — Review evidence

The **Evidence Review** tab
([`apps/web/docs/evidence-review.md`](../../apps/web/docs/evidence-review.md),
Phase 066) is where you correct the system's automated evidence
classification: change a segment's evidence type or party attribution
inline, flag a segment as disputed, bulk-tag multiple segments, and
search/filter the evidence list. Every correction is recorded in a
per-segment audit trail — nothing you change here silently overwrites
what the system originally extracted.

## Step 4 — Read the reasoning tree and draft opinion

The **Reasoning Tree** tab
([`apps/web/docs/tree-visualization.md`](../../apps/web/docs/tree-visualization.md),
Phase 065) renders the case's IRAC tree — Issue → Rule/Fact →
Application → Conclusion — as an interactive graph. You can expand or
collapse nodes, inspect a node's detail in a side panel, limit the
displayed depth, highlight the path from a conclusion back to its
supporting evidence, and export the tree as SVG or JSON.

The **Draft Opinion** tab
([`apps/web/docs/opinion-review.md`](../../apps/web/docs/opinion-review.md),
Phase 067) is the full reviewer surface for the synthesized draft
opinion: one section per issue, both parties' strongest arguments shown
side by side, evidence weights shown inline, uncertainty/weakest-link
callouts wherever the system's confidence is low, a trace-link from
every conclusion back to the reasoning tree node that produced it, a
per-issue comment box for your own notes, and export controls. The
disclaimer banner renders first, always, above everything else on this
tab.

If you need to double-check exactly how a conclusion was reached,
follow its trace-link — this walks the same
[`packages/reasoningtrace`](../../packages/reasoningtrace) (Phase 060)
join the platform itself uses internally to make every conclusion
auditable, not a black box.

## Step 5 — Sign off

No draft opinion is usable, exportable, or citable until it carries an
**approved** sign-off, recorded by
[`packages/signoff`](../../packages/signoff) (Phase 068). This is the
platform's hard, code-enforced gate — there is no way to bypass it from
the UI or the API.

To approve or reject:

- You must hold the sign-off permission (`identity.PermSignOff`,
  already granted to the judge role by default).
- You must type the exact acknowledgement phrase the system requires
  — *"I acknowledge and approve this review decision"* — the UI
  requires you to deliberately confirm this before the request is even
  built; it cannot be satisfied by accident.
- Rejecting requires you to enter non-blank notes explaining why;
  approving notes are optional but recommended.
- The system checks that you are reviewing the *current* version of
  the case (its live metadata version) — if someone else changed the
  case underneath you, your sign-off attempt is rejected rather than
  silently applied to stale content, and you are asked to re-review.

Every sign-off decision — and every automatic re-review trigger — is
permanently recorded in an append-only audit trail alongside the case's
own version history (visible on the **History** tab).

## Cross-case search

Need to find a related case? `/search` — see
[`apps/web/docs/case-search-ux.md`](../../apps/web/docs/case-search-ux.md)
(Phase 069) — offers a query box, structured filters (category, party
name, state, date range), a ranked results list, and the ability to save
and re-run named searches. Search results never leak facts across
cases: retrieval is always case-scoped
(see [`packages/knowledgeisolation`](../../packages/knowledgeisolation),
Phase 047).

## Getting help

- If something in the workspace looks wrong or you are unsure how a
  conclusion was reached, use the trace-link in Step 4 before assuming
  it is an error.
- For deployment or account-provisioning questions, see
  [`docs/admin/setup-guide.md`](../admin/setup-guide.md) or your
  deployment's administrator.
- For the full documentation index, see [`docs/README.md`](../README.md).
