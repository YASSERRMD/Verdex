/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { SearchResultsList } from '@/components/search/SearchResultsList';
import type { SearchResultItem } from '@/types';

const RESULT: SearchResultItem = {
  caseId: 'case-1',
  title: 'Doe v. Acme Corp',
  reference: '2026-CV-001',
  categoryId: 'civil',
  jurisdictionId: 'jur-1',
  state: 'active',
  createdAt: '2026-01-01T00:00:00Z',
  mode: 'keyword',
  score: 0.87,
  snippet: 'The tenant **breached** the lease agreement.',
  hits: [{ nodeId: 'n1', nodeType: 'fact', text: 'breached', score: 0.9, explanation: 'keyword match' }],
};

describe('SearchResultsList', () => {
  it('shows a prompt before any search has run', () => {
    render(<SearchResultsList results={[]} hasSearched={false} />);
    expect(screen.getByText(/enter a search term/i)).toBeInTheDocument();
  });

  it('shows a loading state while searching', () => {
    render(<SearchResultsList results={[]} loading hasSearched />);
    expect(screen.getByRole('status')).toHaveTextContent(/searching/i);
  });

  it('shows an empty state when a search finds nothing', () => {
    render(<SearchResultsList results={[]} hasSearched />);
    expect(screen.getByText(/no cases matched/i)).toBeInTheDocument();
  });

  it('renders each result with title, category, state, and snippet', () => {
    render(<SearchResultsList results={[RESULT]} hasSearched />);
    expect(screen.getByText('Doe v. Acme Corp')).toBeInTheDocument();
    expect(screen.getByText('2026-CV-001')).toBeInTheDocument();
    expect(screen.getByText('civil')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('87%')).toBeInTheDocument();
  });

  it('highlights the matched portion of the snippet as a <mark>', () => {
    render(<SearchResultsList results={[RESULT]} hasSearched />);
    const mark = screen.getByText('breached');
    expect(mark.tagName).toBe('MARK');
  });

  it('links each result to its case workspace', () => {
    render(<SearchResultsList results={[RESULT]} hasSearched />);
    const link = screen.getByRole('link', { name: /Doe v\. Acme Corp/i });
    expect(link).toHaveAttribute('href', '/cases/case-1');
  });

  it('surfaces a skipped-case notice when skippedCases is non-zero', () => {
    render(<SearchResultsList results={[RESULT]} hasSearched skippedCases={2} />);
    expect(screen.getByText(/2 cases could not be searched/i)).toBeInTheDocument();
  });
});
