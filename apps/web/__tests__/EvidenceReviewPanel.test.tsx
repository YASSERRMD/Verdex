/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EvidenceReviewPanel } from '@/components/workspace/EvidenceReviewPanel';
import type { EvidenceSegment } from '@/types';

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

import { apiFetch } from '@/lib/api';
const mockApiFetch = apiFetch as jest.MockedFunction<typeof apiFetch>;

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
  {
    id: 'seg-2',
    text: 'Exhibit A, the signed contract.',
    type: 'documentary',
    party: 'second_party',
    confidence: 0.91,
    sourceFileName: 'contract.pdf',
    sourceSpan: { start: 0, end: 40 },
    disputed: true,
  },
  {
    id: 'seg-3',
    text: 'Section 302 of the applicable code.',
    type: 'statute_citation',
    party: 'unattributed',
    confidence: 0.65,
    sourceSpan: { start: 5, end: 55 },
  },
];

describe('EvidenceReviewPanel', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockApiFetch.mockResolvedValue({});
  });

  it('shows an empty-state message when there are no segments', () => {
    render(<EvidenceReviewPanel segments={[]} />);
    expect(screen.getByText(/no evidence segments recorded for this case yet/i)).toBeInTheDocument();
  });

  it('renders the segment list with correct type badges', () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);
    expect(screen.getByTestId('evidence-review-segment-seg-1')).toBeInTheDocument();
    expect(screen.getByTestId('evidence-review-segment-seg-2')).toBeInTheDocument();
    expect(screen.getByTestId('evidence-review-segment-seg-3')).toBeInTheDocument();

    expect(screen.getByTestId('evidence-type-badge-seg-1')).toHaveTextContent('Testimony');
    expect(screen.getByTestId('evidence-type-badge-seg-2')).toHaveTextContent('Documentary');
    expect(screen.getByTestId('evidence-type-badge-seg-3')).toHaveTextContent('Statute Citation');
  });

  it('summarizes the total and disputed segment counts', () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);
    expect(screen.getByTestId('evidence-summary')).toHaveTextContent('3 segments total');
    expect(screen.getByTestId('evidence-summary')).toHaveTextContent('1 disputed');
  });

  it('re-syncs its list when the segments prop changes after mount', () => {
    const { rerender } = render(<EvidenceReviewPanel segments={SEGMENTS.slice(0, 1)} />);
    expect(screen.getByTestId('evidence-review-segment-seg-1')).toBeInTheDocument();
    expect(screen.queryByTestId('evidence-review-segment-seg-2')).not.toBeInTheDocument();

    rerender(<EvidenceReviewPanel segments={SEGMENTS} />);
    expect(screen.getByTestId('evidence-review-segment-seg-2')).toBeInTheDocument();
    expect(screen.getByTestId('evidence-review-segment-seg-3')).toBeInTheDocument();
  });

  it('shows provenance and confidence for each segment', () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);
    const seg1 = screen.getByTestId('evidence-review-segment-seg-1');
    expect(within(seg1).getByText(/deposition\.pdf/i)).toBeInTheDocument();
    expect(within(seg1).getByText(/source span \[10–120\]/i)).toBeInTheDocument();
    expect(within(seg1).getByText(/82%/)).toBeInTheDocument();
  });

  it('applies an inline classification correction and updates state', async () => {
    const onSegmentsChange = jest.fn();
    render(<EvidenceReviewPanel segments={SEGMENTS} onSegmentsChange={onSegmentsChange} />);

    const seg1 = screen.getByTestId('evidence-review-segment-seg-1');
    const typeSelect = within(seg1).getByLabelText(/evidence type/i);
    await userEvent.selectOptions(typeSelect, 'argument');

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/api/v1/segments/seg-1/classification',
        expect.objectContaining({ method: 'PUT' }),
      );
    });

    await waitFor(() => {
      expect(onSegmentsChange).toHaveBeenCalledWith(
        expect.arrayContaining([expect.objectContaining({ id: 'seg-1', type: 'argument' })]),
      );
    });
  });

  it('reassigns a segment to a different party', async () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);
    const seg3 = screen.getByTestId('evidence-review-segment-seg-3');
    const partySelect = within(seg3).getByLabelText(/^party$/i);
    await userEvent.selectOptions(partySelect, 'first_party');

    await waitFor(() => {
      expect(within(seg3).getByText(/attributed to first party/i)).toBeInTheDocument();
    });
  });

  it('toggles a segment between disputed and undisputed', async () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);
    const seg1 = screen.getByTestId('evidence-review-segment-seg-1');
    expect(within(seg1).getByRole('button', { name: /undisputed/i })).toBeInTheDocument();

    await userEvent.click(within(seg1).getByRole('button', { name: /undisputed/i }));

    await waitFor(() => {
      expect(within(seg1).getByRole('button', { name: /^disputed$/i })).toBeInTheDocument();
    });
  });

  it('applies bulk tagging to all selected segments', async () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);

    await userEvent.click(screen.getByLabelText(/select segment seg-1/i));
    await userEvent.click(screen.getByLabelText(/select segment seg-3/i));

    const bulkBar = screen.getByTestId('bulk-tagging-bar');
    const bulkPartySelect = within(bulkBar).getByLabelText(/bulk set party/i);
    await userEvent.selectOptions(bulkPartySelect, 'second_party');

    await userEvent.click(within(bulkBar).getByRole('button', { name: /apply to 2 selected/i }));

    await waitFor(() => {
      const seg1 = screen.getByTestId('evidence-review-segment-seg-1');
      const seg3 = screen.getByTestId('evidence-review-segment-seg-3');
      expect(within(seg1).getByText(/attributed to second party/i)).toBeInTheDocument();
      expect(within(seg3).getByText(/attributed to second party/i)).toBeInTheDocument();
    });

    // seg-2 was not selected, so it keeps its original party.
    const seg2 = screen.getByTestId('evidence-review-segment-seg-2');
    expect(within(seg2).getByText(/attributed to second party/i)).toBeInTheDocument();
  });

  it('narrows the list with type, party, dispute, and free-text filters', async () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);
    const filters = screen.getByTestId('evidence-filters');

    await userEvent.selectOptions(within(filters).getByLabelText(/^type$/i), 'documentary');
    expect(screen.queryByTestId('evidence-review-segment-seg-1')).not.toBeInTheDocument();
    expect(screen.getByTestId('evidence-review-segment-seg-2')).toBeInTheDocument();
    expect(screen.queryByTestId('evidence-review-segment-seg-3')).not.toBeInTheDocument();

    await userEvent.selectOptions(within(filters).getByLabelText(/^type$/i), '');
    await userEvent.selectOptions(within(filters).getByLabelText(/dispute status/i), 'disputed');
    expect(screen.getByTestId('evidence-review-segment-seg-2')).toBeInTheDocument();
    expect(screen.queryByTestId('evidence-review-segment-seg-1')).not.toBeInTheDocument();

    await userEvent.selectOptions(within(filters).getByLabelText(/dispute status/i), '');
    await userEvent.selectOptions(within(filters).getByLabelText(/^party$/i), 'unattributed');
    expect(screen.getByTestId('evidence-review-segment-seg-3')).toBeInTheDocument();
    expect(screen.queryByTestId('evidence-review-segment-seg-1')).not.toBeInTheDocument();

    await userEvent.selectOptions(within(filters).getByLabelText(/^party$/i), '');
    await userEvent.type(within(filters).getByLabelText(/search evidence segments/i), 'contract');
    expect(screen.getByTestId('evidence-review-segment-seg-2')).toBeInTheDocument();
    expect(screen.queryByTestId('evidence-review-segment-seg-1')).not.toBeInTheDocument();
    expect(screen.queryByTestId('evidence-review-segment-seg-3')).not.toBeInTheDocument();
  });

  it('clears all active filters and restores the full list', async () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);
    const filters = screen.getByTestId('evidence-filters');

    await userEvent.selectOptions(within(filters).getByLabelText(/^type$/i), 'documentary');
    expect(screen.getByText(/showing 1 of 3 segments/i)).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /clear filters/i }));

    expect(screen.getByTestId('evidence-review-segment-seg-1')).toBeInTheDocument();
    expect(screen.getByTestId('evidence-review-segment-seg-2')).toBeInTheDocument();
    expect(screen.getByTestId('evidence-review-segment-seg-3')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /clear filters/i })).not.toBeInTheDocument();
  });

  it('displays audit history after a correction is made', async () => {
    render(<EvidenceReviewPanel segments={SEGMENTS} />);
    const seg1 = screen.getByTestId('evidence-review-segment-seg-1');

    await userEvent.selectOptions(within(seg1).getByLabelText(/evidence type/i), 'other');

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalled();
    });

    await userEvent.click(within(seg1).getByRole('button', { name: /show change history/i }));

    const history = within(seg1).getByTestId('audit-history-seg-1');
    const entries = within(history).getAllByText((_, element) =>
      /changed type from .testimony. to .other./i.test(element?.textContent ?? ''),
    );
    expect(entries.length).toBeGreaterThan(0);
  });

  it('shows an inline error when a correction fails to save', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Network error'));
    render(<EvidenceReviewPanel segments={SEGMENTS} />);

    const seg1 = screen.getByTestId('evidence-review-segment-seg-1');
    await userEvent.selectOptions(within(seg1).getByLabelText(/evidence type/i), 'other');

    await waitFor(() => {
      expect(within(seg1).getByRole('alert')).toHaveTextContent(/network error/i);
    });
  });
});
