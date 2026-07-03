'use client';

import { useMemo, useState } from 'react';
import clsx from 'clsx';
import {
  CheckCircle2Icon,
  MessageSquarePlusIcon,
  MessageSquareTextIcon,
  RotateCcwIcon,
} from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import type { AnnotationEntry } from '@/types';

export interface AnnotationsPanelProps {
  /** Every annotation loaded for the case (all anchor types, flattened). */
  annotations: AnnotationEntry[];
  /** The currently authenticated user's ID, used to gate edit/delete UI. */
  currentUserId?: string;
  /**
   * Called to create a new annotation. Rejects/throws are surfaced as an
   * inline error by this panel.
   */
  onCreate: (input: {
    body: string;
    anchorType: AnnotationEntry['anchorType'];
    anchorId: string;
    parentId?: string;
  }) => Promise<void> | void;
  /** Called to toggle an annotation's resolved state. */
  onToggleResolve: (annotation: AnnotationEntry) => Promise<void> | void;
  className?: string;
}

function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

/** Groups a flat annotation list into thread roots with their ordered replies. */
function groupThreads(items: AnnotationEntry[]): { root: AnnotationEntry; replies: AnnotationEntry[] }[] {
  const roots = items.filter((a) => !a.parentId);
  const repliesByParent = new Map<string, AnnotationEntry[]>();
  for (const a of items) {
    if (!a.parentId) continue;
    const list = repliesByParent.get(a.parentId) ?? [];
    list.push(a);
    repliesByParent.set(a.parentId, list);
  }
  return roots
    .slice()
    .sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime())
    .map((root) => ({
      root,
      replies: (repliesByParent.get(root.id) ?? []).sort(
        (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime(),
      ),
    }));
}

function AnnotationCard({
  annotation,
  currentUserId,
  onToggleResolve,
  isReply = false,
}: {
  annotation: AnnotationEntry;
  currentUserId?: string;
  onToggleResolve: (annotation: AnnotationEntry) => Promise<void> | void;
  isReply?: boolean;
}) {
  const [busy, setBusy] = useState(false);
  const canResolve = !!currentUserId;

  const handleToggle = async () => {
    setBusy(true);
    try {
      await onToggleResolve(annotation);
    } finally {
      setBusy(false);
    }
  };

  return (
    <li
      data-testid={`annotation-${annotation.id}`}
      className={clsx(
        'rounded-lg border px-3 py-2.5 text-sm',
        isReply
          ? 'ml-6 border-neutral-200 bg-neutral-50 dark:border-neutral-700 dark:bg-neutral-900/40'
          : 'border-neutral-200 dark:border-neutral-700',
      )}
    >
      <p className="whitespace-pre-wrap text-neutral-800 dark:text-neutral-100">{annotation.body}</p>
      <div className="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-neutral-500">
        <span>{annotation.authorName ?? annotation.authorId}</span>
        <span aria-hidden="true">·</span>
        <span>{formatTimestamp(annotation.createdAt)}</span>
        {annotation.resolved && (
          <span
            data-testid={`annotation-resolved-badge-${annotation.id}`}
            className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
          >
            <CheckCircle2Icon className="h-3 w-3" aria-hidden="true" />
            Resolved
          </span>
        )}
        {!isReply && canResolve && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            loading={busy}
            onClick={handleToggle}
            leftIcon={
              annotation.resolved ? (
                <RotateCcwIcon className="h-3.5 w-3.5" />
              ) : (
                <CheckCircle2Icon className="h-3.5 w-3.5" />
              )
            }
          >
            {annotation.resolved ? 'Reopen' : 'Resolve'}
          </Button>
        )}
      </div>
    </li>
  );
}

function ThreadSection({
  root,
  replies,
  currentUserId,
  onToggleResolve,
  onReply,
}: {
  root: AnnotationEntry;
  replies: AnnotationEntry[];
  currentUserId?: string;
  onToggleResolve: (annotation: AnnotationEntry) => Promise<void> | void;
  onReply: (parentId: string, body: string) => Promise<void> | void;
}) {
  const [replyDraft, setReplyDraft] = useState('');
  const [showReplyBox, setShowReplyBox] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmitReply = async () => {
    const trimmed = replyDraft.trim();
    if (!trimmed) return;
    setSubmitting(true);
    try {
      await onReply(root.id, trimmed);
      setReplyDraft('');
      setShowReplyBox(false);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div data-testid={`annotation-thread-${root.id}`} className="space-y-2">
      <AnnotationCard annotation={root} currentUserId={currentUserId} onToggleResolve={onToggleResolve} />
      {replies.length > 0 && (
        <ul className="space-y-2">
          {replies.map((reply) => (
            <AnnotationCard
              key={reply.id}
              annotation={reply}
              currentUserId={currentUserId}
              onToggleResolve={onToggleResolve}
              isReply
            />
          ))}
        </ul>
      )}
      <div className="ml-6">
        {showReplyBox ? (
          <div className="flex items-start gap-2">
            <textarea
              aria-label={`Reply to annotation ${root.id}`}
              value={replyDraft}
              onChange={(event) => setReplyDraft(event.target.value)}
              placeholder="Write a reply…"
              rows={2}
              className="block w-full flex-1 rounded-lg border border-neutral-300 px-3 py-2 text-sm shadow-sm placeholder:text-neutral-400 focus:border-primary-DEFAULT focus:outline-none focus:ring-2 focus:ring-primary-DEFAULT/30 dark:border-neutral-600 dark:bg-neutral-900"
            />
            <Button
              type="button"
              size="sm"
              variant="secondary"
              loading={submitting}
              disabled={!replyDraft.trim()}
              onClick={handleSubmitReply}
            >
              Reply
            </Button>
          </div>
        ) : (
          <button
            type="button"
            onClick={() => setShowReplyBox(true)}
            className="text-xs font-medium text-primary-DEFAULT hover:underline"
          >
            Reply
          </button>
        )}
      </div>
    </div>
  );
}

/**
 * The case-level "Discussion" panel: a flat list of threaded annotations
 * (root + one level of replies) with a compose box, a resolve/reopen
 * toggle per thread, and mention hints. Reachable from the case workspace
 * as its own tab (see WorkspaceTabs) so reviewers can leave notes without
 * navigating away from — or duplicating — the tree/evidence views those
 * notes may reference via anchorType/anchorId. See
 * apps/web/docs/annotations-ui.md.
 */
export function AnnotationsPanel({
  annotations,
  currentUserId,
  onCreate,
  onToggleResolve,
  className,
}: AnnotationsPanelProps) {
  const [draft, setDraft] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const threads = useMemo(() => groupThreads(annotations), [annotations]);
  const openCount = threads.filter((t) => !t.root.resolved).length;

  const handleSubmit = async () => {
    const trimmed = draft.trim();
    if (!trimmed) return;
    setSubmitting(true);
    setError(null);
    try {
      await onCreate({ body: trimmed, anchorType: 'case', anchorId: '' });
      setDraft('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to post annotation.');
    } finally {
      setSubmitting(false);
    }
  };

  const handleReply = async (parentId: string, body: string) => {
    setError(null);
    try {
      await onCreate({ body, anchorType: 'case', anchorId: '', parentId });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to post reply.');
    }
  };

  return (
    <div className={clsx('space-y-4', className)}>
      <Card
        header={
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-base font-semibold text-neutral-800 dark:text-white">Discussion</h2>
            <span className="text-xs text-neutral-500">
              {openCount} open {openCount === 1 ? 'thread' : 'threads'}
            </span>
          </div>
        }
      >
        {error && (
          <p role="alert" className="mb-3 rounded-md bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300">
            {error}
          </p>
        )}

        <div className="mb-4 flex items-start gap-2">
          <textarea
            aria-label="Add a case note"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            placeholder="Add a note or @mention a reviewer by ID…"
            rows={2}
            className="block w-full flex-1 rounded-lg border border-neutral-300 px-3 py-2 text-sm shadow-sm placeholder:text-neutral-400 focus:border-primary-DEFAULT focus:outline-none focus:ring-2 focus:ring-primary-DEFAULT/30 dark:border-neutral-600 dark:bg-neutral-900"
          />
          <Button
            type="button"
            size="sm"
            loading={submitting}
            disabled={!draft.trim()}
            onClick={handleSubmit}
            leftIcon={<MessageSquarePlusIcon className="h-3.5 w-3.5" />}
          >
            Post
          </Button>
        </div>

        {threads.length === 0 ? (
          <div
            data-testid="annotations-empty-state"
            className="flex flex-col items-center justify-center gap-3 py-16 text-center"
          >
            <MessageSquareTextIcon className="h-10 w-10 text-neutral-300" aria-hidden="true" />
            <p className="text-sm font-medium text-neutral-600 dark:text-neutral-300">No discussion yet</p>
            <p className="max-w-sm text-xs text-neutral-400">
              Notes, highlights, and replies left by reviewers on this case will appear here.
            </p>
          </div>
        ) : (
          <div data-testid="annotations-thread-list" className="space-y-4">
            {threads.map(({ root, replies }) => (
              <ThreadSection
                key={root.id}
                root={root}
                replies={replies}
                currentUserId={currentUserId}
                onToggleResolve={onToggleResolve}
                onReply={handleReply}
              />
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
