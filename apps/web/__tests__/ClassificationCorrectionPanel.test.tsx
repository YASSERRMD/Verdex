/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ClassificationCorrectionPanel } from '@/components/ingestion/ClassificationCorrectionPanel';
import type { SegmentClassification } from '@/types';

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

const CLASSIFICATIONS: SegmentClassification[] = [
  {
    segmentId: 'seg-1',
    type: 'testimony',
    party: 'first_party',
    confidence: 0.82,
    overridden: false,
  },
];

describe('ClassificationCorrectionPanel', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders an empty-state message when there are no classifications', () => {
    render(<ClassificationCorrectionPanel classifications={[]} />);
    expect(screen.getByText(/no classified segments yet/i)).toBeInTheDocument();
  });

  it('renders evidence type and party controls with confidence', () => {
    render(<ClassificationCorrectionPanel classifications={CLASSIFICATIONS} />);
    expect(screen.getByTestId('classification-seg-1')).toBeInTheDocument();
    expect(screen.getByText(/confidence: 82%/i)).toBeInTheDocument();
  });

  it('calls the stub API and onCorrected when the evidence type is overridden', async () => {
    mockApiFetch.mockResolvedValueOnce({});
    const onCorrected = jest.fn();

    render(
      <ClassificationCorrectionPanel classifications={CLASSIFICATIONS} onCorrected={onCorrected} />,
    );
    const typeSelect = screen.getByLabelText(/evidence type/i);
    await userEvent.selectOptions(typeSelect, 'documentary');

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/api/v1/segments/seg-1/classification',
        expect.objectContaining({ method: 'PUT' }),
      );
      expect(onCorrected).toHaveBeenCalledWith(
        expect.objectContaining({ segmentId: 'seg-1', type: 'documentary', overridden: true }),
      );
    });
  });

  it('shows an inline error when the correction fails to save', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Network error'));

    render(<ClassificationCorrectionPanel classifications={CLASSIFICATIONS} />);
    const partySelect = screen.getByLabelText(/^party$/i);
    await userEvent.selectOptions(partySelect, 'second_party');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/network error/i);
    });
  });
});
