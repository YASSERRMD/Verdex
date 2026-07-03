# IRAC Reasoning Tree Visualization

## Overview

`TreeVisualizationPanel` (mounted on the Case Workspace's "Reasoning Tree" tab, see
`docs/case-workspace-ux.md`) renders a case's IRAC reasoning tree as an interactive
hierarchical graph: Issue → Rule/Fact → Application → Conclusion, colored by node type,
with per-node detail, collapse/expand, a depth-limit control, conclusion-to-evidence path
highlighting, confidence indicators, and export to SVG or JSON.

This phase (065) fills in the placeholder/loading-only panel built in Phase 064. It does not
change the empty/loading/error state contract that panel already had — those states are
preserved exactly, including the `data-testid="tree-visualization-placeholder"` mount point
Phase 064's and `CaseWorkspacePage`'s tests already assert on.

As with every other reasoning/opinion surface in the app, the tree is draft, non-binding
material — a `ConclusionNode`'s `label` field always reads `draft_analysis` (see
`packages/irac`'s guardrail) and is surfaced verbatim in the node detail panel rather than
presented as a finding or verdict.

## Data Shape

The tree types in `src/types/index.ts` mirror `packages/irac`'s Go schema field-for-field
(camelCased), not knowledgeapi's slimmer wire `NodeDTO` — because this panel needs to
surface source spans, provenance, and jurisdiction tags that `NodeDTO` does not carry yet:

```ts
type TreeNodeType = 'issue' | 'rule' | 'fact' | 'application' | 'conclusion'; // packages/irac/node.go NodeType
type TreeEdgeType = 'governs' | 'applies_to' | 'supports' | 'concludes_from'; // packages/irac/edge.go EdgeType

interface TreeNode {
  id: string;
  type: TreeNodeType;
  caseId: string;
  text: string;
  confidence: number;        // [0, 1]
  createdAt: string;
  spans?: TreeSourceSpan[];  // packages/irac/span.go SourceSpan (rune offsets, OCR page, STT ms)
  provenance?: TreeNodeProvenance;
  jurisdictionCode?: string; // RuleNode only
  legalFamily?: string;      // RuleNode only
  label?: string;            // ConclusionNode only — always "draft_analysis"
}

interface TreeEdge {
  fromId: string;
  toId: string;
  type: TreeEdgeType;
}

interface ReasoningTree {
  caseId: string;
  nodes: TreeNode[];
  edges: TreeEdge[];
}
```

A single flat `TreeNode` shape covers every IRAC node type (rather than a discriminated
union of `IssueNode`/`RuleNode`/etc.) so the panel can render a heterogeneous node list
without a switch at every call site; type-specific fields are simply optional and only
populated for the node types that carry them, exactly as `packages/irac.NodeLike` treats
its concrete wrapper types uniformly on the Go side.

### Edge directions are not uniformly parent → child

This is the single most important thing to know before touching `src/lib/treeLayout.ts`.
`packages/irac`'s legal edge triples (`packages/irac/edge.go`) are:

- `Rule --governs--> Issue`
- `Application --applies_to--> Fact`
- `Application --applies_to--> Rule`
- `Fact --supports--> Application`
- `Conclusion --concludes_from--> Application`

Reading left to right, the edge *source* is sometimes the earlier-rank node (Rule, in
`governs`) and sometimes the later-rank node (Fact, in `supports`, points forward toward the
Application it supports). There is no single "parent points to child" or "child points to
parent" convention to rely on — algorithms that walk edges directionally (ancestor tracing,
collapse/expand) have to account for this explicitly rather than assuming one direction.

## Data Fetching

```
GET /api/v1/cases/:caseId/tree  ->  ReasoningTree
```

This endpoint does not exist on the backend yet — `packages/knowledgeapi` and
`packages/treeindex` have the read logic (`KnowledgeAPI.GetTree`, `Indexer.LookupPaths`) but
no HTTP handler is wired up, matching every other panel in this workspace (see
`case-workspace-ux.md`'s Data Fetching section for the same situation on the case endpoint).
`TreeVisualizationPanel` handles this gracefully:

- **No `caseId` prop** — no fetch is attempted at all; the panel renders its empty state.
  This keeps the component usable standalone (e.g. in isolation, or before a route context
  exists) without requiring a session/router.
- **404** — treated the same as "no tree yet" (empty state), not as an error, since a case
  that has not had reasoning generated for it yet is an expected, normal condition.
- **Any other failure** (network error, 5xx, etc.) — shown as a genuine error state
  (`data-testid="tree-visualization-error"`) with a Retry button.
- **A response with missing/empty `nodes`** — also renders the empty state.

## Layout Algorithm

`src/lib/treeLayout.ts`'s `computeTreeLayout` is a small, dependency-free, pure function
(no DOM, no React) that produces a deterministic left-to-right column layout:

- Every node is placed into one of four fixed columns by `NODE_TYPE_DEPTH`: Issue (0),
  Rule/Fact (1, sharing a column), Application (2), Conclusion (3) — a rank fixed by the
  IRAC schema itself, not derived from graph distance from some root. This means the layout
  is well-defined even for a tree with multiple issues, orphan facts, or any other shape a
  real extraction pipeline might produce, and it never needs a "root node" to be identified.
- Within a column, nodes are ordered by ID for a stable, reproducible layout across renders
  (no simulated force layout, no layout jitter).
- Edges are drawn as cubic Bezier curves between each node's right/left edge midpoints.

`apps/web` has no charting or graph-layout library installed (checked `package.json` before
starting this phase) and none was added: a general force-directed graph library would be
overkill for what is always exactly a 4-column hierarchy, and hand-rolled SVG keeps the
panel dependency-free, fully unit-testable (`__tests__/treeLayout.test.ts` exercises the
layout math directly, with no component mounted), and easy to export losslessly as an SVG
file (see Export, below).

## Color Legend

Defined once in `src/lib/treeLayout.ts` (`NODE_TYPE_COLORS`/`NODE_TYPE_LABELS`) and reused
by both the SVG canvas and the panel's on-screen legend, so the mapping cannot drift:

| Node type | Color | Hex |
|---|---|---|
| Issue | Blue | `#3b82f6` |
| Rule | Orange | `#f97316` |
| Fact | Amber | `#eab308` |
| Application | Purple | `#a855f7` |
| Conclusion | Green | `#22c55e` |

Colors were chosen for contrast against both the light detail panel and the tree canvas
background, and so each pair remains distinguishable under common color-vision deficiencies
(deuteranopia/protanopia) — lightness differs across the set, not only hue.

## Node Detail

Clicking a node (or pressing Enter/Space while it is focused — nodes are keyboard
focusable, `role="button"`, `tabIndex={0}`) opens `TreeNodeDetail`, showing:

- The node's type badge and full `text` (not truncated, unlike the canvas label).
- Every `SourceSpan` the node carries — rune offset range, plus page number (OCR origin) or
  timestamp range (STT origin) when present — this is the node's citation back to the
  ingested source document.
- A confidence percentage and an intensity bar (`confidenceTier`: <50% low/red, 50–80%
  medium/amber, ≥80% high/green).
- `RuleNode`-only jurisdiction code and legal family tag.
- `ConclusionNode`-only guardrail label (`draft_analysis`).
- Provenance (`generatedBy`, and a count of upstream node IDs it was derived from), when
  present.

Clicking the same node again deselects it and closes the panel.

## Collapse / Expand

Any node that has at least one connected node of strictly higher rank can be collapsed (a
small `+`/`−` toggle rendered on its top-right corner). Because edge direction is not
uniformly parent → child (see above), collapse/expand does **not** walk edges directionally;
`descendantIdsByRank` in `treeLayout.ts` instead: (1) finds every node connected to the
clicked node in either edge direction (the tree is a single connected DAG per reasoning
line, so this cannot cross into an unrelated branch), then (2) keeps only the ones whose
column rank is strictly greater. This always hides exactly the set of nodes that would
visually sit to the right of the collapsed node — collapsing an Issue hides its Rules and
everything downstream of them; collapsing a Rule or Fact hides the Applications (and their
Conclusions) it feeds; collapsing an Application hides only its Conclusion(s).

## Depth Control

A `Select` above the canvas limits the maximum visible column rank (Issues only / through
Rules & Facts / through Applications / full tree). Nodes beyond the selected depth are
filtered out before layout, and any edge with an endpoint that got filtered out is dropped
too — this is a simpler, blunter tool than collapse/expand, useful for very large trees
where you want to see the top-level issue structure without expanding/collapsing each
branch individually.

## Conclusion-to-Evidence Path Highlighting

Selecting a `ConclusionNode` computes `ancestorPath` from it and highlights every connected
node/edge in red; everything else dims to 30% opacity. `ancestorPath` walks the union of
both edge directions from the selected node (again, because `supports` points the "wrong"
way relative to `concludes_from`/`applies_to`) — this is safe specifically because the tree
is a connected DAG with a single reasoning direction per branch, so there is only one path
between any two connected nodes; the union walk cannot accidentally pull in a sibling issue's
unrelated Rule/Fact/Application chain. Selecting any non-conclusion node does not trigger
highlighting.

## Confidence Indicators

Two visual cues combine per node, both driven by `packages/irac.Node.Confidence` in `[0,
1]`:

- **On the canvas**: fill opacity scales with `confidenceTier` (low ≈ 55%, medium ≈ 80%,
  high = 100%), and the confidence percentage is printed directly on the node.
- **In the detail panel**: a colored horizontal bar (red/amber/emerald) sized to the exact
  percentage, plus the numeric value.

## Export

Two export actions in the panel header (visible once a tree is loaded):

- **Export SVG** (`exportTreeAsSVG` in `src/lib/treeExport.ts`) — serializes the live,
  currently-rendered `<svg>` element via `XMLSerializer`, so whatever collapse/expand,
  depth-limit, and highlight state is on screen is baked into the exported file exactly as
  seen. Downloaded as `reasoning-tree-<caseId>.svg`.
- **Export JSON** (`exportTreeAsJSON`) — downloads the full, unfiltered `ReasoningTree`
  (every node/edge, regardless of current collapse/depth-limit state) as pretty-printed
  JSON, for programmatic reuse. Downloaded as `reasoning-tree-<caseId>.json`.

Both go through a shared blob-URL download helper (`triggerDownload`). No PNG rasterization
is offered — SVG already satisfies "export as an image", is lossless, and avoids pulling in
a canvas-rasterization dependency for marginal benefit.

## Directory Structure

```
apps/web/src/
├── components/workspace/
│   ├── TreeVisualizationPanel.tsx  # Fetch/state orchestration; loading/empty/error/content
│   ├── TreeCanvas.tsx              # SVG hierarchical graph renderer
│   └── TreeNodeDetail.tsx          # Selected-node detail side panel
├── lib/
│   ├── treeLayout.ts               # Pure layout + graph algorithms + color/label legend
│   └── treeExport.ts               # SVG/JSON export (blob download)
└── types/index.ts                  # TreeNode, TreeEdge, TreeSourceSpan, ReasoningTree, …
```

## Testing

- `__tests__/treeLayout.test.ts` — layout column/position math, `ancestorPath`,
  `descendantIds`/`descendantIdsByRank`, confidence formatting/tiering, and that the color
  legend covers every node type. No component mounted — pure function tests.
- `__tests__/treeExport.test.ts` — both export functions create a blob URL, trigger an
  anchor click, and revoke the URL; the JSON export's filename includes the case ID. jsdom
  does not implement `URL.createObjectURL`, so tests stub it directly on the global.
- `__tests__/TreeNodeDetail.test.tsx` — text/type-badge rendering, guardrail label, rule
  jurisdiction/legal-family tags, confidence percentage, per-span rendering (including page
  number), the no-spans fallback, and the close callback.
- `__tests__/TreeVisualizationPanel.test.tsx` — the full integration surface: preserved
  loading/empty/error states (including 404-as-empty), the `/api/v1/cases/:caseId/tree`
  fetch call, per-type node rendering, the legend, node selection opening/closing the detail
  panel, collapse/expand hiding and restoring a subtree, the depth control filtering nodes,
  path highlighting (and its absence for non-conclusion selections), and both export buttons
  actually triggering a download.

Run: `npm test` from `apps/web/`.
