/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { TreeVisualizationPanel } from '@/components/workspace/TreeVisualizationPanel';

describe('TreeVisualizationPanel', () => {
  it('renders an empty state by default', () => {
    render(<TreeVisualizationPanel />);
    expect(screen.getByText(/no reasoning tree yet/i)).toBeInTheDocument();
  });

  it('renders a loading state when loading is true', () => {
    render(<TreeVisualizationPanel loading />);
    expect(screen.getByText(/generating the reasoning tree/i)).toBeInTheDocument();
    expect(screen.queryByText(/no reasoning tree yet/i)).not.toBeInTheDocument();
  });

  it('provides a mount point for the future tree visualization', () => {
    render(<TreeVisualizationPanel />);
    expect(screen.getByTestId('tree-visualization-placeholder')).toBeInTheDocument();
  });
});
