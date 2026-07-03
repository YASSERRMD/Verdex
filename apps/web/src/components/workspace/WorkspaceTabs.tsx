'use client';

import clsx from 'clsx';

export type WorkspaceTabId = 'overview' | 'evidence' | 'tree' | 'reasoning';

export interface WorkspaceTab {
  id: WorkspaceTabId;
  label: string;
}

export const WORKSPACE_TABS: WorkspaceTab[] = [
  { id: 'overview', label: 'Overview' },
  { id: 'evidence', label: 'Evidence & Timeline' },
  { id: 'tree', label: 'Reasoning Tree' },
  { id: 'reasoning', label: 'Draft Opinion' },
];

export interface WorkspaceTabsProps {
  activeTab: WorkspaceTabId;
  onTabChange: (tab: WorkspaceTabId) => void;
  className?: string;
}

/**
 * Quick-navigation tab strip between the case workspace's panels
 * (overview, evidence/timeline, reasoning tree, draft opinion).
 */
export function WorkspaceTabs({ activeTab, onTabChange, className }: WorkspaceTabsProps) {
  return (
    <div
      role="tablist"
      aria-label="Case workspace sections"
      className={clsx(
        'flex gap-1 overflow-x-auto border-b border-neutral-200 dark:border-neutral-700',
        className,
      )}
    >
      {WORKSPACE_TABS.map((tab) => {
        const active = tab.id === activeTab;
        return (
          <button
            key={tab.id}
            type="button"
            role="tab"
            id={`workspace-tab-${tab.id}`}
            aria-selected={active}
            aria-controls={`workspace-tabpanel-${tab.id}`}
            onClick={() => onTabChange(tab.id)}
            className={clsx(
              'flex-shrink-0 whitespace-nowrap border-b-2 px-4 py-2.5 text-sm font-medium transition-colors',
              active
                ? 'border-primary-DEFAULT text-primary-DEFAULT'
                : 'border-transparent text-neutral-500 hover:border-neutral-300 hover:text-neutral-700 dark:text-neutral-400 dark:hover:text-neutral-200',
            )}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
