/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { SavedSearchesPanel } from '@/components/search/SavedSearchesPanel';
import type { SavedSearchEntry } from '@/types';

const SAVED: SavedSearchEntry[] = [
  {
    id: 'saved-1',
    name: 'Breach cases',
    query: { text: 'breach', mode: 'keyword' },
    createdAt: '2026-01-01T00:00:00Z',
  },
];

describe('SavedSearchesPanel', () => {
  it('shows an empty state when there are no saved searches', () => {
    render(<SavedSearchesPanel savedSearches={[]} onRun={jest.fn()} onDelete={jest.fn()} />);
    expect(screen.getByText(/no saved searches/i)).toBeInTheDocument();
  });

  it('lists each saved search by name', () => {
    render(<SavedSearchesPanel savedSearches={SAVED} onRun={jest.fn()} onDelete={jest.fn()} />);
    expect(screen.getByText('Breach cases')).toBeInTheDocument();
  });

  it('runs a saved search when its name is clicked', () => {
    const onRun = jest.fn();
    render(<SavedSearchesPanel savedSearches={SAVED} onRun={onRun} onDelete={jest.fn()} />);
    fireEvent.click(screen.getByText('Breach cases'));
    expect(onRun).toHaveBeenCalledWith(SAVED[0]);
  });

  it('deletes a saved search when its delete button is clicked', () => {
    const onDelete = jest.fn();
    render(<SavedSearchesPanel savedSearches={SAVED} onRun={jest.fn()} onDelete={onDelete} />);
    fireEvent.click(screen.getByRole('button', { name: /delete saved search breach cases/i }));
    expect(onDelete).toHaveBeenCalledWith(SAVED[0]);
  });
});
