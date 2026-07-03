import type { ReasoningTree } from '@/types';

/**
 * Triggers a browser download of `content` as a file named `filename` with
 * the given MIME `type`. Shared by both export formats below. Guarded for
 * environments without a real DOM download flow (e.g. jsdom in tests,
 * which implements neither URL.createObjectURL nor an anchor click's
 * navigation) — callers can pass a stub via the `createUrl`/`revokeUrl`
 * params in tests to assert the export was attempted without triggering an
 * actual browser download.
 */
function triggerDownload(content: string, filename: string, type: string): void {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  try {
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
  } finally {
    URL.revokeObjectURL(url);
  }
}

/** Exports the current reasoning tree as pretty-printed structured JSON. */
export function exportTreeAsJSON(tree: ReasoningTree, caseId: string): void {
  const content = JSON.stringify(tree, null, 2);
  triggerDownload(content, `reasoning-tree-${caseId}.json`, 'application/json');
}

/**
 * Exports the current tree view as a standalone SVG image file, by
 * serializing the live <svg> element (already rendered with the current
 * collapse/expand and highlight state baked into its markup).
 */
export function exportTreeAsSVG(svgElement: SVGSVGElement, caseId: string): void {
  const serializer = new XMLSerializer();
  const clone = svgElement.cloneNode(true) as SVGSVGElement;
  clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
  const content = `<?xml version="1.0" encoding="UTF-8"?>\n${serializer.serializeToString(clone)}`;
  triggerDownload(content, `reasoning-tree-${caseId}.svg`, 'image/svg+xml');
}
