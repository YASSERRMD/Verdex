/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { DiscardConfirmationBanner } from '@/components/ingestion/DiscardConfirmationBanner';

describe('DiscardConfirmationBanner', () => {
  it('renders with an accessible note role and label', () => {
    render(<DiscardConfirmationBanner />);
    expect(screen.getByRole('note', { name: /source file discard notice/i })).toBeInTheDocument();
  });

  it('explains that files are hashed then discarded after extraction', () => {
    render(<DiscardConfirmationBanner />);
    const note = screen.getByRole('note', { name: /source file discard notice/i });
    expect(note).toHaveTextContent(/hashed/i);
    expect(note).toHaveTextContent(/discarded/i);
    expect(note).toHaveTextContent(/transcribed or extracted/i);
  });

  it('does not claim the extracted content has been reviewed or finalized', () => {
    render(<DiscardConfirmationBanner />);
    const note = screen.getByRole('note', { name: /source file discard notice/i });
    expect(note).toHaveTextContent(/draft material only/i);
    expect(note).not.toHaveTextContent(/verdict|ruling|final decision/i);
  });

  it('applies compact styling when compact is true', () => {
    render(<DiscardConfirmationBanner compact />);
    const note = screen.getByRole('note', { name: /source file discard notice/i });
    expect(note.className).toEqual(expect.stringContaining('px-3'));
  });
});
