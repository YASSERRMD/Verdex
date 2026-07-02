/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CaseCreationForm } from '@/components/ingestion/CaseCreationForm';

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

describe('CaseCreationForm', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders category and party fields and a submit button', () => {
    render(<CaseCreationForm />);
    expect(screen.getByLabelText(/case category/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/first party name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/second party name/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create case/i })).toBeInTheDocument();
  });

  it('shows validation errors when submitted empty', async () => {
    render(<CaseCreationForm />);
    fireEvent.click(screen.getByRole('button', { name: /create case/i }));
    await waitFor(() => {
      expect(screen.getByText(/please select a case category/i)).toBeInTheDocument();
      expect(screen.getByText(/first party name is required/i)).toBeInTheDocument();
      expect(screen.getByText(/second party name is required/i)).toBeInTheDocument();
    });
    expect(mockApiFetch).not.toHaveBeenCalled();
  });

  it('submits with correct payload when valid', async () => {
    mockApiFetch.mockResolvedValueOnce({ id: 'case-123' });
    const onCreated = jest.fn();

    render(<CaseCreationForm onCreated={onCreated} />);
    await userEvent.selectOptions(screen.getByLabelText(/case category/i), 'civil');
    await userEvent.type(screen.getByLabelText(/first party name/i), 'Jane Doe');
    await userEvent.type(screen.getByLabelText(/second party name/i), 'Acme Corp');
    fireEvent.click(screen.getByRole('button', { name: /create case/i }));

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/api/v1/cases',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            category: 'civil',
            firstPartyName: 'Jane Doe',
            secondPartyName: 'Acme Corp',
          }),
        }),
      );
      expect(onCreated).toHaveBeenCalledWith('case-123');
    });
  });

  it('shows an error message when the API call fails', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Server unavailable'));

    render(<CaseCreationForm />);
    await userEvent.selectOptions(screen.getByLabelText(/case category/i), 'civil');
    await userEvent.type(screen.getByLabelText(/first party name/i), 'Jane Doe');
    await userEvent.type(screen.getByLabelText(/second party name/i), 'Acme Corp');
    fireEvent.click(screen.getByRole('button', { name: /create case/i }));

    await waitFor(() => {
      expect(screen.getByText(/server unavailable/i)).toBeInTheDocument();
    });
  });
});
