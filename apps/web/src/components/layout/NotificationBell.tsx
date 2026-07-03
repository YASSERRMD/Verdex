'use client';

import { useEffect, useRef, useState } from 'react';
import { BellIcon, CheckCheckIcon, InboxIcon } from 'lucide-react';
import clsx from 'clsx';
import { apiFetch } from '@/lib/api';
import type { NotificationEntry } from '@/types';

const POLL_INTERVAL_MS = 30000;

function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

const KIND_LABEL: Record<NotificationEntry['kind'], string> = {
  ingestion_complete: 'Ingestion complete',
  pending_signoff: 'Sign-off',
  mention: 'Mention',
  quality_alert: 'Quality alert',
  budget_alert: 'Budget alert',
  task_assignment: 'Assignment',
};

/**
 * The notification bell + inbox dropdown shown in TopBar: unread
 * count badge, a recent-notifications list, per-item mark-read, and a
 * mark-all-read action. Backed by packages/notifications via
 * GET /api/v1/notifications, GET /api/v1/notifications/unread-count,
 * POST /api/v1/notifications/:id/read, and
 * POST /api/v1/notifications/mark-all-read. See
 * packages/notifications/doc/notifications.md, "Web UI".
 */
export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const [notifications, setNotifications] = useState<NotificationEntry[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Poll the unread count in the background so the badge stays fresh
  // even while the dropdown is closed.
  useEffect(() => {
    let cancelled = false;

    const refreshCount = () => {
      apiFetch<{ count: number }>('/api/v1/notifications/unread-count')
        .then((result) => {
          if (!cancelled) setUnreadCount(result.count);
        })
        .catch(() => {
          // No notifications endpoint yet in this build; leave the
          // badge at its last known value rather than erroring.
        });
    };

    refreshCount();
    const interval = setInterval(refreshCount, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  // Load the notification list lazily the first time the dropdown opens.
  useEffect(() => {
    if (!open || loaded) return;
    let cancelled = false;
    apiFetch<NotificationEntry[]>('/api/v1/notifications')
      .then((result) => {
        if (!cancelled) {
          setNotifications(result);
          setLoaded(true);
        }
      })
      .catch(() => {
        if (!cancelled) setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [open, loaded]);

  // Close on outside click.
  useEffect(() => {
    if (!open) return;
    const handleClick = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [open]);

  const handleMarkRead = async (notification: NotificationEntry) => {
    if (notification.readAt) return;
    const now = new Date().toISOString();
    setNotifications((prev) =>
      prev.map((n) => (n.id === notification.id ? { ...n, readAt: now } : n)),
    );
    setUnreadCount((prev) => Math.max(0, prev - 1));
    try {
      await apiFetch(`/api/v1/notifications/${notification.id}/read`, { method: 'POST' });
    } catch {
      // Best-effort: the optimistic update stands even if the request
      // ultimately fails, mirroring AnnotationsPanel's posture of not
      // rolling back on a background sync failure for a read receipt.
    }
  };

  const handleMarkAllRead = async () => {
    const now = new Date().toISOString();
    setNotifications((prev) => prev.map((n) => (n.readAt ? n : { ...n, readAt: now })));
    setUnreadCount(0);
    try {
      await apiFetch('/api/v1/notifications/mark-all-read', { method: 'POST' });
    } catch {
      // Best-effort, same as handleMarkRead.
    }
  };

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="relative flex h-9 w-9 items-center justify-center rounded-lg text-neutral-500 hover:bg-neutral-100 hover:text-neutral-700 dark:text-neutral-400 dark:hover:bg-neutral-700"
        aria-label="Notifications"
        aria-expanded={open}
      >
        <BellIcon className="h-5 w-5" aria-hidden="true" />
        {unreadCount > 0 && (
          <span
            data-testid="notification-unread-badge"
            className="absolute right-1 top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-semibold leading-none text-white"
            aria-hidden="true"
          >
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div
          data-testid="notification-dropdown"
          className="absolute right-0 top-12 z-50 max-h-96 w-80 overflow-y-auto rounded-xl border border-neutral-200 bg-white shadow-lg dark:border-neutral-700 dark:bg-neutral-800"
        >
          <div className="flex items-center justify-between border-b border-neutral-200 px-4 py-3 dark:border-neutral-700">
            <h2 className="text-sm font-semibold text-neutral-800 dark:text-white">
              Notifications
            </h2>
            {unreadCount > 0 && (
              <button
                type="button"
                onClick={handleMarkAllRead}
                className="flex items-center gap-1 text-xs font-medium text-primary-DEFAULT hover:underline"
              >
                <CheckCheckIcon className="h-3.5 w-3.5" aria-hidden="true" />
                Mark all read
              </button>
            )}
          </div>

          {notifications.length === 0 ? (
            <div
              data-testid="notification-empty-state"
              className="flex flex-col items-center justify-center gap-2 px-4 py-10 text-center"
            >
              <InboxIcon className="h-8 w-8 text-neutral-300" aria-hidden="true" />
              <p className="text-sm font-medium text-neutral-600 dark:text-neutral-300">
                No notifications yet
              </p>
            </div>
          ) : (
            <ul data-testid="notification-list" className="divide-y divide-neutral-100 dark:divide-neutral-700">
              {notifications.map((n) => (
                <li key={n.id}>
                  <button
                    type="button"
                    data-testid={`notification-${n.id}`}
                    onClick={() => handleMarkRead(n)}
                    className={clsx(
                      'block w-full px-4 py-3 text-left transition-colors hover:bg-neutral-50 dark:hover:bg-neutral-700/50',
                      !n.readAt && 'bg-primary-50/50 dark:bg-primary-900/10',
                    )}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <span className="text-xs font-medium uppercase tracking-wide text-neutral-400">
                        {KIND_LABEL[n.kind] ?? n.kind}
                      </span>
                      {!n.readAt && (
                        <span
                          data-testid={`notification-unread-dot-${n.id}`}
                          className="mt-1 h-2 w-2 shrink-0 rounded-full bg-primary-DEFAULT"
                          aria-hidden="true"
                        />
                      )}
                    </div>
                    <p className="mt-1 text-sm font-medium text-neutral-800 dark:text-neutral-100">
                      {n.title}
                    </p>
                    {n.body && (
                      <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">{n.body}</p>
                    )}
                    <p className="mt-1 text-xs text-neutral-400">{formatTimestamp(n.createdAt)}</p>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
