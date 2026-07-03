/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { WorkspaceLoading } from '@/components/workspace/WorkspaceLoading';

describe('WorkspaceLoading', () => {
  it('renders a loading indicator and message', () => {
    render(<WorkspaceLoading />);
    expect(screen.getByTestId('workspace-loading')).toBeInTheDocument();
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByText(/loading case/i)).toBeInTheDocument();
  });
});
