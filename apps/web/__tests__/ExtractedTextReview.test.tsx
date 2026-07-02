/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { ExtractedTextReview } from '@/components/ingestion/ExtractedTextReview';
import type { SegmentReview } from '@/types';

function makeSegments(count: number): SegmentReview[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `seg-${i}`,
    text: `Segment text ${i}`,
    sourceSpan: { start: i * 10, end: i * 10 + 9 },
    sourceFileName: 'testimony.pdf',
  }));
}

describe('ExtractedTextReview', () => {
  it('renders an empty-state message when there are no segments', () => {
    render(<ExtractedTextReview segments={[]} />);
    expect(screen.getByText(/no extracted segments yet/i)).toBeInTheDocument();
  });

  it('renders segment text and source-span reference', () => {
    render(<ExtractedTextReview segments={makeSegments(1)} pageSize={5} />);
    expect(screen.getByText('Segment text 0')).toBeInTheDocument();
    expect(screen.getByText(/source span \[0–9\]/i)).toBeInTheDocument();
    expect(screen.getByText(/testimony.pdf/)).toBeInTheDocument();
  });

  it('paginates when there are more segments than the page size', () => {
    render(<ExtractedTextReview segments={makeSegments(7)} pageSize={5} />);
    expect(screen.getByText('Segment text 0')).toBeInTheDocument();
    expect(screen.getByText('Segment text 4')).toBeInTheDocument();
    expect(screen.queryByText('Segment text 5')).not.toBeInTheDocument();
    expect(screen.getByText(/page 1 of 2/i)).toBeInTheDocument();
  });

  it('navigates to the next page when Next is clicked', () => {
    render(<ExtractedTextReview segments={makeSegments(7)} pageSize={5} />);
    fireEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(screen.getByText('Segment text 5')).toBeInTheDocument();
    expect(screen.queryByText('Segment text 0')).not.toBeInTheDocument();
  });

  it('disables Previous on the first page and Next on the last page', () => {
    render(<ExtractedTextReview segments={makeSegments(7)} pageSize={5} />);
    expect(screen.getByRole('button', { name: /previous/i })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(screen.getByRole('button', { name: /next/i })).toBeDisabled();
  });
});
