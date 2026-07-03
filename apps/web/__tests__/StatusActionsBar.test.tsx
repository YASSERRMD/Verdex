/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StatusActionsBar } from '@/components/workspace/StatusActionsBar';

describe('StatusActionsBar', () => {
  it('shows "Activate" as the only action for a draft case', () => {
    render(<StatusActionsBar state="draft" />);
    expect(screen.getByTestId('status-bar-state-badge')).toHaveTextContent('Draft');
    expect(screen.getByRole('button', { name: /activate/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /reopen/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /archive/i })).not.toBeInTheDocument();
  });

  it('shows "Submit for Review" for an active case', () => {
    render(<StatusActionsBar state="active" />);
    expect(screen.getByRole('button', { name: /submit for review/i })).toBeInTheDocument();
  });

  it('shows both "Close Case" and "Send Back to Active" for an under_review case', () => {
    render(<StatusActionsBar state="under_review" />);
    expect(screen.getByRole('button', { name: /close case/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /send back to active/i })).toBeInTheDocument();
  });

  it('shows "Reopen" and "Archive" for a closed case, and no forward transitions', () => {
    render(<StatusActionsBar state="closed" />);
    expect(screen.getByRole('button', { name: /^reopen$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /archive/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /activate/i })).not.toBeInTheDocument();
  });

  it('shows no actions for an archived case', () => {
    render(<StatusActionsBar state="archived" />);
    expect(screen.getByText(/no actions available in this state/i)).toBeInTheDocument();
  });

  it('calls onTransition with the target state when a transition button is clicked', async () => {
    const onTransition = jest.fn();
    render(<StatusActionsBar state="active" onTransition={onTransition} />);
    await userEvent.click(screen.getByRole('button', { name: /submit for review/i }));
    expect(onTransition).toHaveBeenCalledWith('under_review');
  });

  it('requires a non-blank justification before confirming Reopen', async () => {
    const onReopen = jest.fn();
    render(<StatusActionsBar state="closed" onReopen={onReopen} />);
    await userEvent.click(screen.getByRole('button', { name: /^reopen$/i }));

    const confirmButton = screen.getByRole('button', { name: /confirm reopen/i });
    expect(confirmButton).toBeDisabled();

    await userEvent.type(
      screen.getByTestId('reopen-justification-input'),
      'Reopened due to new evidence',
    );
    expect(confirmButton).toBeEnabled();

    await userEvent.click(confirmButton);
    expect(onReopen).toHaveBeenCalledWith('Reopened due to new evidence');
  });

  it('calls onArchive when Archive is clicked', async () => {
    const onArchive = jest.fn();
    render(<StatusActionsBar state="closed" onArchive={onArchive} />);
    await userEvent.click(screen.getByRole('button', { name: /archive/i }));
    expect(onArchive).toHaveBeenCalled();
  });
});
