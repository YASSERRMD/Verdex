/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { EvidenceTimelinePanel } from '@/components/workspace/EvidenceTimelinePanel';
import type { EvidenceSegment, TimelineEvent } from '@/types';

const SEGMENTS: EvidenceSegment[] = [
  {
    id: 'seg-1',
    text: 'Witness testimony describing the incident.',
    type: 'testimony',
    party: 'first_party',
    confidence: 0.82,
    sourceFileName: 'deposition.pdf',
    sourceSpan: { start: 10, end: 120 },
  },
];

const EVENTS: TimelineEvent[] = [
  { id: 'evt-2', description: 'Contract signed', occurredAt: '2025-06-01' },
  { id: 'evt-1', description: 'Dispute arose', occurredAt: '2025-01-15' },
  { id: 'evt-3', description: 'Undated filing note' },
];

describe('EvidenceTimelinePanel', () => {
  it('renders evidence segments with type and source span', () => {
    render(<EvidenceTimelinePanel segments={SEGMENTS} events={[]} />);
    expect(screen.getByText(/witness testimony describing the incident/i)).toBeInTheDocument();
    expect(screen.getByText('Testimony')).toBeInTheDocument();
    expect(screen.getByText(/source span \[10–120\]/i)).toBeInTheDocument();
    expect(screen.getByText(/82%/)).toBeInTheDocument();
  });

  it('renders timeline events sorted chronologically, undated last', () => {
    render(<EvidenceTimelinePanel segments={[]} events={EVENTS} />);
    const items = screen.getAllByText(/signed|arose|undated filing note/i);
    expect(items[0]).toHaveTextContent('Dispute arose');
    expect(items[1]).toHaveTextContent('Contract signed');
    expect(items[2]).toHaveTextContent('Undated filing note');
  });

  it('shows empty-state messages when there is no evidence or timeline data', () => {
    render(<EvidenceTimelinePanel segments={[]} events={[]} />);
    expect(screen.getByText(/no evidence segments recorded for this case yet/i)).toBeInTheDocument();
    expect(screen.getByText(/no timeline events recorded for this case yet/i)).toBeInTheDocument();
  });
});
