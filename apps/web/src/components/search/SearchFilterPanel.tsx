'use client';

import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { Button } from '@/components/ui/Button';
import type { SearchFilters, SearchMode } from '@/types';

const MODE_OPTIONS = [
  { value: '', label: 'Auto' },
  { value: 'keyword', label: 'Keyword' },
  { value: 'semantic', label: 'Semantic' },
  { value: 'issue_rule', label: 'Issue / Rule' },
];

const STATE_OPTIONS = [
  { value: '', label: 'Any state' },
  { value: 'draft', label: 'Draft' },
  { value: 'active', label: 'Active' },
  { value: 'under_review', label: 'Under Review' },
  { value: 'closed', label: 'Closed' },
  { value: 'archived', label: 'Archived' },
];

interface SearchFilterPanelProps {
  query: string;
  onQueryChange: (value: string) => void;
  mode: SearchMode;
  onModeChange: (mode: SearchMode) => void;
  filters: SearchFilters;
  onFiltersChange: (filters: SearchFilters) => void;
  onSubmit: () => void;
  onSaveSearch?: () => void;
  submitting?: boolean;
}

export function SearchFilterPanel({
  query,
  onQueryChange,
  mode,
  onModeChange,
  filters,
  onFiltersChange,
  onSubmit,
  onSaveSearch,
  submitting = false,
}: SearchFilterPanelProps) {
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit();
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4" aria-label="Case search filters">
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="flex-1">
          <Input
            label="Search"
            placeholder="Search case content, issues, or rules…"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            aria-label="Search query"
          />
        </div>
        <div className="w-full sm:w-48">
          <Select
            label="Mode"
            options={MODE_OPTIONS}
            value={mode}
            onChange={(e) => onModeChange(e.target.value as SearchMode)}
          />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Input
          label="Category"
          placeholder="e.g. civil"
          value={filters.categoryCode ?? ''}
          onChange={(e) => onFiltersChange({ ...filters, categoryCode: e.target.value })}
        />
        <Input
          label="Party name"
          placeholder="e.g. Acme Corp"
          value={filters.partyName ?? ''}
          onChange={(e) => onFiltersChange({ ...filters, partyName: e.target.value })}
        />
        <Select
          label="State"
          options={STATE_OPTIONS}
          value={filters.state ?? ''}
          onChange={(e) => onFiltersChange({ ...filters, state: e.target.value })}
        />
        <div className="flex gap-2">
          <Input
            label="From"
            type="date"
            value={filters.dateFrom ?? ''}
            onChange={(e) => onFiltersChange({ ...filters, dateFrom: e.target.value })}
          />
          <Input
            label="To"
            type="date"
            value={filters.dateTo ?? ''}
            onChange={(e) => onFiltersChange({ ...filters, dateTo: e.target.value })}
          />
        </div>
      </div>

      <div className="flex items-center gap-3">
        <Button type="submit" loading={submitting}>
          Search
        </Button>
        {onSaveSearch && (
          <Button type="button" variant="ghost" onClick={onSaveSearch}>
            Save search
          </Button>
        )}
      </div>
    </form>
  );
}
