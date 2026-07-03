'use client';

import { Button } from '@/components/ui/Button';
import type { SavedSearchEntry } from '@/types';

interface SavedSearchesPanelProps {
  savedSearches: SavedSearchEntry[];
  onRun: (saved: SavedSearchEntry) => void;
  onDelete: (saved: SavedSearchEntry) => void;
}

export function SavedSearchesPanel({ savedSearches, onRun, onDelete }: SavedSearchesPanelProps) {
  if (savedSearches.length === 0) {
    return <p className="text-sm text-neutral-500">You have no saved searches yet.</p>;
  }

  return (
    <ul className="space-y-2" aria-label="Saved searches">
      {savedSearches.map((saved) => (
        <li
          key={saved.id}
          className="flex items-center justify-between gap-2 rounded-lg border border-neutral-200 px-3 py-2 dark:border-neutral-700"
        >
          <button
            type="button"
            onClick={() => onRun(saved)}
            className="min-w-0 flex-1 truncate text-left text-sm font-medium text-primary-DEFAULT hover:underline"
          >
            {saved.name}
          </button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onDelete(saved)}
            aria-label={`Delete saved search ${saved.name}`}
          >
            Delete
          </Button>
        </li>
      ))}
    </ul>
  );
}
