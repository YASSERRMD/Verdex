import type { TreeEdge, TreeNode, TreeNodeType } from '@/types';

/**
 * Fixed left-to-right rank per node type, matching the IRAC reasoning
 * order this tree always flows in: Issue -> Rule/Fact -> Application ->
 * Conclusion (packages/irac/edge.go's legal edge triples: Rule--governs-->
 * Issue, Fact--supports-->Application, Application--applies_to-->Rule/Fact,
 * Conclusion--concludes_from-->Application). Rule and Fact share a rank
 * since both feed directly into an Application.
 */
const NODE_TYPE_DEPTH: Record<TreeNodeType, number> = {
  issue: 0,
  rule: 1,
  fact: 1,
  application: 2,
  conclusion: 3,
};

/**
 * Documented color legend for the reasoning tree, keyed by node type. Used
 * consistently for the node fill in the SVG graph and the legend swatches.
 * Colors chosen for contrast against both the light node-detail panel and
 * the tree canvas background, and to stay distinguishable for common forms
 * of color-vision deficiency (deuteranopia/protanopia): blue vs. orange vs.
 * amber vs. purple vs. green each differ enough in lightness, not only hue.
 */
export const NODE_TYPE_COLORS: Record<TreeNodeType, { fill: string; border: string; text: string }> = {
  issue: { fill: '#3b82f6', border: '#1d4ed8', text: '#ffffff' }, // blue
  rule: { fill: '#f97316', border: '#c2410c', text: '#ffffff' }, // orange
  fact: { fill: '#eab308', border: '#a16207', text: '#1c1917' }, // amber
  application: { fill: '#a855f7', border: '#7e22ce', text: '#ffffff' }, // purple
  conclusion: { fill: '#22c55e', border: '#15803d', text: '#ffffff' }, // green
};

/** Human-readable label per node type, used in the legend and detail panel. */
export const NODE_TYPE_LABELS: Record<TreeNodeType, string> = {
  issue: 'Issue',
  rule: 'Rule',
  fact: 'Fact',
  application: 'Application',
  conclusion: 'Conclusion',
};

/** Human-readable label per edge type, used in the detail panel. */
export const EDGE_TYPE_LABELS: Record<TreeEdge['type'], string> = {
  governs: 'governs',
  applies_to: 'applies to',
  supports: 'supports',
  concludes_from: 'concludes from',
};

export interface LayoutNode {
  node: TreeNode;
  depth: number;
  /** Position within its depth column, top to bottom, 0-based. */
  order: number;
  x: number;
  y: number;
}

export interface LayoutEdge {
  edge: TreeEdge;
  from: LayoutNode;
  to: LayoutNode;
}

export interface TreeLayout {
  nodes: LayoutNode[];
  edges: LayoutEdge[];
  width: number;
  height: number;
}

const COLUMN_WIDTH = 220;
const ROW_HEIGHT = 96;
const MARGIN_X = 110;
const MARGIN_Y = 56;

/**
 * Computes a deterministic left-to-right hierarchical layout for a set of
 * IRAC tree nodes/edges. Nodes are grouped into columns by their fixed
 * NODE_TYPE_DEPTH rank (not by graph distance from a root), so the visual
 * layout always reads Issue -> Rule/Fact -> Application -> Conclusion
 * regardless of how the underlying edges happen to be ordered. Within a
 * column, nodes are ordered by ID for a stable, reproducible layout across
 * renders.
 *
 * Pure function with no DOM/React dependency, so it can be unit tested and
 * reused (e.g. for SVG export) without mounting a component.
 */
export function computeTreeLayout(nodes: TreeNode[], edges: TreeEdge[]): TreeLayout {
  const columns = new Map<number, TreeNode[]>();
  for (const node of nodes) {
    const depth = NODE_TYPE_DEPTH[node.type] ?? 0;
    const bucket = columns.get(depth);
    if (bucket) {
      bucket.push(node);
    } else {
      columns.set(depth, [node]);
    }
  }

  const layoutNodes: LayoutNode[] = [];
  const byId = new Map<string, LayoutNode>();
  const maxDepth = Math.max(0, ...Array.from(columns.keys()));
  let maxRows = 1;

  Array.from(columns.entries()).forEach(([depth, bucket]) => {
    const sorted = [...bucket].sort((a, b) => a.id.localeCompare(b.id));
    maxRows = Math.max(maxRows, sorted.length);
    sorted.forEach((node, order) => {
      const layoutNode: LayoutNode = {
        node,
        depth,
        order,
        x: MARGIN_X + depth * COLUMN_WIDTH,
        y: MARGIN_Y + order * ROW_HEIGHT,
      };
      layoutNodes.push(layoutNode);
      byId.set(node.id, layoutNode);
    });
  });

  const layoutEdges: LayoutEdge[] = [];
  edges.forEach((edge) => {
    const from = byId.get(edge.fromId);
    const to = byId.get(edge.toId);
    if (from && to) {
      layoutEdges.push({ edge, from, to });
    }
  });

  return {
    nodes: layoutNodes,
    edges: layoutEdges,
    width: MARGIN_X * 2 + maxDepth * COLUMN_WIDTH,
    height: MARGIN_Y * 2 + Math.max(0, maxRows - 1) * ROW_HEIGHT,
  };
}

/**
 * Returns the depth (rank) of a node type, used to drive the depth-limit
 * control (nodes with depth greater than the selected limit are hidden).
 */
export function nodeDepth(type: TreeNodeType): number {
  return NODE_TYPE_DEPTH[type] ?? 0;
}

/** The maximum depth any node type can occupy (fixed by the IRAC schema). */
export const MAX_TREE_DEPTH = Math.max(...Object.values(NODE_TYPE_DEPTH));

/**
 * Builds an adjacency map from child node ID to the set of node IDs that
 * directly feed into it (i.e. every `from` of an edge whose `to` is that
 * node). Used both for collapse/expand (children of an issue/rule/fact) and
 * for walking backwards from a conclusion to its supporting evidence.
 */
export function buildReverseAdjacency(edges: TreeEdge[]): Map<string, string[]> {
  const map = new Map<string, string[]>();
  for (const edge of edges) {
    const list = map.get(edge.toId);
    if (list) {
      list.push(edge.fromId);
    } else {
      map.set(edge.toId, [edge.fromId]);
    }
  }
  return map;
}

/**
 * Builds a forward adjacency map from parent node ID to the set of node IDs
 * it directly points to. Used for collapse/expand of descendant subtrees.
 */
export function buildForwardAdjacency(edges: TreeEdge[]): Map<string, string[]> {
  const map = new Map<string, string[]>();
  for (const edge of edges) {
    const list = map.get(edge.fromId);
    if (list) {
      list.push(edge.toId);
    } else {
      map.set(edge.fromId, [edge.toId]);
    }
  }
  return map;
}

/**
 * Returns every node ID on the evidence path leading to `nodeId`, inclusive
 * of `nodeId` itself. The IRAC edge directions are not uniformly "parent ->
 * child": a Conclusion points *to* the Application it concludes from
 * (concludes_from), an Application points *to* the Rule/Fact it applies
 * (applies_to), but a Fact points *to* the Application it supports
 * (supports) — the reverse of the other two (packages/irac/edge.go's
 * legalEdgeTriples). So tracing "conclusion back to its supporting
 * evidence" means following edges in the forward direction for
 * concludes_from/applies_to, and in the reverse direction for supports.
 * Rather than special-case edge types, this walks the union of both
 * adjacencies (forward and reverse) from `nodeId`, which is safe because
 * the IRAC schema's edge triples form a DAG with a single reasoning
 * direction (Issue -> ... -> Conclusion) — there is only one path between
 * any two connected nodes, so the union cannot pull in an unrelated branch.
 */
export function ancestorPath(nodeId: string, edges: TreeEdge[]): Set<string> {
  const forward = buildForwardAdjacency(edges);
  const reverse = buildReverseAdjacency(edges);
  const visited = new Set<string>();
  const stack = [nodeId];
  while (stack.length > 0) {
    const current = stack.pop()!;
    if (visited.has(current)) continue;
    visited.add(current);
    const neighbors = [...(forward.get(current) ?? []), ...(reverse.get(current) ?? [])];
    for (const neighbor of neighbors) {
      if (!visited.has(neighbor)) stack.push(neighbor);
    }
  }
  return visited;
}

/**
 * Returns every edge (by from/to pair) that lies on the ancestor path set,
 * i.e. both endpoints are in `pathNodeIds`. Used to highlight the exact
 * connecting lines between a conclusion and its supporting evidence.
 */
export function pathEdgeKey(edge: TreeEdge): string {
  return `${edge.fromId}->${edge.toId}`;
}

/**
 * Returns every node ID reachable by walking forwards (source -> target)
 * from `nodeId`, exclusive of `nodeId` itself. Used by collapse/expand to
 * determine which nodes belong to a subtree rooted at a collapsed node.
 */
export function descendantIds(nodeId: string, edges: TreeEdge[]): Set<string> {
  const forward = buildForwardAdjacency(edges);
  const visited = new Set<string>();
  const stack = [...(forward.get(nodeId) ?? [])];
  while (stack.length > 0) {
    const current = stack.pop()!;
    if (visited.has(current)) continue;
    visited.add(current);
    for (const child of forward.get(current) ?? []) {
      if (!visited.has(child)) stack.push(child);
    }
  }
  return visited;
}

/** Formats a confidence score in [0, 1] as a percentage string, e.g. "82%". */
export function formatConfidence(confidence: number): string {
  return `${Math.round(confidence * 100)}%`;
}

/**
 * Buckets a confidence score into a qualitative tier used to drive visual
 * intensity (opacity) of the confidence indicator: low scores are visually
 * lighter/more muted, high scores fuller/more saturated.
 */
export function confidenceTier(confidence: number): 'low' | 'medium' | 'high' {
  if (confidence < 0.5) return 'low';
  if (confidence < 0.8) return 'medium';
  return 'high';
}
