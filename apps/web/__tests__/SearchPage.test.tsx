/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import SearchPage from '@/app/search/page';
import type { SearchResults } from '@/types';

const mockReplace = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace: mockReplace, push: jest.fn() }),
  usePathname: () => '/search',
}));

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

jest.mock('@/lib/auth', () => ({
  getSession: jest.fn(),
}));

import { apiFetch } from '@/lib/api';
import { getSession } from '@/lib/auth';

const mockApiFetch = apiFetch as jest.MockedFunction<typeof apiFetch>;
const mockGetSession = getSession as jest.MockedFunction<typeof getSession>;

const RESULTS: SearchResults = {
  items: [
    {
      caseId: 'case-1',
      title: 'Doe v. Acme Corp',
      reference: '',
      categoryId: 'civil',
      jurisdictionId: 'jur-1',
      state: 'active',
      createdAt: '2026-01-01T00:00:00Z',
      mode: 'keyword',
      score: 0.9,
      snippet: 'The tenant **breached** the lease.',
      hits: [],
    },
  ],
  totalMatches: 1,
  page: { number: 1, size: 20 },
  mode: 'keyword',
  skippedCases: 0,
};

describe('SearchPage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockApiFetch.mockReset();
    mockGetSession.mockReturnValue({
      token: 'test-token',
      user: { id: 'user-1', name: 'Jane Judge', email: 'jane@example.com', roles: ['judge'] },
    });
    // The page loads saved searches on mount; default to an empty list so
    // tests only asserting search behaviour don't need to mock it every
    // time.
    mockApiFetch.mockResolvedValue([]);
  });

  it('redirects to login when there is no session', () => {
    mockGetSession.mockReturnValue(null);
    render(<SearchPage />);
    expect(mockReplace).toHaveBeenCalledWith('/login');
  });

  it('shows a prompt before any search has run', () => {
    render(<SearchPage />);
    expect(screen.getByText(/enter a search term/i)).toBeInTheDocument();
  });

  it('runs a search and renders the results', async () => {
    mockApiFetch.mockImplementation((path: string) => {
      if (path.startsWith('/api/v1/search/saved')) return Promise.resolve([]);
      if (path.startsWith('/api/v1/search?')) return Promise.resolve(RESULTS);
      return Promise.reject(new Error(`unexpected path ${path}`));
    });

    render(<SearchPage />);

    fireEvent.change(screen.getByLabelText(/search query/i), { target: { value: 'breach' } });
    fireEvent.click(screen.getByRole('button', { name: /^search$/i }));

    await waitFor(() => {
      expect(screen.getByText('Doe v. Acme Corp')).toBeInTheDocument();
    });

    const searchCall = mockApiFetch.mock.calls.find(([path]) =>
      String(path).startsWith('/api/v1/search?'),
    );
    expect(searchCall?.[0]).toContain('q=breach');
  });

  it('shows an error message when the search request fails', async () => {
    mockApiFetch.mockImplementation((path: string) => {
      if (path.startsWith('/api/v1/search/saved')) return Promise.resolve([]);
      return Promise.reject(new Error('Search failed.'));
    });

    render(<SearchPage />);
    fireEvent.change(screen.getByLabelText(/search query/i), { target: { value: 'breach' } });
    fireEvent.click(screen.getByRole('button', { name: /^search$/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Search failed.');
    });
  });

  it('loads saved searches on mount and lists them', async () => {
    mockApiFetch.mockImplementation((path: string) => {
      if (path.startsWith('/api/v1/search/saved')) {
        return Promise.resolve([
          {
            id: 'saved-1',
            name: 'My saved search',
            query: { text: 'breach', mode: 'keyword' },
            createdAt: '2026-01-01T00:00:00Z',
          },
        ]);
      }
      return Promise.resolve(RESULTS);
    });

    render(<SearchPage />);

    await waitFor(() => {
      expect(screen.getByText('My saved search')).toBeInTheDocument();
    });
  });
});
