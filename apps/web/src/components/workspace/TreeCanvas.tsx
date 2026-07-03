import { forwardRef } from 'react';
import clsx from 'clsx';
import type { TreeEdge, TreeNode } from '@/types';
import {
  NODE_TYPE_COLORS,
  NODE_TYPE_LABELS,
  computeTreeLayout,
  confidenceTier,
  formatConfidence,
  pathEdgeKey,
} from '@/lib/treeLayout';

export interface TreeCanvasProps {
  nodes: TreeNode[];
  edges: TreeEdge[];
  selectedNodeId: string | null;
  highlightedNodeIds: Set<string>;
  highlightedEdgeKeys: Set<string>;
  onSelectNode: (nodeId: string) => void;
  collapsedNodeIds: Set<string>;
  onToggleCollapse: (nodeId: string) => void;
  collapsibleNodeIds: Set<string>;
  className?: string;
}

const NODE_WIDTH = 176;
const NODE_HEIGHT = 64;

/**
 * Renders the IRAC reasoning tree as a hand-rolled SVG hierarchical graph
 * (no charting/graph library dependency — apps/web has none installed, and
 * this is a fixed 5-column layout rather than a general force-directed
 * graph, so plain SVG is the simplest correct tool). Columns run left to
 * right: Issue -> Rule/Fact -> Application -> Conclusion. Wrapped in
 * forwardRef so the panel can grab the root <svg> element for export.
 */
export const TreeCanvas = forwardRef<SVGSVGElement, TreeCanvasProps>(function TreeCanvas(
  {
    nodes,
    edges,
    selectedNodeId,
    highlightedNodeIds,
    highlightedEdgeKeys,
    onSelectNode,
    collapsedNodeIds,
    onToggleCollapse,
    collapsibleNodeIds,
    className,
  },
  ref,
) {
  const layout = computeTreeLayout(nodes, edges);
  const isHighlightActive = highlightedNodeIds.size > 0;

  return (
    <svg
      ref={ref}
      data-testid="tree-canvas"
      viewBox={`0 0 ${layout.width} ${layout.height}`}
      width={layout.width}
      height={layout.height}
      role="img"
      aria-label="IRAC reasoning tree diagram"
      className={clsx('max-w-full', className)}
    >
      <g data-testid="tree-canvas-edges">
        {layout.edges.map((layoutEdge) => {
          const key = pathEdgeKey(layoutEdge.edge);
          const isHighlighted = highlightedEdgeKeys.has(key);
          const fromX = layoutEdge.from.x + NODE_WIDTH;
          const fromY = layoutEdge.from.y + NODE_HEIGHT / 2;
          const toX = layoutEdge.to.x;
          const toY = layoutEdge.to.y + NODE_HEIGHT / 2;
          const midX = (fromX + toX) / 2;

          return (
            <path
              key={key}
              data-testid={`tree-edge-${layoutEdge.edge.fromId}-${layoutEdge.edge.toId}`}
              d={`M ${fromX} ${fromY} C ${midX} ${fromY}, ${midX} ${toY}, ${toX} ${toY}`}
              fill="none"
              stroke={isHighlighted ? '#ef4444' : '#cbd5e1'}
              strokeWidth={isHighlighted ? 3 : 1.5}
              opacity={isHighlightActive && !isHighlighted ? 0.25 : 1}
            />
          );
        })}
      </g>

      <g data-testid="tree-canvas-nodes">
        {layout.nodes.map((layoutNode) => {
          const { node } = layoutNode;
          const colors = NODE_TYPE_COLORS[node.type];
          const tier = confidenceTier(node.confidence);
          const isSelected = node.id === selectedNodeId;
          const isHighlighted = highlightedNodeIds.has(node.id);
          const isDimmed = isHighlightActive && !isHighlighted;
          const isCollapsible = collapsibleNodeIds.has(node.id);
          const isCollapsed = collapsedNodeIds.has(node.id);
          // Confidence drives fill opacity intensity: low confidence renders
          // visually lighter/more muted, high confidence fuller.
          const fillOpacity = tier === 'low' ? 0.55 : tier === 'medium' ? 0.8 : 1;

          return (
            <g
              key={node.id}
              data-testid={`tree-node-${node.id}`}
              data-node-type={node.type}
              data-selected={isSelected}
              data-collapsed={isCollapsed}
              transform={`translate(${layoutNode.x}, ${layoutNode.y})`}
              opacity={isDimmed ? 0.3 : 1}
              className="cursor-pointer"
              role="button"
              tabIndex={0}
              aria-label={`${NODE_TYPE_LABELS[node.type]} node: ${node.text}`}
              onClick={() => onSelectNode(node.id)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelectNode(node.id);
                }
              }}
            >
              <rect
                width={NODE_WIDTH}
                height={NODE_HEIGHT}
                rx={10}
                fill={colors.fill}
                fillOpacity={fillOpacity}
                stroke={isSelected ? '#0f172a' : colors.border}
                strokeWidth={isSelected ? 3 : 1.5}
              />
              <text
                x={10}
                y={22}
                fill={colors.text}
                fontSize={11}
                fontWeight={600}
                aria-hidden="true"
              >
                {NODE_TYPE_LABELS[node.type]}
              </text>
              <text
                x={10}
                y={40}
                fill={colors.text}
                fontSize={10}
                aria-hidden="true"
              >
                {node.text.length > 28 ? `${node.text.slice(0, 28)}…` : node.text}
              </text>
              <text
                x={10}
                y={56}
                fill={colors.text}
                fontSize={9}
                fontWeight={600}
                data-testid={`tree-node-confidence-${node.id}`}
                aria-hidden="true"
              >
                {formatConfidence(node.confidence)} confidence
              </text>

              {isCollapsible && (
                <g
                  data-testid={`tree-node-collapse-toggle-${node.id}`}
                  role="button"
                  aria-label={isCollapsed ? `Expand ${node.text}` : `Collapse ${node.text}`}
                  transform={`translate(${NODE_WIDTH - 20}, 4)`}
                  onClick={(event) => {
                    event.stopPropagation();
                    onToggleCollapse(node.id);
                  }}
                >
                  <circle cx={8} cy={8} r={9} fill="#ffffff" fillOpacity={0.9} />
                  <text x={8} y={12} fontSize={12} textAnchor="middle" fill="#0f172a">
                    {isCollapsed ? '+' : '−'}
                  </text>
                </g>
              )}
            </g>
          );
        })}
      </g>
    </svg>
  );
});
