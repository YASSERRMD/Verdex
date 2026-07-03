'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import clsx from 'clsx';
import { FileJsonIcon, GitBranchIcon, ImageIcon } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { Button } from '@/components/ui/Button';
import { Select } from '@/components/ui/Select';
import { apiFetch, ApiError } from '@/lib/api';
import { exportTreeAsJSON, exportTreeAsSVG } from '@/lib/treeExport';
import {
  ancestorPath,
  descendantIdsByRank,
  nodeDepth,
  pathEdgeKey,
  NODE_TYPE_COLORS,
  NODE_TYPE_LABELS,
} from '@/lib/treeLayout';
import { TreeCanvas } from './TreeCanvas';
import { TreeNodeDetail } from './TreeNodeDetail';
import type { ReasoningTree, TreeNodeType } from '@/types';

export interface TreeVisualizationPanelProps {
  /** True while the reasoning tree is being generated or fetched. */
  loading?: boolean;
  /**
   * Case ID to fetch the reasoning tree for. When omitted, no fetch is
   * attempted and the panel renders its empty state — this keeps the
   * component usable standalone (e.g. before a case has a tree yet, or in
   * isolation in tests) without requiring a route/session context.
   */
  caseId?: string;
  className?: string;
}

const NODE_TYPE_ORDER: TreeNodeType[] = ['issue', 'rule', 'fact', 'application', 'conclusion'];
const DEPTH_OPTIONS = [
  { value: '0', label: 'Issues only' },
  { value: '1', label: 'Through Rules & Facts' },
  { value: '2', label: 'Through Applications' },
  { value: '3', label: 'Full tree' },
];

/**
 * Interactive IRAC reasoning tree view: fetches the case's tree, renders it
 * as a hierarchical SVG graph colored by node type, and lets the user
 * select a node for full detail (text, source span, citation, confidence),
 * collapse/expand subtrees, limit visible depth, highlight the evidence
 * path behind a conclusion, and export the current view as SVG or JSON.
 *
 * There is no `/api/v1/cases/:caseId/tree` endpoint wired up on the backend
 * yet (packages/knowledgeapi and packages/treeindex have the read logic,
 * but no HTTP handler) — a fetch failure or empty response is treated the
 * same as "no tree yet", not as an error, except for genuine network/server
 * errors (surfaced via the error state below).
 */
export function TreeVisualizationPanel({
  loading: loadingProp = false,
  caseId,
  className,
}: TreeVisualizationPanelProps) {
  const [tree, setTree] = useState<ReasoningTree | null>(null);
  const [fetching, setFetching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [collapsedNodeIds, setCollapsedNodeIds] = useState<Set<string>>(new Set());
  const [maxDepth, setMaxDepth] = useState(3);
  const svgRef = useRef<SVGSVGElement | null>(null);

  const loadTree = useCallback(async () => {
    if (!caseId) return;
    setFetching(true);
    setError(null);
    try {
      const result = await apiFetch<ReasoningTree>(`/api/v1/cases/${caseId}/tree`);
      setTree(result ?? null);
    } catch (err) {
      // A tree that has not been generated (or the endpoint not yet
      // existing) is not an error condition — treat 404 as "no tree yet"
      // rather than surfacing an alert.
      if (err instanceof ApiError && err.status === 404) {
        setTree(null);
      } else {
        setError(err instanceof Error ? err.message : 'Failed to load the reasoning tree.');
      }
    } finally {
      setFetching(false);
    }
  }, [caseId]);

  useEffect(() => {
    loadTree();
  }, [loadTree]);

  const nodes = useMemo(() => tree?.nodes ?? [], [tree]);
  const edges = useMemo(() => tree?.edges ?? [], [tree]);

  const hiddenByDepth = useMemo(() => {
    const hidden = new Set<string>();
    nodes.forEach((node) => {
      if (nodeDepth(node.type) > maxDepth) hidden.add(node.id);
    });
    return hidden;
  }, [nodes, maxDepth]);

  const hiddenByCollapse = useMemo(() => {
    const hidden = new Set<string>();
    collapsedNodeIds.forEach((collapsedId) => {
      descendantIdsByRank(collapsedId, nodes, edges).forEach((id) => hidden.add(id));
    });
    return hidden;
  }, [collapsedNodeIds, nodes, edges]);

  const visibleNodes = useMemo(
    () => nodes.filter((node) => !hiddenByDepth.has(node.id) && !hiddenByCollapse.has(node.id)),
    [nodes, hiddenByDepth, hiddenByCollapse],
  );
  const visibleNodeIds = useMemo(() => new Set(visibleNodes.map((n) => n.id)), [visibleNodes]);
  const visibleEdges = useMemo(
    () => edges.filter((edge) => visibleNodeIds.has(edge.fromId) && visibleNodeIds.has(edge.toId)),
    [edges, visibleNodeIds],
  );

  const collapsibleNodeIds = useMemo(() => {
    const collapsible = new Set<string>();
    nodes.forEach((node) => {
      if (descendantIdsByRank(node.id, nodes, edges).size > 0) collapsible.add(node.id);
    });
    return collapsible;
  }, [nodes, edges]);

  const selectedNode = useMemo(
    () => nodes.find((n) => n.id === selectedNodeId) ?? null,
    [nodes, selectedNodeId],
  );

  const highlightedNodeIds = useMemo(() => {
    if (!selectedNode || selectedNode.type !== 'conclusion') return new Set<string>();
    return ancestorPath(selectedNode.id, edges);
  }, [selectedNode, edges]);

  const highlightedEdgeKeys = useMemo(() => {
    if (highlightedNodeIds.size === 0) return new Set<string>();
    const keys = new Set<string>();
    edges.forEach((edge) => {
      if (highlightedNodeIds.has(edge.fromId) && highlightedNodeIds.has(edge.toId)) {
        keys.add(pathEdgeKey(edge));
      }
    });
    return keys;
  }, [edges, highlightedNodeIds]);

  const handleSelectNode = useCallback((nodeId: string) => {
    setSelectedNodeId((current) => (current === nodeId ? null : nodeId));
  }, []);

  const handleToggleCollapse = useCallback((nodeId: string) => {
    setCollapsedNodeIds((current) => {
      const next = new Set(current);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });
  }, []);

  const handleExportJSON = useCallback(() => {
    if (!tree) return;
    exportTreeAsJSON(tree, tree.caseId || caseId || 'case');
  }, [tree, caseId]);

  const handleExportSVG = useCallback(() => {
    if (!svgRef.current) return;
    exportTreeAsSVG(svgRef.current, tree?.caseId || caseId || 'case');
  }, [tree, caseId]);

  const isLoading = loadingProp || fetching;
  const hasTree = !isLoading && !error && nodes.length > 0;
  const isEmpty = !isLoading && !error && nodes.length === 0;

  return (
    <Card
      className={clsx(className)}
      header={
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-base font-semibold text-neutral-800 dark:text-white">
            Reasoning Tree
          </h2>
          {hasTree && (
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={handleExportSVG}
                leftIcon={<ImageIcon className="h-3.5 w-3.5" />}
              >
                Export SVG
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleExportJSON}
                leftIcon={<FileJsonIcon className="h-3.5 w-3.5" />}
              >
                Export JSON
              </Button>
            </div>
          )}
        </div>
      }
    >
      {isLoading && (
        <div
          data-testid="tree-visualization-placeholder"
          className="flex flex-col items-center justify-center gap-3 py-16 text-center"
        >
          <Spinner size="lg" className="text-primary-DEFAULT" />
          <p className="text-sm text-neutral-500">Generating the reasoning tree…</p>
        </div>
      )}

      {!isLoading && error && (
        <div
          data-testid="tree-visualization-error"
          role="alert"
          className="flex flex-col items-center justify-center gap-3 py-16 text-center"
        >
          <GitBranchIcon className="h-10 w-10 text-red-300" aria-hidden="true" />
          <p className="text-sm font-medium text-red-600">{error}</p>
          {caseId && (
            <Button variant="secondary" size="sm" onClick={loadTree}>
              Retry
            </Button>
          )}
        </div>
      )}

      {isEmpty && (
        <div
          data-testid="tree-visualization-placeholder"
          className="flex flex-col items-center justify-center gap-3 py-16 text-center"
        >
          <GitBranchIcon className="h-10 w-10 text-neutral-300" aria-hidden="true" />
          <p className="text-sm font-medium text-neutral-600 dark:text-neutral-300">
            No reasoning tree yet
          </p>
          <p className="max-w-sm text-xs text-neutral-400">
            The interactive issue/rule/fact/conclusion tree view will render here once
            reasoning has been generated for this case.
          </p>
        </div>
      )}

      {hasTree && (
        <div className="flex flex-col gap-4" data-testid="tree-visualization-content">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <ul className="flex flex-wrap items-center gap-3" aria-label="Node type legend">
              {NODE_TYPE_ORDER.map((type) => (
                <li
                  key={type}
                  className="flex items-center gap-1.5 text-xs text-neutral-600 dark:text-neutral-300"
                >
                  <span
                    className="h-3 w-3 rounded-full"
                    style={{ backgroundColor: NODE_TYPE_COLORS[type].fill }}
                    aria-hidden="true"
                  />
                  {NODE_TYPE_LABELS[type]}
                </li>
              ))}
            </ul>

            <div className="w-56">
              <Select
                aria-label="Depth limit"
                value={String(maxDepth)}
                onChange={(event) => setMaxDepth(Number(event.target.value))}
                options={DEPTH_OPTIONS}
              />
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
            <div className="overflow-auto rounded-lg border border-neutral-100 bg-neutral-50 p-2 dark:border-neutral-700 dark:bg-neutral-900 lg:col-span-2">
              <TreeCanvas
                ref={svgRef}
                nodes={visibleNodes}
                edges={visibleEdges}
                selectedNodeId={selectedNodeId}
                highlightedNodeIds={highlightedNodeIds}
                highlightedEdgeKeys={highlightedEdgeKeys}
                onSelectNode={handleSelectNode}
                collapsedNodeIds={collapsedNodeIds}
                onToggleCollapse={handleToggleCollapse}
                collapsibleNodeIds={collapsibleNodeIds}
              />
            </div>

            <div>
              {selectedNode ? (
                <TreeNodeDetail node={selectedNode} onClose={() => setSelectedNodeId(null)} />
              ) : (
                <p className="rounded-lg border border-dashed border-neutral-200 p-4 text-xs text-neutral-400 dark:border-neutral-700">
                  Select a node to see its full text, source span, citation, and confidence.
                  Selecting a conclusion highlights the supporting facts and rules behind it.
                </p>
              )}
            </div>
          </div>
        </div>
      )}
    </Card>
  );
}
