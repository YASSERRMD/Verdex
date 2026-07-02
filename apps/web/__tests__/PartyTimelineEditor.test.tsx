/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PartyTimelineEditor } from '@/components/ingestion/PartyTimelineEditor';

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

describe('PartyTimelineEditor', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders empty-state messaging when there are no parties or events', () => {
    render(
      <PartyTimelineEditor
        parties={[]}
        events={[]}
        onPartiesChange={jest.fn()}
        onEventsChange={jest.fn()}
      />,
    );
    expect(screen.getByText(/no parties added yet/i)).toBeInTheDocument();
    expect(screen.getByText(/no timeline events added yet/i)).toBeInTheDocument();
  });

  it('adds a party via the stub API when Add Party is clicked', async () => {
    mockApiFetch.mockResolvedValueOnce({});
    const onPartiesChange = jest.fn();

    render(
      <PartyTimelineEditor
        parties={[]}
        events={[]}
        onPartiesChange={onPartiesChange}
        onEventsChange={jest.fn()}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /add party/i }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/parties', expect.objectContaining({ method: 'POST' }));
      expect(onPartiesChange).toHaveBeenCalledWith([
        expect.objectContaining({ role: 'first_party', name: '' }),
      ]);
    });
  });

  it('adds a timeline event via the stub API when Add Event is clicked', async () => {
    mockApiFetch.mockResolvedValueOnce({});
    const onEventsChange = jest.fn();

    render(
      <PartyTimelineEditor
        parties={[]}
        events={[]}
        onPartiesChange={jest.fn()}
        onEventsChange={onEventsChange}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /add event/i }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/api/v1/timeline/events',
        expect.objectContaining({ method: 'POST' }),
      );
      expect(onEventsChange).toHaveBeenCalledWith([
        expect.objectContaining({ description: '' }),
      ]);
    });
  });

  it('shows an inline error when adding a party fails', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Could not save party'));

    render(
      <PartyTimelineEditor
        parties={[]}
        events={[]}
        onPartiesChange={jest.fn()}
        onEventsChange={jest.fn()}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /add party/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/could not save party/i);
    });
  });

  it('removes a party when its remove button is clicked', async () => {
    const onPartiesChange = jest.fn();
    render(
      <PartyTimelineEditor
        parties={[{ id: 'party-1', role: 'first_party', name: 'Jane Doe' }]}
        events={[]}
        onPartiesChange={onPartiesChange}
        onEventsChange={jest.fn()}
      />,
    );
    await userEvent.click(screen.getByLabelText(/remove party jane doe/i));
    expect(onPartiesChange).toHaveBeenCalledWith([]);
  });
});
