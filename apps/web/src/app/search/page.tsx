'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { getSession } from '@/lib/auth';
import { apiFetch, ApiError } from '@/lib/api';
import { AppShell } from '@/components/layout/AppShell';
import { Card } from '@/components/ui/Card';
import { SearchFilterPanel } from '@/components/search/SearchFilterPanel';
import { SearchResultsList } from '@/components/search/SearchResultsList';
import { SavedSearchesPanel } from '@/components/search/SavedSearchesPanel';
import type { Role, SavedSearchEntry, SearchFilters, SearchMode, SearchResults } from '@/types';

function buildSearchParams(query: string, mode: SearchMode, filters: SearchFilters): URLSearchParams {
  const params = new URLSearchParams();
  if (query) params.set('q', query);
  if (mode) params.set('mode', mode);
  if (filters.categoryCode) params.set('category', filters.categoryCode);
  if (filters.jurisdictionId) params.set('jurisdiction_id', filters.jurisdictionId);
  if (filters.partyName) params.set('party', filters.partyName);
  if (filters.state) params.set('state', filters.state);
  if (filters.dateFrom) params.set('date_from', filters.dateFrom);
  if (filters.dateTo) params.set('date_to', filters.dateTo);
  return params;
}

export default function SearchPage() {
  const router = useRouter();
  const session = getSession();

  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<SearchMode>('');
  const [filters, setFilters] = useState<SearchFilters>({});
  const [results, setResults] = useState<SearchResults | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedSearches, setSavedSearches] = useState<SavedSearchEntry[]>([]);

  useEffect(() => {
    if (!session) {
      router.replace('/login');
    }
  }, [session, router]);

  useEffect(() => {
    if (!session) return;
    apiFetch<SavedSearchEntry[]>('/api/v1/search/saved')
      .then(setSavedSearches)
      .catch(() => {
        // A failure to load saved searches should not block the search
        // page itself from being usable.
      });
  }, [session]);

  if (!session) return null;

  const roles = (session.user.roles ?? []) as Role[];

  const runSearch = async (q: string, m: SearchMode, f: SearchFilters) => {
    setLoading(true);
    setError(null);
    setHasSearched(true);
    try {
      const params = buildSearchParams(q, m, f);
      const result = await apiFetch<SearchResults>(`/api/v1/search?${params.toString()}`);
      setResults(result);
    } catch (err) {
      setResults(null);
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError(err instanceof Error ? err.message : 'Search failed.');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = () => {
    void runSearch(query, mode, filters);
  };

  const handleSaveSearch = async () => {
    const name = window.prompt('Name this search:');
    if (!name) return;
    try {
      const saved = await apiFetch<SavedSearchEntry>('/api/v1/search/saved', {
        method: 'POST',
        body: JSON.stringify({ name, text: query, mode, filter: filters }),
      });
      setSavedSearches((prev) => [saved, ...prev]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save search.');
    }
  };

  const handleRunSaved = (saved: SavedSearchEntry) => {
    setQuery(saved.query.text ?? '');
    setMode(saved.query.mode ?? '');
    setFilters(saved.query.filter ?? {});
    void runSearch(saved.query.text ?? '', saved.query.mode ?? '', saved.query.filter ?? {});
  };

  const handleDeleteSaved = async (saved: SavedSearchEntry) => {
    try {
      await apiFetch(`/api/v1/search/saved/${saved.id}`, { method: 'DELETE' });
      setSavedSearches((prev) => prev.filter((s) => s.id !== saved.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete saved search.');
    }
  };

  return (
    <AppShell user={session.user} roles={roles}>
      <div className="mx-auto max-w-5xl space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-neutral-900 dark:text-white">Search cases</h1>
          <p className="mt-1 text-sm text-neutral-500">
            Search across every case you have access to by content, issue/rule, or filter.
          </p>
        </div>

        <Card padding="md">
          <SearchFilterPanel
            query={query}
            onQueryChange={setQuery}
            mode={mode}
            onModeChange={setMode}
            filters={filters}
            onFiltersChange={setFilters}
            onSubmit={handleSubmit}
            onSaveSearch={handleSaveSearch}
            submitting={loading}
          />
        </Card>

        {error && (
          <p role="alert" className="text-sm text-red-600">
            {error}
          </p>
        )}

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-[2fr_1fr]">
          <div>
            <SearchResultsList
              results={results?.items ?? []}
              loading={loading}
              hasSearched={hasSearched}
              skippedCases={results?.skippedCases ?? 0}
            />
          </div>
          <div>
            <Card padding="md" header={<h2 className="text-sm font-semibold">Saved searches</h2>}>
              <SavedSearchesPanel
                savedSearches={savedSearches}
                onRun={handleRunSaved}
                onDelete={handleDeleteSaved}
              />
            </Card>
          </div>
        </div>
      </div>
    </AppShell>
  );
}
