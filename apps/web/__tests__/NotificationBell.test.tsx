/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { NotificationBell } from '@/components/layout/NotificationBell';
import { apiFetch } from '@/lib/api';
import type { NotificationEntry } from '@/types';

jest.mock('@/lib/api', () => ({
  apiFetch: jest.fn(),
}));

const mockedApiFetch = apiFetch as jest.MockedFunction<typeof apiFetch>;

const UNREAD: NotificationEntry = {
  id: 'notif-1',
  tenantId: 'tenant-1',
  recipientId: 'user-1',
  kind: 'mention',
  title: 'You were mentioned',
  body: 'Judge Doe mentioned you in a case note.',
  createdAt: '2026-01-01T10:00:00Z',
};

const READ: NotificationEntry = {
  id: 'notif-2',
  tenantId: 'tenant-1',
  recipientId: 'user-1',
  kind: 'pending_signoff',
  title: 'Case awaiting your sign-off',
  createdAt: '2026-01-01T09:00:00Z',
  readAt: '2026-01-01T09:05:00Z',
};

function mockApi(overrides: {
  unreadCount?: number;
  list?: NotificationEntry[];
}) {
  mockedApiFetch.mockImplementation((path: string) => {
    if (path === '/api/v1/notifications/unread-count') {
      return Promise.resolve({ count: overrides.unreadCount ?? 0 });
    }
    if (path === '/api/v1/notifications') {
      return Promise.resolve(overrides.list ?? []);
    }
    if (path.endsWith('/read') || path === '/api/v1/notifications/mark-all-read') {
      return Promise.resolve({});
    }
    return Promise.reject(new Error(`unexpected path: ${path}`));
  });
}

afterEach(() => {
  jest.clearAllMocks();
});

describe('NotificationBell', () => {
  it('shows the unread count badge', async () => {
    mockApi({ unreadCount: 3 });
    render(<NotificationBell />);

    expect(await screen.findByTestId('notification-unread-badge')).toHaveTextContent('3');
  });

  it('hides the badge when there are no unread notifications', async () => {
    mockApi({ unreadCount: 0 });
    render(<NotificationBell />);

    await waitFor(() => expect(mockedApiFetch).toHaveBeenCalledWith('/api/v1/notifications/unread-count'));
    expect(screen.queryByTestId('notification-unread-badge')).not.toBeInTheDocument();
  });

  it('shows an empty state when there are no notifications', async () => {
    mockApi({ unreadCount: 0, list: [] });
    render(<NotificationBell />);

    fireEvent.click(screen.getByRole('button', { name: /notifications/i }));

    expect(await screen.findByTestId('notification-empty-state')).toBeInTheDocument();
  });

  it('lists notifications with unread markers, opened from the bell', async () => {
    mockApi({ unreadCount: 1, list: [UNREAD, READ] });
    render(<NotificationBell />);

    fireEvent.click(screen.getByRole('button', { name: /notifications/i }));

    expect(await screen.findByTestId(`notification-${UNREAD.id}`)).toBeInTheDocument();
    expect(screen.getByTestId(`notification-${READ.id}`)).toBeInTheDocument();
    expect(screen.getByTestId(`notification-unread-dot-${UNREAD.id}`)).toBeInTheDocument();
    expect(screen.queryByTestId(`notification-unread-dot-${READ.id}`)).not.toBeInTheDocument();
    expect(screen.getByText(UNREAD.title)).toBeInTheDocument();
  });

  it('marks a single notification read on click and decrements the badge', async () => {
    mockApi({ unreadCount: 1, list: [UNREAD] });
    render(<NotificationBell />);

    expect(await screen.findByTestId('notification-unread-badge')).toHaveTextContent('1');

    fireEvent.click(screen.getByRole('button', { name: /notifications/i }));
    const item = await screen.findByTestId(`notification-${UNREAD.id}`);
    fireEvent.click(item);

    await waitFor(() =>
      expect(mockedApiFetch).toHaveBeenCalledWith(
        `/api/v1/notifications/${UNREAD.id}/read`,
        expect.objectContaining({ method: 'POST' }),
      ),
    );
    expect(screen.queryByTestId('notification-unread-badge')).not.toBeInTheDocument();
    expect(screen.queryByTestId(`notification-unread-dot-${UNREAD.id}`)).not.toBeInTheDocument();
  });

  it('marks all notifications read via the mark-all-read action', async () => {
    mockApi({ unreadCount: 2, list: [UNREAD, { ...UNREAD, id: 'notif-3', title: 'Second' }] });
    render(<NotificationBell />);

    fireEvent.click(screen.getByRole('button', { name: /notifications/i }));
    await screen.findByTestId(`notification-${UNREAD.id}`);

    fireEvent.click(screen.getByRole('button', { name: /mark all read/i }));

    await waitFor(() =>
      expect(mockedApiFetch).toHaveBeenCalledWith(
        '/api/v1/notifications/mark-all-read',
        expect.objectContaining({ method: 'POST' }),
      ),
    );
    expect(screen.queryByTestId('notification-unread-badge')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /mark all read/i })).not.toBeInTheDocument();
  });
});
