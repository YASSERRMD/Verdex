/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { SearchFilterPanel } from '@/components/search/SearchFilterPanel';
import type { SearchFilters, SearchMode } from '@/types';

function Harness({ onSubmit }: { onSubmit: (q: string, m: SearchMode, f: SearchFilters) => void }) {
  const [query, setQuery] = React.useState('');
  const [mode, setMode] = React.useState<SearchMode>('');
  const [filters, setFilters] = React.useState<SearchFilters>({});

  return (
    <SearchFilterPanel
      query={query}
      onQueryChange={setQuery}
      mode={mode}
      onModeChange={setMode}
      filters={filters}
      onFiltersChange={setFilters}
      onSubmit={() => onSubmit(query, mode, filters)}
    />
  );
}

describe('SearchFilterPanel', () => {
  it('renders a query box, mode select, and filter fields', () => {
    render(<Harness onSubmit={jest.fn()} />);
    expect(screen.getByLabelText(/search query/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^category$/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/party name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^state$/i)).toBeInTheDocument();
  });

  it('submits the current query, mode, and filters', () => {
    const onSubmit = jest.fn();
    render(<Harness onSubmit={onSubmit} />);

    fireEvent.change(screen.getByLabelText(/search query/i), { target: { value: 'breach' } });
    fireEvent.change(screen.getByLabelText(/^mode$/i), { target: { value: 'keyword' } });
    fireEvent.change(screen.getByLabelText(/^category$/i), { target: { value: 'civil' } });
    fireEvent.click(screen.getByRole('button', { name: /^search$/i }));

    expect(onSubmit).toHaveBeenCalledWith('breach', 'keyword', { categoryCode: 'civil' });
  });

  it('renders a save search button only when onSaveSearch is provided', () => {
    const { rerender } = render(
      <SearchFilterPanel
        query=""
        onQueryChange={jest.fn()}
        mode=""
        onModeChange={jest.fn()}
        filters={{}}
        onFiltersChange={jest.fn()}
        onSubmit={jest.fn()}
      />,
    );
    expect(screen.queryByRole('button', { name: /save search/i })).not.toBeInTheDocument();

    rerender(
      <SearchFilterPanel
        query=""
        onQueryChange={jest.fn()}
        mode=""
        onModeChange={jest.fn()}
        filters={{}}
        onFiltersChange={jest.fn()}
        onSubmit={jest.fn()}
        onSaveSearch={jest.fn()}
      />,
    );
    expect(screen.getByRole('button', { name: /save search/i })).toBeInTheDocument();
  });
});
