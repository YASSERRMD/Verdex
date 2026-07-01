'use client';

import type { ReactNode } from 'react';
import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';
import type { AuthUser } from '@/lib/auth';
import type { Role } from '@/types';

interface AppShellProps {
  children: ReactNode;
  user?: AuthUser | null;
  roles?: Role[];
}

export function AppShell({ children, user, roles = [] }: AppShellProps) {
  return (
    <div className="flex h-screen overflow-hidden bg-neutral-50 dark:bg-neutral-900">
      {/* Sidebar */}
      <Sidebar roles={roles} />

      {/* Main area */}
      <div className="flex flex-1 flex-col overflow-hidden">
        <TopBar user={user} />

        {/* Page content */}
        <main
          id="main-content"
          className="flex-1 overflow-y-auto px-6 py-8"
        >
          {children}
        </main>
      </div>
    </div>
  );
}
