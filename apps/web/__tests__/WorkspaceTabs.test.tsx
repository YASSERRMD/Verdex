/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkspaceTabs } from '@/components/workspace/WorkspaceTabs';

describe('WorkspaceTabs', () => {
  it('renders all six workspace tabs', () => {
    render(<WorkspaceTabs activeTab="overview" onTabChange={jest.fn()} />);
    expect(screen.getByRole('tab', { name: /overview/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /evidence & timeline/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /^evidence review$/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /reasoning tree/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /draft opinion/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /discussion/i })).toBeInTheDocument();
  });

  it('marks the active tab as selected', () => {
    render(<WorkspaceTabs activeTab="evidence" onTabChange={jest.fn()} />);
    expect(screen.getByRole('tab', { name: /evidence & timeline/i })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByRole('tab', { name: /overview/i })).toHaveAttribute(
      'aria-selected',
      'false',
    );
  });

  it('calls onTabChange with the clicked tab id', async () => {
    const onTabChange = jest.fn();
    render(<WorkspaceTabs activeTab="overview" onTabChange={onTabChange} />);
    await userEvent.click(screen.getByRole('tab', { name: /reasoning tree/i }));
    expect(onTabChange).toHaveBeenCalledWith('tree');
  });

  it('calls onTabChange with "discussion" when the Discussion tab is clicked', async () => {
    const onTabChange = jest.fn();
    render(<WorkspaceTabs activeTab="overview" onTabChange={onTabChange} />);
    await userEvent.click(screen.getByRole('tab', { name: /discussion/i }));
    expect(onTabChange).toHaveBeenCalledWith('discussion');
  });
});
