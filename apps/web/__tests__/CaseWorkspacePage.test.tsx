/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import CaseWorkspacePage from '@/app/cases/[caseId]/page';
import type { CaseLifecycle } from '@/types';

const mockReplace = jest.fn();
const mockPush = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace: mockReplace, push: mockPush }),
  useParams: () => ({ caseId: 'case-1' }),
  usePathname: () => '/cases/case-1',
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

import { apiFetch, ApiError } from '@/lib/api';
import { getSession } from '@/lib/auth';

const mockApiFetch = apiFetch as jest.MockedFunction<typeof apiFetch>;
const mockGetSession = getSession as jest.MockedFunction<typeof getSession>;

const CASE_DATA: CaseLifecycle = {
  id: 'case-1',
  tenantId: 'tenant-1',
  jurisdictionId: 'jur-1',
  jurisdictionName: 'District Court',
  categoryId: 'civil',
  categoryLabel: 'Civil',
  title: 'Doe v. Acme Corp',
  state: 'active',
  metadata: {},
  metadataVersion: 1,
  createdBy: 'user-1',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

describe('CaseWorkspacePage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockApiFetch.mockReset();
    mockGetSession.mockReturnValue({
      token: 'test-token',
      user: { id: 'user-1', name: 'Jane Judge', email: 'jane@example.com', roles: ['judge'] },
    });
  });

  it('redirects to login when there is no session', () => {
    mockGetSession.mockReturnValue(null);
    render(<CaseWorkspacePage />);
    expect(mockReplace).toHaveBeenCalledWith('/login');
  });

  it('shows a loading state while the case is being fetched', () => {
    mockApiFetch.mockReturnValueOnce(new Promise(() => {}));
    render(<CaseWorkspacePage />);
    expect(screen.getByTestId('workspace-loading')).toBeInTheDocument();
  });

  it('renders the case header and status bar once loaded', async () => {
    mockApiFetch.mockResolvedValueOnce({
      caseData: CASE_DATA,
      parties: [],
      evidence: [],
      events: [],
    });
    render(<CaseWorkspacePage />);

    await waitFor(() => {
      expect(screen.getByText('Doe v. Acme Corp')).toBeInTheDocument();
    });
    expect(screen.getByTestId('status-bar-state-badge')).toHaveTextContent('Active');
  });

  it('shows a not-found message when the case does not exist', async () => {
    mockApiFetch.mockRejectedValueOnce(new ApiError(404, 'not found'));
    render(<CaseWorkspacePage />);

    await waitFor(() => {
      expect(screen.getByTestId('workspace-error')).toHaveTextContent(/case not found/i);
    });
  });

  it('shows a generic error message on other failures', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Network error'));
    render(<CaseWorkspacePage />);

    await waitFor(() => {
      expect(screen.getByTestId('workspace-error')).toHaveTextContent(/network error/i);
    });
  });

  it('switches panels when a different tab is selected', async () => {
    mockApiFetch.mockResolvedValueOnce({
      caseData: CASE_DATA,
      parties: [{ id: 'party-1', role: 'first_party', name: 'Jane Doe' }],
      evidence: [],
      events: [],
    });
    render(<CaseWorkspacePage />);

    await waitFor(() => expect(screen.getByText('Jane Doe')).toBeInTheDocument());

    await userEvent.click(screen.getByRole('tab', { name: /reasoning tree/i }));
    expect(screen.getByText(/no reasoning tree yet/i)).toBeInTheDocument();
    expect(screen.queryByText('Jane Doe')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('tab', { name: /draft opinion/i }));
    expect(screen.getByLabelText(/non-binding disclaimer/i)).toBeInTheDocument();
  });

  it('renders the evidence review tab with the case evidence segments', async () => {
    mockApiFetch.mockResolvedValueOnce({
      caseData: CASE_DATA,
      parties: [],
      evidence: [
        {
          id: 'seg-1',
          text: 'Witness testimony describing the incident.',
          type: 'testimony',
          party: 'first_party',
          confidence: 0.82,
          sourceSpan: { start: 0, end: 10 },
        },
      ],
      events: [],
    });
    render(<CaseWorkspacePage />);

    await waitFor(() => expect(screen.getByRole('tab', { name: /^evidence review$/i })).toBeInTheDocument());

    await userEvent.click(screen.getByRole('tab', { name: /^evidence review$/i }));
    expect(screen.getByTestId('evidence-review-segment-seg-1')).toBeInTheDocument();
    expect(screen.getByText(/review and correct extracted evidence segments/i)).toBeInTheDocument();
  });
});
