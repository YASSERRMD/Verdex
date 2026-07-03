/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { ReasoningOpinionPanel } from '@/components/workspace/ReasoningOpinionPanel';

describe('ReasoningOpinionPanel', () => {
  it('always renders the non-binding disclaimer', () => {
    render(<ReasoningOpinionPanel />);
    expect(screen.getByLabelText(/non-binding disclaimer/i)).toBeInTheDocument();
    expect(screen.getByText(/non-binding draft analysis/i)).toBeInTheDocument();
  });

  it('renders an empty state when there is no draft opinion', () => {
    render(<ReasoningOpinionPanel />);
    expect(screen.getByText(/no draft opinion yet/i)).toBeInTheDocument();
  });

  it('renders a loading state while synthesizing', () => {
    render(<ReasoningOpinionPanel loading />);
    expect(screen.getByText(/synthesizing draft analysis/i)).toBeInTheDocument();
  });

  it('renders a draft-available message once a draft opinion exists', () => {
    render(<ReasoningOpinionPanel hasDraftOpinion />);
    expect(screen.getByText(/a draft opinion is available/i)).toBeInTheDocument();
  });

  it('never uses verdict language', () => {
    render(<ReasoningOpinionPanel hasDraftOpinion />);
    expect(screen.getByTestId('reasoning-opinion-placeholder')).not.toHaveTextContent(
      /verdict|ruling|final decision/i,
    );
  });
});
