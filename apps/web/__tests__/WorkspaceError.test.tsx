/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkspaceError } from '@/components/workspace/WorkspaceError';

describe('WorkspaceError', () => {
  it('renders the error message as an alert', () => {
    render(<WorkspaceError message="Case not found." />);
    expect(screen.getByRole('alert')).toHaveTextContent(/case not found/i);
  });

  it('does not render a retry button when onRetry is not provided', () => {
    render(<WorkspaceError message="Case not found." />);
    expect(screen.queryByRole('button', { name: /retry/i })).not.toBeInTheDocument();
  });

  it('calls onRetry when the retry button is clicked', async () => {
    const onRetry = jest.fn();
    render(<WorkspaceError message="Failed to load case." onRetry={onRetry} />);
    await userEvent.click(screen.getByRole('button', { name: /retry/i }));
    expect(onRetry).toHaveBeenCalled();
  });
});
