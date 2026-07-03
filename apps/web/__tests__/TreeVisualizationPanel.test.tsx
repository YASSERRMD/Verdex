/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TreeVisualizationPanel } from '@/components/workspace/TreeVisualizationPanel';
import type { ReasoningTree } from '@/types';

jest.mock('@/lib/api', () => ({
  apiFetch: jest.fn(),
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
      this.name = 'ApiError';
    }
  },
}));

import { apiFetch, ApiError } from '@/lib/api';

const mockApiFetch = apiFetch as jest.MockedFunction<typeof apiFetch>;

const TREE: ReasoningTree = {
  caseId: 'case-1',
  nodes: [
    {
      id: 'issue-1',
      type: 'issue',
      caseId: 'case-1',
      text: 'Was the contract validly formed?',
      confidence: 0.91,
      createdAt: '2026-01-01T00:00:00Z',
    },
    {
      id: 'rule-1',
      type: 'rule',
      caseId: 'case-1',
      text: 'A contract requires offer, acceptance, and consideration.',
      confidence: 0.88,
      createdAt: '2026-01-01T00:00:00Z',
      jurisdictionCode: 'US-NY',
      legalFamily: 'common_law',
      spans: [{ start: 0, end: 40 }],
    },
    {
      id: 'fact-1',
      type: 'fact',
      caseId: 'case-1',
      text: 'The parties exchanged signed purchase orders.',
      confidence: 0.76,
      createdAt: '2026-01-01T00:00:00Z',
      spans: [{ start: 100, end: 140, page: 3 }],
    },
    {
      id: 'application-1',
      type: 'application',
      caseId: 'case-1',
      text: 'The signed purchase orders satisfy offer and acceptance.',
      confidence: 0.7,
      createdAt: '2026-01-01T00:00:00Z',
    },
    {
      id: 'conclusion-1',
      type: 'conclusion',
      caseId: 'case-1',
      text: 'The contract was likely validly formed.',
      confidence: 0.65,
      createdAt: '2026-01-01T00:00:00Z',
      label: 'draft_analysis',
    },
  ],
  edges: [
    { fromId: 'rule-1', toId: 'issue-1', type: 'governs' },
    { fromId: 'application-1', toId: 'rule-1', type: 'applies_to' },
    { fromId: 'application-1', toId: 'fact-1', type: 'applies_to' },
    { fromId: 'fact-1', toId: 'application-1', type: 'supports' },
    { fromId: 'conclusion-1', toId: 'application-1', type: 'concludes_from' },
  ],
};

describe('TreeVisualizationPanel', () => {
  beforeEach(() => {
    mockApiFetch.mockReset();
  });

  it('renders an empty state by default when no caseId is given', () => {
    render(<TreeVisualizationPanel />);
    expect(screen.getByText(/no reasoning tree yet/i)).toBeInTheDocument();
    expect(mockApiFetch).not.toHaveBeenCalled();
  });

  it('renders a loading state when loading is true', () => {
    render(<TreeVisualizationPanel loading />);
    expect(screen.getByText(/generating the reasoning tree/i)).toBeInTheDocument();
    expect(screen.queryByText(/no reasoning tree yet/i)).not.toBeInTheDocument();
  });

  it('provides a mount point for the empty state', () => {
    render(<TreeVisualizationPanel />);
    expect(screen.getByTestId('tree-visualization-placeholder')).toBeInTheDocument();
  });

  it('fetches the tree for the given caseId and shows a loading state while pending', () => {
    mockApiFetch.mockReturnValueOnce(new Promise(() => {}));
    render(<TreeVisualizationPanel caseId="case-1" />);
    expect(screen.getByText(/generating the reasoning tree/i)).toBeInTheDocument();
    expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/cases/case-1/tree');
  });

  it('treats a 404 tree response as the empty state, not an error', async () => {
    mockApiFetch.mockRejectedValueOnce(new ApiError(404, 'not found'));
    render(<TreeVisualizationPanel caseId="case-1" />);
    await waitFor(() => expect(screen.getByText(/no reasoning tree yet/i)).toBeInTheDocument());
  });

  it('shows an error state on a generic fetch failure', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Network error'));
    render(<TreeVisualizationPanel caseId="case-1" />);
    await waitFor(() => expect(screen.getByTestId('tree-visualization-error')).toBeInTheDocument());
    expect(screen.getByTestId('tree-visualization-error')).toHaveTextContent(/network error/i);
  });

  describe('with a loaded tree', () => {
    async function renderWithTree() {
      mockApiFetch.mockResolvedValueOnce(TREE);
      render(<TreeVisualizationPanel caseId="case-1" />);
      await waitFor(() => expect(screen.getByTestId('tree-visualization-content')).toBeInTheDocument());
    }

    it('renders every node, colored and labeled by type', async () => {
      await renderWithTree();
      const canvas = screen.getByTestId('tree-canvas');
      for (const node of TREE.nodes) {
        const nodeGroup = within(canvas).getByTestId(`tree-node-${node.id}`);
        expect(nodeGroup).toHaveAttribute('data-node-type', node.type);
      }
    });

    it('pre-selects the node given via initialSelectedNodeId once the tree loads (deep-link from the opinion panel)', async () => {
      mockApiFetch.mockResolvedValueOnce(TREE);
      render(<TreeVisualizationPanel caseId="case-1" initialSelectedNodeId="fact-1" />);
      await waitFor(() => expect(screen.getByTestId('tree-visualization-content')).toBeInTheDocument());

      const detail = screen.getByTestId('tree-node-detail');
      expect(within(detail).getByText(TREE.nodes.find((n) => n.id === 'fact-1')!.text)).toBeInTheDocument();
    });

    it('re-selects when initialSelectedNodeId changes on an already-mounted panel', async () => {
      mockApiFetch.mockResolvedValueOnce(TREE);
      const { rerender } = render(
        <TreeVisualizationPanel caseId="case-1" initialSelectedNodeId="fact-1" />,
      );
      await waitFor(() => expect(screen.getByTestId('tree-visualization-content')).toBeInTheDocument());
      expect(
        within(screen.getByTestId('tree-node-detail')).getByText(
          TREE.nodes.find((n) => n.id === 'fact-1')!.text,
        ),
      ).toBeInTheDocument();

      rerender(<TreeVisualizationPanel caseId="case-1" initialSelectedNodeId="rule-1" />);
      expect(
        within(screen.getByTestId('tree-node-detail')).getByText(
          TREE.nodes.find((n) => n.id === 'rule-1')!.text,
        ),
      ).toBeInTheDocument();
    });

    it('renders the node-type legend', async () => {
      await renderWithTree();
      const legend = screen.getByLabelText(/node type legend/i);
      expect(within(legend).getByText('Issue')).toBeInTheDocument();
      expect(within(legend).getByText('Rule')).toBeInTheDocument();
      expect(within(legend).getByText('Fact')).toBeInTheDocument();
      expect(within(legend).getByText('Application')).toBeInTheDocument();
      expect(within(legend).getByText('Conclusion')).toBeInTheDocument();
    });

    it('shows node detail with source span and confidence when a node is selected', async () => {
      await renderWithTree();
      await userEvent.click(screen.getByTestId('tree-node-rule-1'));

      const detail = screen.getByTestId('tree-node-detail');
      expect(within(detail).getByText(TREE.nodes[1].text)).toBeInTheDocument();
      expect(within(detail).getByTestId('tree-node-detail-confidence')).toHaveTextContent('88%');
      expect(within(detail).getByTestId('tree-node-detail-spans')).toHaveTextContent('Offset [0–40]');
    });

    it('deselects the node when clicked again, closing the detail panel', async () => {
      await renderWithTree();
      await userEvent.click(screen.getByTestId('tree-node-rule-1'));
      expect(screen.getByTestId('tree-node-detail')).toBeInTheDocument();

      await userEvent.click(screen.getByTestId('tree-node-rule-1'));
      expect(screen.queryByTestId('tree-node-detail')).not.toBeInTheDocument();
    });

    it('collapses and expands a subtree via the collapse toggle', async () => {
      await renderWithTree();
      // rule-1 feeds application-1 which feeds conclusion-1, so rule-1 is collapsible.
      const collapseToggle = screen.getByTestId('tree-node-collapse-toggle-rule-1');

      expect(screen.getByTestId('tree-node-application-1')).toBeInTheDocument();

      await userEvent.click(collapseToggle);
      expect(screen.queryByTestId('tree-node-application-1')).not.toBeInTheDocument();
      expect(screen.queryByTestId('tree-node-conclusion-1')).not.toBeInTheDocument();
      // fact-1's own path to application-1 also disappears since application-1 is gone.
      expect(screen.getByTestId('tree-node-rule-1')).toBeInTheDocument();

      await userEvent.click(screen.getByTestId('tree-node-collapse-toggle-rule-1'));
      expect(screen.getByTestId('tree-node-application-1')).toBeInTheDocument();
      expect(screen.getByTestId('tree-node-conclusion-1')).toBeInTheDocument();
    });

    it('limits visible nodes using the depth control', async () => {
      await renderWithTree();
      expect(screen.getByTestId('tree-node-conclusion-1')).toBeInTheDocument();

      await userEvent.selectOptions(screen.getByLabelText(/depth limit/i), 'Issues only');

      expect(screen.getByTestId('tree-node-issue-1')).toBeInTheDocument();
      expect(screen.queryByTestId('tree-node-rule-1')).not.toBeInTheDocument();
      expect(screen.queryByTestId('tree-node-conclusion-1')).not.toBeInTheDocument();
    });

    it('highlights the conclusion-to-evidence path when a conclusion is selected', async () => {
      await renderWithTree();
      await userEvent.click(screen.getByTestId('tree-node-conclusion-1'));

      // Non-highlighted nodes are dimmed via opacity; highlighted ones (on
      // the path) are not. issue-1/rule-1/fact-1/application-1/conclusion-1
      // are all on the path here since it's a single connected chain.
      const path = ['conclusion-1', 'application-1', 'rule-1', 'fact-1', 'issue-1'];
      for (const id of path) {
        expect(screen.getByTestId(`tree-node-${id}`)).toHaveAttribute('opacity', '1');
      }
    });

    it('does not highlight a path when a non-conclusion node is selected', async () => {
      await renderWithTree();
      await userEvent.click(screen.getByTestId('tree-node-fact-1'));
      // No dimming should occur for any node.
      for (const node of TREE.nodes) {
        expect(screen.getByTestId(`tree-node-${node.id}`)).toHaveAttribute('opacity', '1');
      }
    });

    it('triggers a JSON export when the export button is clicked', async () => {
      await renderWithTree();
      const createObjectURL = jest.fn(() => 'blob:mock');
      const revokeObjectURL = jest.fn();
      URL.createObjectURL = createObjectURL;
      URL.revokeObjectURL = revokeObjectURL;
      const clickSpy = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

      await userEvent.click(screen.getByRole('button', { name: /export json/i }));

      expect(createObjectURL).toHaveBeenCalledTimes(1);
      expect(clickSpy).toHaveBeenCalledTimes(1);
      clickSpy.mockRestore();
    });

    it('triggers an SVG export when the export button is clicked', async () => {
      await renderWithTree();
      const createObjectURL = jest.fn(() => 'blob:mock');
      const revokeObjectURL = jest.fn();
      URL.createObjectURL = createObjectURL;
      URL.revokeObjectURL = revokeObjectURL;
      const clickSpy = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

      await userEvent.click(screen.getByRole('button', { name: /export svg/i }));

      expect(createObjectURL).toHaveBeenCalledTimes(1);
      expect(clickSpy).toHaveBeenCalledTimes(1);
      clickSpy.mockRestore();
    });
  });
});
