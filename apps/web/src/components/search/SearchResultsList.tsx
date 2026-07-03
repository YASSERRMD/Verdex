'use client';

import Link from 'next/link';
import { Card } from '@/components/ui/Card';
import type { SearchResultItem } from '@/types';

interface SearchResultsListProps {
  results: SearchResultItem[];
  loading?: boolean;
  hasSearched?: boolean;
  skippedCases?: number;
}

/**
 * Renders a highlighted snippet, converting casesearch's `**match**`
 * highlight markers (packages/casesearch.SnippetHighlightOpen/Close) into
 * a <mark> element, since the backend deliberately returns a
 * markup-agnostic convention rather than embedding HTML.
 */
function renderSnippet(snippet: string) {
  const parts = snippet.split('**');
  return parts.map((part, i) =>
    i % 2 === 1 ? (
      <mark key={i} className="rounded bg-accent-DEFAULT/40 px-0.5 text-neutral-900">
        {part}
      </mark>
    ) : (
      <span key={i}>{part}</span>
    ),
  );
}

const STATE_LABELS: Record<string, string> = {
  draft: 'Draft',
  active: 'Active',
  under_review: 'Under Review',
  closed: 'Closed',
  archived: 'Archived',
};

export function SearchResultsList({
  results,
  loading = false,
  hasSearched = false,
  skippedCases = 0,
}: SearchResultsListProps) {
  if (loading) {
    return (
      <div role="status" aria-live="polite" className="py-12 text-center text-sm text-neutral-500">
        Searching cases…
      </div>
    );
  }

  if (!hasSearched) {
    return (
      <div className="py-12 text-center text-sm text-neutral-500">
        Enter a search term or apply a filter to find cases.
      </div>
    );
  }

  if (results.length === 0) {
    return (
      <div className="py-12 text-center text-sm text-neutral-500">
        No cases matched your search.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {skippedCases > 0 && (
        <p className="text-xs text-neutral-500">
          {skippedCases} case{skippedCases === 1 ? '' : 's'} could not be searched and{' '}
          {skippedCases === 1 ? 'was' : 'were'} skipped.
        </p>
      )}
      <ul className="space-y-3" data-testid="search-results-list">
        {results.map((item) => (
          <li key={item.caseId}>
            <Card padding="md">
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 flex-1">
                  <Link
                    href={`/cases/${item.caseId}`}
                    className="text-base font-semibold text-primary-DEFAULT hover:underline"
                  >
                    {item.title || 'Untitled case'}
                  </Link>
                  {item.reference && (
                    <span className="ml-2 text-xs text-neutral-400">{item.reference}</span>
                  )}
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-neutral-500">
                    {item.categoryId && (
                      <span className="rounded-full bg-neutral-100 px-2 py-0.5 dark:bg-neutral-700">
                        {item.categoryId}
                      </span>
                    )}
                    {item.state && (
                      <span className="rounded-full bg-neutral-100 px-2 py-0.5 dark:bg-neutral-700">
                        {STATE_LABELS[item.state] ?? item.state}
                      </span>
                    )}
                    {item.createdAt && (
                      <span>{new Date(item.createdAt).toLocaleDateString()}</span>
                    )}
                  </div>
                  {item.snippet && (
                    <p className="mt-2 text-sm text-neutral-600 dark:text-neutral-300">
                      {renderSnippet(item.snippet)}
                    </p>
                  )}
                </div>
                <span
                  className="flex-shrink-0 rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-DEFAULT dark:bg-primary-900/30"
                  title="Relevance score"
                >
                  {Math.round(item.score * 100)}%
                </span>
              </div>
            </Card>
          </li>
        ))}
      </ul>
    </div>
  );
}
