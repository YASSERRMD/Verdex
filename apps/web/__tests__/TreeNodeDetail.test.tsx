/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TreeNodeDetail } from '@/components/workspace/TreeNodeDetail';
import type { TreeNode } from '@/types';

const CONCLUSION_NODE: TreeNode = {
  id: 'conclusion-1',
  type: 'conclusion',
  caseId: 'case-1',
  text: 'The claim is likely supported by the available evidence.',
  confidence: 0.87,
  createdAt: '2026-01-01T00:00:00Z',
  label: 'draft_analysis',
  spans: [{ start: 10, end: 42, page: 2 }],
  provenance: { generatedBy: 'irac-reasoner-v1', generatedAt: '2026-01-01T00:00:00Z' },
};

const RULE_NODE: TreeNode = {
  id: 'rule-1',
  type: 'rule',
  caseId: 'case-1',
  text: 'A contract requires offer, acceptance, and consideration.',
  confidence: 0.42,
  createdAt: '2026-01-01T00:00:00Z',
  jurisdictionCode: 'US-NY',
  legalFamily: 'common_law',
};

describe('TreeNodeDetail', () => {
  it('renders the node text and type badge', () => {
    render(<TreeNodeDetail node={CONCLUSION_NODE} onClose={jest.fn()} />);
    expect(screen.getByText(CONCLUSION_NODE.text)).toBeInTheDocument();
    expect(screen.getByText('Conclusion')).toBeInTheDocument();
  });

  it('shows the guardrail label for a conclusion node', () => {
    render(<TreeNodeDetail node={CONCLUSION_NODE} onClose={jest.fn()} />);
    expect(screen.getByText(/guardrail label: draft_analysis/i)).toBeInTheDocument();
  });

  it('shows jurisdiction and legal family for a rule node', () => {
    render(<TreeNodeDetail node={RULE_NODE} onClose={jest.fn()} />);
    expect(screen.getByText(/jurisdiction: us-ny/i)).toBeInTheDocument();
    expect(screen.getByText(/legal family: common_law/i)).toBeInTheDocument();
  });

  it('shows the confidence percentage', () => {
    render(<TreeNodeDetail node={CONCLUSION_NODE} onClose={jest.fn()} />);
    expect(screen.getByTestId('tree-node-detail-confidence')).toHaveTextContent('87%');
  });

  it('shows source span and page for each span', () => {
    render(<TreeNodeDetail node={CONCLUSION_NODE} onClose={jest.fn()} />);
    expect(screen.getByTestId('tree-node-detail-spans')).toHaveTextContent('Offset [10–42]');
    expect(screen.getByTestId('tree-node-detail-spans')).toHaveTextContent('Page 2');
  });

  it('shows a fallback message when no spans are recorded', () => {
    render(<TreeNodeDetail node={RULE_NODE} onClose={jest.fn()} />);
    expect(screen.getByText(/no source span recorded/i)).toBeInTheDocument();
  });

  it('calls onClose when the close button is clicked', async () => {
    const onClose = jest.fn();
    render(<TreeNodeDetail node={CONCLUSION_NODE} onClose={onClose} />);
    await userEvent.click(screen.getByRole('button', { name: /close node detail/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
