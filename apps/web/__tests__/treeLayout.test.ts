import {
  computeTreeLayout,
  nodeDepth,
  ancestorPath,
  descendantIds,
  descendantIdsByRank,
  formatConfidence,
  confidenceTier,
  NODE_TYPE_COLORS,
  NODE_TYPE_LABELS,
} from '@/lib/treeLayout';
import type { TreeEdge, TreeNode } from '@/types';

function node(id: string, type: TreeNode['type'], confidence = 0.8): TreeNode {
  return {
    id,
    type,
    caseId: 'case-1',
    text: `${type} ${id}`,
    confidence,
    createdAt: '2026-01-01T00:00:00Z',
  };
}

const NODES: TreeNode[] = [
  node('issue-1', 'issue'),
  node('rule-1', 'rule'),
  node('fact-1', 'fact'),
  node('application-1', 'application'),
  node('conclusion-1', 'conclusion'),
];

const EDGES: TreeEdge[] = [
  { fromId: 'rule-1', toId: 'issue-1', type: 'governs' },
  { fromId: 'application-1', toId: 'rule-1', type: 'applies_to' },
  { fromId: 'application-1', toId: 'fact-1', type: 'applies_to' },
  { fromId: 'fact-1', toId: 'application-1', type: 'supports' },
  { fromId: 'conclusion-1', toId: 'application-1', type: 'concludes_from' },
];

describe('computeTreeLayout', () => {
  it('places nodes into columns ordered issue -> rule/fact -> application -> conclusion', () => {
    const layout = computeTreeLayout(NODES, EDGES);
    const depthById = new Map(layout.nodes.map((n) => [n.node.id, n.depth]));

    expect(depthById.get('issue-1')).toBe(0);
    expect(depthById.get('rule-1')).toBe(1);
    expect(depthById.get('fact-1')).toBe(1);
    expect(depthById.get('application-1')).toBe(2);
    expect(depthById.get('conclusion-1')).toBe(3);
  });

  it('increases x position with column depth', () => {
    const layout = computeTreeLayout(NODES, EDGES);
    const byId = new Map(layout.nodes.map((n) => [n.node.id, n]));
    expect(byId.get('issue-1')!.x).toBeLessThan(byId.get('rule-1')!.x);
    expect(byId.get('rule-1')!.x).toBeLessThan(byId.get('application-1')!.x);
    expect(byId.get('application-1')!.x).toBeLessThan(byId.get('conclusion-1')!.x);
  });

  it('gives same-depth nodes distinct y positions', () => {
    const layout = computeTreeLayout(NODES, EDGES);
    const byId = new Map(layout.nodes.map((n) => [n.node.id, n]));
    expect(byId.get('rule-1')!.y).not.toBe(byId.get('fact-1')!.y);
  });

  it('only includes edges whose endpoints are both present', () => {
    const edgesWithDangling: TreeEdge[] = [
      ...EDGES,
      { fromId: 'missing', toId: 'issue-1', type: 'governs' },
    ];
    const layout = computeTreeLayout(NODES, edgesWithDangling);
    expect(layout.edges).toHaveLength(EDGES.length);
  });

  it('computes a non-zero width and height for a populated tree', () => {
    const layout = computeTreeLayout(NODES, EDGES);
    expect(layout.width).toBeGreaterThan(0);
    expect(layout.height).toBeGreaterThan(0);
  });

  it('returns an empty layout for an empty tree', () => {
    const layout = computeTreeLayout([], []);
    expect(layout.nodes).toHaveLength(0);
    expect(layout.edges).toHaveLength(0);
  });
});

describe('nodeDepth', () => {
  it('returns the fixed rank for each node type', () => {
    expect(nodeDepth('issue')).toBe(0);
    expect(nodeDepth('rule')).toBe(1);
    expect(nodeDepth('fact')).toBe(1);
    expect(nodeDepth('application')).toBe(2);
    expect(nodeDepth('conclusion')).toBe(3);
  });
});

describe('ancestorPath', () => {
  it('walks backwards from a conclusion through application to its supporting rule and fact', () => {
    const path = ancestorPath('conclusion-1', EDGES);
    expect(path.has('conclusion-1')).toBe(true);
    expect(path.has('application-1')).toBe(true);
    expect(path.has('rule-1')).toBe(true);
    expect(path.has('fact-1')).toBe(true);
    expect(path.has('issue-1')).toBe(true);
  });

  it('returns just the node itself when it has no incoming edges', () => {
    const path = ancestorPath('issue-1', []);
    expect(path).toEqual(new Set(['issue-1']));
  });

  it('does not include unrelated branches', () => {
    const extraEdges: TreeEdge[] = [
      ...EDGES,
      { fromId: 'rule-2', toId: 'issue-2', type: 'governs' },
    ];
    const path = ancestorPath('conclusion-1', extraEdges);
    expect(path.has('rule-2')).toBe(false);
    expect(path.has('issue-2')).toBe(false);
  });
});

describe('descendantIds', () => {
  it('returns every node reachable forward from a node, excluding itself', () => {
    const descendants = descendantIds('conclusion-1', EDGES);
    expect(descendants.has('application-1')).toBe(true);
    expect(descendants.has('rule-1')).toBe(true);
    expect(descendants.has('fact-1')).toBe(true);
    expect(descendants.has('issue-1')).toBe(true);
    expect(descendants.has('conclusion-1')).toBe(false);
  });

  it('returns an empty set for a leaf node', () => {
    expect(descendantIds('issue-1', EDGES)).toEqual(new Set());
  });
});

describe('descendantIdsByRank', () => {
  it('collapsing the issue hides every connected node with a higher rank', () => {
    // fact-1 is transitively connected to issue-1 via application-1, so it
    // is included too — the whole tree is one connected component here.
    const descendants = descendantIdsByRank('issue-1', NODES, EDGES);
    expect(descendants).toEqual(new Set(['rule-1', 'application-1', 'fact-1', 'conclusion-1']));
  });

  it('collapsing a rule hides only application and conclusion, not the issue', () => {
    const descendants = descendantIdsByRank('rule-1', NODES, EDGES);
    expect(descendants.has('issue-1')).toBe(false);
    expect(descendants.has('application-1')).toBe(true);
    expect(descendants.has('conclusion-1')).toBe(true);
  });

  it('collapsing the application hides only the conclusion', () => {
    const descendants = descendantIdsByRank('application-1', NODES, EDGES);
    expect(descendants).toEqual(new Set(['conclusion-1']));
  });

  it('collapsing the conclusion (highest rank) hides nothing', () => {
    expect(descendantIdsByRank('conclusion-1', NODES, EDGES)).toEqual(new Set());
  });

  it('returns an empty set for an unknown node id', () => {
    expect(descendantIdsByRank('missing', NODES, EDGES)).toEqual(new Set());
  });
});

describe('formatConfidence', () => {
  it('formats a fractional confidence as a rounded percentage', () => {
    expect(formatConfidence(0.823)).toBe('82%');
    expect(formatConfidence(1)).toBe('100%');
    expect(formatConfidence(0)).toBe('0%');
  });
});

describe('confidenceTier', () => {
  it('buckets scores into low/medium/high', () => {
    expect(confidenceTier(0.2)).toBe('low');
    expect(confidenceTier(0.6)).toBe('medium');
    expect(confidenceTier(0.95)).toBe('high');
  });
});

describe('NODE_TYPE_COLORS and NODE_TYPE_LABELS', () => {
  it('define an entry for every IRAC node type', () => {
    const types: TreeNode['type'][] = ['issue', 'rule', 'fact', 'application', 'conclusion'];
    for (const type of types) {
      expect(NODE_TYPE_COLORS[type]).toBeDefined();
      expect(NODE_TYPE_LABELS[type]).toBeDefined();
    }
  });
});
