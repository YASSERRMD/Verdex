/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { AnnotationsPanel } from '@/components/workspace/AnnotationsPanel';
import type { AnnotationEntry } from '@/types';

const ROOT: AnnotationEntry = {
  id: 'anno-1',
  caseId: 'case-1',
  authorId: 'user-1',
  authorName: 'Judge Doe',
  body: 'This finding looks unsupported by the evidence.',
  anchorType: 'case',
  anchorId: '',
  resolved: false,
  createdAt: '2026-01-01T10:00:00Z',
  updatedAt: '2026-01-01T10:00:00Z',
};

const REPLY: AnnotationEntry = {
  id: 'anno-2',
  caseId: 'case-1',
  authorId: 'user-2',
  authorName: 'Clerk Smith',
  body: 'Agreed, flagging for review.',
  anchorType: 'case',
  anchorId: '',
  parentId: 'anno-1',
  resolved: false,
  createdAt: '2026-01-01T11:00:00Z',
  updatedAt: '2026-01-01T11:00:00Z',
};

const RESOLVED_ROOT: AnnotationEntry = {
  ...ROOT,
  id: 'anno-3',
  body: 'Already handled.',
  resolved: true,
  resolvedBy: 'user-1',
  resolvedAt: '2026-01-02T09:00:00Z',
};

describe('AnnotationsPanel', () => {
  it('shows an empty state when there are no annotations', () => {
    render(
      <AnnotationsPanel
        annotations={[]}
        currentUserId="user-1"
        onCreate={jest.fn()}
        onToggleResolve={jest.fn()}
      />,
    );
    expect(screen.getByTestId('annotations-empty-state')).toBeInTheDocument();
    expect(screen.getByText(/no discussion yet/i)).toBeInTheDocument();
  });

  it('groups a flat annotation list into a thread with its reply nested under the root', () => {
    render(
      <AnnotationsPanel
        annotations={[ROOT, REPLY]}
        currentUserId="user-1"
        onCreate={jest.fn()}
        onToggleResolve={jest.fn()}
      />,
    );
    const thread = screen.getByTestId('annotation-thread-anno-1');
    expect(within(thread).getByTestId('annotation-anno-1')).toBeInTheDocument();
    expect(within(thread).getByTestId('annotation-anno-2')).toBeInTheDocument();
    expect(within(thread).getByText(ROOT.body)).toBeInTheDocument();
    expect(within(thread).getByText(REPLY.body)).toBeInTheDocument();
  });

  it('shows the open-thread count in the header', () => {
    render(
      <AnnotationsPanel
        annotations={[ROOT, RESOLVED_ROOT]}
        currentUserId="user-1"
        onCreate={jest.fn()}
        onToggleResolve={jest.fn()}
      />,
    );
    expect(screen.getByText('1 open thread')).toBeInTheDocument();
  });

  it('shows a resolved badge for a resolved thread root', () => {
    render(
      <AnnotationsPanel
        annotations={[RESOLVED_ROOT]}
        currentUserId="user-1"
        onCreate={jest.fn()}
        onToggleResolve={jest.fn()}
      />,
    );
    expect(screen.getByTestId(`annotation-resolved-badge-${RESOLVED_ROOT.id}`)).toBeInTheDocument();
  });

  it('posts a new case-level annotation from the compose box', async () => {
    const onCreate = jest.fn().mockResolvedValue(undefined);
    render(
      <AnnotationsPanel
        annotations={[]}
        currentUserId="user-1"
        onCreate={onCreate}
        onToggleResolve={jest.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText(/add a case note/i), {
      target: { value: 'New note from a reviewer.' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^post$/i }));

    await waitFor(() => expect(onCreate).toHaveBeenCalledTimes(1));
    expect(onCreate).toHaveBeenCalledWith({
      body: 'New note from a reviewer.',
      anchorType: 'case',
      anchorId: '',
    });
  });

  it('does not post a blank annotation', () => {
    const onCreate = jest.fn();
    render(
      <AnnotationsPanel
        annotations={[]}
        currentUserId="user-1"
        onCreate={onCreate}
        onToggleResolve={jest.fn()}
      />,
    );
    expect(screen.getByRole('button', { name: /^post$/i })).toBeDisabled();
    expect(onCreate).not.toHaveBeenCalled();
  });

  it('shows an inline error when creating an annotation fails', async () => {
    const onCreate = jest.fn().mockRejectedValue(new Error('Failed to post annotation.'));
    render(
      <AnnotationsPanel
        annotations={[]}
        currentUserId="user-1"
        onCreate={onCreate}
        onToggleResolve={jest.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText(/add a case note/i), {
      target: { value: 'This will fail.' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^post$/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to post annotation.');
  });

  it('replies to a thread root via its own reply box', async () => {
    const onCreate = jest.fn().mockResolvedValue(undefined);
    render(
      <AnnotationsPanel
        annotations={[ROOT]}
        currentUserId="user-1"
        onCreate={onCreate}
        onToggleResolve={jest.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /^reply$/i }));
    fireEvent.change(screen.getByLabelText(`Reply to annotation ${ROOT.id}`), {
      target: { value: 'A reply body.' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^reply$/i }));

    await waitFor(() =>
      expect(onCreate).toHaveBeenCalledWith({
        body: 'A reply body.',
        anchorType: 'case',
        anchorId: '',
        parentId: ROOT.id,
      }),
    );
  });

  it('toggles resolve for an open thread and reopen for a resolved one', async () => {
    const onToggleResolve = jest.fn().mockResolvedValue(undefined);
    const { rerender } = render(
      <AnnotationsPanel
        annotations={[ROOT]}
        currentUserId="user-1"
        onCreate={jest.fn()}
        onToggleResolve={onToggleResolve}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /^resolve$/i }));
    await waitFor(() => expect(onToggleResolve).toHaveBeenCalledWith(ROOT));

    rerender(
      <AnnotationsPanel
        annotations={[RESOLVED_ROOT]}
        currentUserId="user-1"
        onCreate={jest.fn()}
        onToggleResolve={onToggleResolve}
      />,
    );
    expect(screen.getByRole('button', { name: /^reopen$/i })).toBeInTheDocument();
  });

  it('hides the resolve toggle when there is no authenticated user', () => {
    render(
      <AnnotationsPanel
        annotations={[ROOT]}
        onCreate={jest.fn()}
        onToggleResolve={jest.fn()}
      />,
    );
    expect(screen.queryByRole('button', { name: /^resolve$/i })).not.toBeInTheDocument();
  });
});
